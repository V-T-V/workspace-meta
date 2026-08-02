# KidsPark · AI 素材生成管线

> 综合自《AI 时代的 Godot 全流程实操》一文的验证经验，适配本项目。

## 为什么需要这个文档

当前 kids-park 的视觉资产全部由**代码生成**（CSG 几何体 + 少量 CC0 glTF 模型）。
优势：零依赖、体积小、可参数化。
瓶颈：画面质感有天花板，缺少手绘纹理、UI 图标、装饰贴图。

**AI 素材管线**用于在需要时补充高质量 2D/3D 素材，而非替代现有的代码生成方式。

---

## 工具链分层（三层并行，互不冲突）

| 层级 | 工具 | 用途 | 国产替代 |
|------|------|------|---------|
| **图像生成** | GPT-image-2 / Midjourney / SD | 角色贴图、UI 图标、背景 | 通义万相 / 即梦 / 可灵 |
| **AI 编码** | Claude Code / Cursor | GDScript 代码 | Trae（免费）+ DeepSeek/GLM/Kimi |
| **编辑器操控** | Godot MCP Pro / godot-mcp | AI 直接建节点、导出 | godot-mcp（开源 13 工具）|

> **本项目当前主要用第二层（AI 编码）**。第一、三层为可选增强。

---

## 一、2D 素材生成（贴图/图标/UI）

### 适用场景
- 收集物图标（图鉴 UI 用，替换纯 emoji）
- 区域指示牌贴图
- 贴纸册贴纸（比 emoji 更精美）
- 背景天空盒（替换 ProceduralSkyMaterial）
- 地面纹理（草地/沙滩/雪地 tile）

### Prompt 模板（经实战验证）

#### 收集物图标（透明背景）
```
A cute cartoon [apple/flower/shell/pearl...] icon for a children's game.
Style: flat design, thick rounded outline, vibrant saturated colors,
soft pastel shading, centered, white background (will be removed).
Simple enough for ages 3-8. 256x256 px.
```

#### UI 面板素材（NinePatchRect 用）
```
A rounded rectangle UI panel background for a kids game.
Warm cream color (#FAF0E6) with golden border (#F0C040).
Soft drop shadow, slightly glossy. 9-slice friendly (corners 20px radius).
512x512 px, flat, no text.
```

#### 贴纸（贴纸册用）
```
A collectible sticker of a [cute bunny/kitten/bear cub/fox cub]
wearing a tiny [crown/scarf/bow]. Kawaii style, thick white border
like a real sticker, sparkles around it. 512x512, white background.
```

#### 天空背景
```
A bright cheerful cartoon sky for a children's park game.
Warm sunrise palette: soft pink clouds, golden sun glow, light blue sky.
No characters, no text. Seamless horizontal panorama. 4096x1024 px.
```

### 关键经验（踩坑总结）
1. **AI 直接生成透明背景**不可靠 → 生成白底图，用 remove.bg 或 Photoshop 手动去背
2. **多图风格一致性**：GPT-image-2 的 16 张参考图功能 / SD 的 LoRA / 给同一 style 描述
3. **中文文字渲染**：GPT-image-2 能正确生成中文按钮，但 SD/MJ 不行 → 含文字的 UI 用 GPT
4. **圆角面板**：生成后必须配 `NinePatchRect` 节点，否则拉伸会变形

### 导入 Godot 流程
```
1. 生成 PNG → 放入 assets/textures/
2. Godot 自动导入（.import 文件生成）
3. 代码中：load("res://assets/textures/apple_icon.png")
4. UI 中用 TextureRect / NinePatchRect
5. 3D 中用 StandardMaterial3D.albedo_texture 贴在 CSG 表面
```

---

## 二、3D 模型获取策略

### 当前方案（已在用）
| 来源 | 模型 | 用途 |
|------|------|------|
| Khronos glTF Sample Assets | Fox/duck/Avocado/BarramundiFish/BoomBox/Lantern/Box | 玩家/NPC/收集物/装饰 |
| 代码生成（CSG） | ModelFactory.gd | 树/花/房子/动物/收集物/设施 |

### 可扩展来源
| 来源 | 说明 | License |
|------|------|---------|
| **Kenney.nl** | 大量卡通低多边形包（Nature Pack/Props/Characters）| CC0 |
| **Quaternius** | 卡通动物/家具/食物模型 | CC0 |
| **KayKit** | 低多边形角色 + 动画 | CC0 |
| **Poly Pizza** | 5000+ 单个模型搜索库 | CC-BY/CC0 |
| **AI: Tripo/Meshy** | 文本/图片生成 3D | 商用付费 |

### 下载脚本模式（已验证可用）
```bash
# 从 Khronos 下载（本项目已用此方式获取 9 个模型）
curl -L -o model.glb "https://raw.githubusercontent.com/KhronosGroup/glTF-Sample-Assets/main/Models/[Name]/glTF-Binary/[Name].glb"
# 验证 magic header
head -c 4 model.glb  # 应输出 "glTF"
# Godot 导入
Godot.exe --headless --path kids-park --import --quit
```

---

## 三、AI 编辑器操控（Godot MCP，可选高级用法）

### 何时不该用 MCP
- 写代码 → 用 Claude Code / Cursor 直接改文件更可靠
- 简单场景 → 手动建更快
- 自动化测试 → 用 `--headless --quit-after` 命令行

### 何时该用 MCP
- AI 需要可视化验证（截图比对）
- 批量建节点（一句话建 10 个收集物）
- 实时调参（光照/材质参数试错）

### MCP 的坑（实战教训）
1. **连接断开**常见，需要重连机制
2. **工具调用失败**静默吞错，要检查返回值
3. **上下文膨胀**：Playwright MCP 会灌入 13700 token 工具描述 → 按需加载，别全开

---

## 四、游戏内 AI（NPC 智能对话，未来方向）

### 方案：NobodyWho（本地 LLM）
- 原理：GGUF 量化模型在游戏进程内跑
- 优点：离线、隐私、免费
- 缺点：**中低端手机性能开销大**，桌面可用

### 适用场景（kids-park 未来）
- NPC 不再只说硬编码台词，能和儿童自由对话
- 故事动态生成（"给我讲个关于小兔的故事"）

### 实现路径
```
1. 安装 NobodyWho Godot 插件
2. 下载量化模型（Qwen2.5-1.5B-Instruct Q4 适合移动端）
3. NPC.gd 接入：对话气泡 → LLM → 回复显示
4. 加儿童内容过滤（prompt 约束 + 关键词拦截）
```

---

## 项目素材现状 & 升级路线

| 资产类型 | 当前 | 可升级到 | 优先级 |
|----------|------|---------|--------|
| 收集物模型 | CSG 组合体 | AI 生成精灵贴图贴 CSG | 中 |
| UI 图标 | emoji 字符 | AI 生成图标 PNG | 中 |
| 贴纸 | 文字 emoji | AI 生成贴纸图 | 低 |
| 背景 | ProceduralSky | AI 生成天空盒贴图 | 低 |
| 角色 | Fox.glb (无动画) | KayKit 带骨骼动画模型 | 高 |
| 地面 | 纯色 unshaded | AI tile 纹理 | 低 |
| 音乐 | 代码合成正弦波 | AI 生成 BGM (Suno) | 中 |

---

*本文档随项目演进持续更新。每新增一类 AI 素材，记录其 prompt 和踩坑经验。*
