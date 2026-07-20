# Web 游戏引擎研究与抉择指南

> 基于本工作区 7 个游戏/3D 项目的真实代码核实，结合 2026 行业现状。
> 生成日期：2026-07-16

---

## 一、当前工作区游戏项目梳理

| 项目 | 引擎/库 | 版本 | 渲染模式 | 物理 | 架构 | 规模 | 状态 |
|------|---------|------|----------|------|------|------|------|
| **wildera** | Babylon.js | ^8.0 | **WebGPU** 优先，WebGL2 回退 | **Havok**(真启用) + recast 寻路 | 模块化 Game 类 + EventBus | 58 文件 | 接近成品的游戏雏形 |
| **future-world-3055** | Babylon.js | ^7.54 | WebGL2 | Havok(**声明但未真启用**) | **严格 ECS**(18+系统) | 69 文件 | 重型技术原型/系统沙盒 |
| **taixu-dao-world** | three.js | ^0.178 | WebGL + 后处理(Bloom) | 无 | 系统化(16 system，非正式 ECS) | 64 文件 | 技术原型 |
| **agenttrain** | 无(裸 TS) | — | **Canvas 2D** | 无 | 逻辑/渲染分离 + 固定步长 | 17 文件 | **完整可玩**小游戏 |
| **kids-games** | 无(裸 TS) | — | **DOM 为主** + 少量 Canvas 2D | 无 | BaseGame 基类 + 81 游戏工厂 | 109 文件 | **完整**游戏集合(最大) |
| **modelstudio** | React Three Fiber + three | R3F 8 / three 0.169 | WebGL | 无 | React + Zustand | 13 文件 | 3D 编辑器(非游戏) |
| **pastoral-travel** | **Godot 4.7** + Web 原型 | Godot 4.7 | Godot: OpenGL ES; Web: DOM | Godot 侧无 | Godot autoload 单例 | 12 .gd | 手游原型(已出 APK) |

### 关键事实（代码核实，非 README 臆测）
- **wildera 是唯一真用 WebGPU 的项目**（`WebGPUEngine` 探测 + WebGL2 回退）。
- **future-world-3055 的 Havok 是"半接入"**：装了依赖、调了 `HavokPhysics()`，但**没 `scene.enablePhysics()`，物理实际未生效**。
- agenttrain / kids-games / pastoral-travel(Web原型) **没有任何 3D/渲染库**，纯 Canvas 2D / DOM。
- 只有 future-world-3055 是**严格 ECS**（Entity/Component/System/World 四件套）。

---

## 二、核心概念：游戏引擎 vs 3D 渲染引擎

这是两个常被混淆但本质不同的东西。

### 3D 渲染引擎（Rendering Engine）
**只负责"把 3D 场景画到屏幕上"**。它提供：场景图、相机、光照、材质、几何体、后处理、动画混合——但不提供游戏循环之外的任何游戏逻辑。

| 典型代表 | 性质 |
|----------|------|
| **three.js** | 最流行的 Web 3D 渲染库。低层、灵活，什么都要自己拼 |
| **Babylon.js** 的渲染核心 | 强渲染（WebGPU 优先、PBR、节点编辑器） |

**适合**：数据可视化、产品展示、3D 编辑器（如本工作区 modelstudio）、轻量 3D 网站、AR 展示。
**不适合**：完整 3D 游戏（你要自己造物理、音频、动画状态机、场景管理、序列化……成本极高）。

### 游戏引擎（Game Engine）
**渲染只是其中一个子系统**。它在渲染之上，还提供：

| 能力 | 渲染引擎有吗 | 游戏引擎有 |
|------|:---:|:---:|
| 渲染（场景/相机/光照/材质） | ✅ | ✅ |
| **物理引擎**（碰撞/刚体/约束） | ❌ 需外接 | ✅ 内建 |
| **音频系统**（3D 空间音效/混音） | ❌ | ✅ |
| **动画系统**（骨骼/状态机/ blend tree） | 部分 | ✅ 完整 |
| **资源管线**（导入/压缩/LOD/序列化） | ❌ | ✅ |
| **场景编辑器**（可视化拖拽） | ❌ | ✅ |
| **脚本/组件系统**（ECS 或 MonoBehaviour） | ❌ | ✅ |
| **跨平台导出**（桌面/移动/主机） | ❌ | ✅ |
| **粒子/特效/后处理** | 部分 | ✅ 完整 |
| **寻路/AI/网络** | ❌ | ✅ |

**典型 Web 游戏引擎**：Babylon.js（完整版）、PlayCanvas、Phaser（2D）、PixiJS（2D 渲染）、Cocos Creator、Godot（Web 导出）、Unity（WebGL 导出）。

### 一句话区分
> **渲染引擎**给你画笔；**游戏引擎**给你整个工作室（画笔 + 物理 + 音效 + 编辑器 + 导出管线）。
> three.js 是渲染引擎；Babylon.js 既是渲染引擎也是游戏引擎（取决于你用多少子系统）；Godot/Unity 是完整游戏引擎。

---

## 三、Web 端主流方案对比（2026）

### A. 纯 Canvas 2D / DOM（无引擎）
- **代表**：本工作区 agenttrain、kids-games
- **优点**：零依赖、零体积、加载极快、SEO 友好、手机兼容性最好
- **缺点**：复杂效果要手写、无物理/音频辅助
- **抉择**：**2D 休闲/教育/轻量游戏**（如儿童游戏、益智、卡牌）→ 首选

### B. 2D 渲染引擎（PixiJS / Phaser）
- **PixiJS**：最快的 Web 2D 渲染器（WebGL），只管画，游戏逻辑自理
- **Phaser**：基于 PixiJS/WebGL 的**完整 2D 游戏框架**（物理 Arcade/Matter、音频、输入、场景、粒子）
- **抉择**：**中重度 2D 游戏**（平台跳跃、射击、RPG）→ Phaser；**只要 2D 高性能渲染不要框架** → PixiJS

### C. three.js（3D 渲染引擎）
- **优点**：生态最大、文档最全、灵活、与 React 生态（R3F）无缝
- **缺点**：**只是渲染**——物理要接 cannon/rapier、音频要自建、无编辑器、做完整游戏极累
- **抉择**：**3D 可视化 / 产品展示 / 3D 编辑器 / 轻交互**（如本工作区 modelstudio、taixu-dao-world）→ 首选；**完整 3D 游戏** → 不推荐单独用

### D. Babylon.js（3D 渲染 + 游戏引擎）
- **优点**：**Web 渲染最强**（WebGPU 优先、PBR、节点材质编辑器）、内建物理（Havok/Cannon）、音频、GUI、粒子、寻路、glTF、一套全包
- **缺点**：体积大、学习曲线陡、生态不如 three.js
- **抉择**：**严肃 3D 游戏 / 工业级 3D 应用**（如本工作区 wildera、future-world-3055）→ 强烈推荐

### E. Godot（Web 导出）
- **优点**：**完整游戏引擎**（2D+3D、内置编辑器、GDScript、物理、动画、信号）、开源免费、导出 Web/桌面/移动
- **缺点**：Web 导出体积偏大（wasm）、WebGPU 支持滞后、Web 端性能不如原生
- **抉择**：**要可视化编辑器 + 完整游戏循环 + 多平台**（如本工作区 pastoral-travel 出了 APK）→ Godot；**纯 Web 性能敏感** → 慎用 Web 导出

### F. Unity / Unreal（WebGL 导出）
- **优点**：工业级完整、资源商店
- **缺点**：WebGL 导出**体积巨大**（10MB+ 起步）、加载慢、移动端体验差、Unity 已弱化 Web 导出
- **抉择**：**已有 Unity 团队要顺带出 Web 版** → 可；**Web 优先的新项目** → 不推荐

---

## 四、如何抉择（决策树）

```
你的项目是什么？
│
├─ 2D 轻量/教育/休闲（儿童游戏、益智、卡牌）
│  └─→ 纯 Canvas 2D / DOM（如 kids-games）
│       或 Phaser（要物理/场景管理时）
│
├─ 3D 可视化 / 产品展示 / 3D 编辑器
│  └─→ three.js + React Three Fiber（如 modelstudio）
│       灵活、生态好、不要游戏引擎的重量
│
├─ 严肃 3D 游戏（生存/开放世界/模拟）
│  └─→ Babylon.js + Havok（如 wildera）
│       渲染强、物理内建、WebGPU 就绪
│
├─ 需要可视化编辑器 + 多平台导出（含移动）
│  └─→ Godot 4（如 pastoral-travel）
│       完整引擎、编辑器、可出 APK
│
└─ 已有 Unity 团队
   └─→ Unity WebGL 导出（仅限必须）
```

### 抉择的 5 个关键问题
1. **2D 还是 3D？** 2D 别上 3D 引擎（杀鸡用牛刀）；3D 别用 2D 库硬撑。
2. **要不要物理？** 要→选内建物理的（Babylon Havok / Phaser Matter / Godot）；不要→three.js 足矣。
3. **要不要可视化编辑器？** 要→Godot/Unity/PlayCanvas；不要→代码手撸（three/Babylon 都行）。
4. **Web 优先还是多平台？** Web 优先→Babylon/three/Phaser；要多平台（移动/桌面）→Godot/Unity。
5. **团队规模与维护成本？** 一个人→别碰 Unreal；小团队→Godot 或 Babylon；要快速出 2D→Phaser。

---

## 五、对本工作区的技术建议

| 项目 | 当前 | 建议 | 理由 |
|------|------|------|------|
| **kids-games** | 纯 DOM | ✅ 保持 | 81 个轻量儿童游戏，DOM/Canvas 2D 是最优解，无需引擎 |
| **agenttrain** | Canvas 2D | ✅ 保持 | 单个调度小游戏，2D 足够 |
| **modelstudio** | three.js + R3F | ✅ 保持 | 3D 编辑器，three.js + R3F 灵活适配 React，不需游戏引擎 |
| **wildera** | Babylon 8 + Havok | ✅ 保持 | 开放世界生存游戏，Babylon 渲染强+Havok 物理是对的组合 |
| **future-world-3055** | Babylon 7 | ⚠️ 升级考虑 | Havok 未真启用（bug），若要做真物理建议升级到 Babylon 8 并修复 enablePhysics；ECS 架构可保留 |
| **taixu-dao-world** | three.js | ⚠️ 评估 | 若要做完整修仙游戏（战斗/物理），three.js 会很吃力，考虑迁 Babylon 或接 rapier；若只是展示/飞行则保持 |
| **pastoral-travel** | Godot 4.7 | ✅ 保持 | 已出 APK，Godot 多平台+编辑器是对的 |

### 通用建议
- **不要为了"用引擎"而用引擎**：本工作区最大的成功恰恰是 kids-games（纯 DOM，零引擎，81 游戏最快加载）和 modelstudio（three.js 精准匹配编辑器需求）。
- **物理引擎别"半接入"**：future-world-3055 的教训——装了 Havok 却没 enablePhysics，要么真用要么别装，避免误导。
- **3D 游戏统一到 Babylon**：wildera 已验证 Babylon 8 + WebGPU + Havok 路线可行，future-world/taixu 若要做重 3D，向 wildera 对齐可互鉴代码。
