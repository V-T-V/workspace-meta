# 低成本 3D 建模方案研究

> 基于本工作区 8 个 3D / 游戏项目的真实代码核实，结合 2026 行业现状。
> 生成日期：2026-07-20 · 同类文档：[`GAME_ENGINE_RESEARCH.md`](./GAME_ENGINE_RESEARCH.md)

---

## 一、工作区 3D 建模现状（代码核实，非 README 臆测）

### 关键事实

1. **8 个 3D/游戏项目全部 100% 程序化生成几何**（纯代码 + 程序化噪声 + 实例化），**零外部模型文件、零美术资源、零 GLTFLoader**。
2. `find .glb/.gltf/.obj/.fbx/.vox/.stl` 在所有项目里结果全为空；`grep SceneLoader|ImportMesh|GLTFLoader|useLoader` 全空。
3. 纹理也"无图化"：顶点色 AO / `<canvas>` 程序化 CanvasTexture / ShaderMaterial / PBR 纯色，**没有任何贴图文件**。
4. 唯一的 AI 建模实验是 `modelstudio`（单图 → glb，本地 + 云端混合后端）。

### 逐项明细

| 项目 | 引擎 | 几何来源 | 批量渲染 | 纹理来源 | 外部模型文件 |
|---|---|---|---|---|---|
| `wildera` | Babylon 8 | MeshBuilder 基本体 + simplex 噪声地形 | **ThinInstance**（建筑） | 顶点色 + CellMaterial | 无（`public/assets` 空） |
| `future-world-3055` | Babylon 7 | MeshBuilder 基本体 | Box 实例化 | 顶点色 | 无（**`public/assets/*` 目录骨架已建好但全空**） |
| `taixu-dao-world` | Three | 分层 Group 拼修士（14+ 部件） | InstancedMesh 群众 | **`<canvas>` 程序化 CanvasTexture** | 无 |
| `sky-carrier` | Babylon | MeshBuilder（**445 次**，零件级） | Torus/Box 堆叠 | PBR StandardMaterial | 无 |
| `voxel-craft` | Godot 4.7 | SurfaceTool 手写立方体 + FastNoiseLite | **MultiMesh** | **顶点色 AO 替代贴图** | 无 |
| `fusion-power-3d` | Three | 多层 SphereGeometry/TorusGeometry 嵌套 | 能量粒子流 | **GLSL Shader** | 无 |
| `physics-sim` | Babylon + Havok | MeshBuilder 基本体 + Lines | thinInstance | 顶点色 | 无 |
| `modelstudio` | React+Three (web) + Python (engine) | **AI 生成**（TripoSR/MiDaS/Replicate） | — | 上游模型自带 | 输出 glb |

### 共性规律（工作区的"建模风格指纹"）

- **零美术资源、纯代码几何 + 程序化噪声 + 实例化**是统一打法。
- 角色和载具都是"基本体拼装"，最高完成度是 `taixu-dao-world/CharacterModel.ts`（分层修士，526 行）和 `sky-carrier/src/space/vehicles/DeepSpaceParts.ts`（零件级堆叠，445 次 MeshBuilder）。
- 贴图全部"无图化"：顶点色 AO / Canvas 程序纹理 / ShaderMaterial / PBR 纯色。
- **唯一在文档中明写建模策略**的是 `voxel-craft/AGENTS.md`（"顶点色 AO 替代贴图，无纹理资源"）和 `taixu-dao-world/README`（"低成本实例化人群 + 距离 LOD"）。

### 重要修正（三轮调研后）

- **`pastoral-travel` 不是 3D 游戏**：它是 2D 等距视角的"旅行拍照 + AI 识别"游戏，`models/` 目录装的是 ncnn AI 模型权重（`landscape_cls.bin`），**不是 3D 模型**。它的 APK 96MB 主要来自 ncnn 端侧 AI 库（1.5GB godot_demo 目录），**与 3D 美术无关**。**它不是研究移动端 3D 预算的正确样本**。
- **真实移动端 3D 样本只有 `voxel-craft`**。

---

## 二、modelstudio：工作区唯一的 AI 建模实验

[`modelstudio`](./modelstudio) 定位：**单张 2D 图片 → glb 模型 + 浏览器网格编辑器**，三层架构（Python 生成引擎 :8001 / Node 网关 :8000 / React+Three 前端 :5173）。

### 5 条生成路线（按硬件自动降级）

| 后端 | 算法 | 完成度 | 门槛 |
|---|---|---|---|
| `depth_midas` | MiDaS DPT_Large 单目深度 → 高度位移网格（浮雕式 2.5D） | ✅ 完整 | CPU 可跑，~1GB 权重 |
| `tripo_sr` | TripoSR 单图→3D | ✅ 完整 | ~4GB 显存 |
| `cloud_replicate` | Replicate 云端 Tripo-SR / TRELLIS / SF3D | ✅ 完整 | 需 `REPLICATE_API_TOKEN` |
| `ai_trellis` | Microsoft TRELLIS | 🔧 仅探测，`generate()` 抛 NotImplementedError | Linux + **16GB 显存** + 编译 4 个 CUDA 子模块 |
| `ai_sf3d` | Stable Fast 3D | 🔧 仅探测，占位 | gated 模型 + VS2022 + 6GB 显存 |

### 关键实现细节

- **Backend 抽象**（`generator/app/backends/base.py:35-55`）：`generate(image_path, max_faces, on_progress) -> Asset`，返回内存里的 `trimesh.Mesh` 对象（**不是 glb 字节**）。
- **降级链**（`runner.py:54-61`）：任一 backend 抛 `NotImplementedError`/异常则继续下一个，**深度重建永远兜底**。
- **路由**（`router.py:20-26`）：用户偏好优先 → 按 `(可用性, provider)` 排序 → 本地优于云端。
- **输入**：单张 `.png/.jpg/.jpeg/.webp/.bmp`（不支持多视角/视频）。
- **输出**：唯一 `glb`（trimesh 导出 + 前端 GLTFExporter 回存）。
- **前端编辑**：TransformControls（W/E/R）、顶点拾取（V）、CSG 布尔（three-bvh-csg）、变形修改器、撤销重做。

### 已知缺陷（影响新后端设计）

- **`postprocess.py` 的 glb 导出走 trimesh，不支持 PBR**（金属/粗糙/法线贴图会丢失，只保留 baseColor）。
- **`_center_and_normalize`（postprocess.py:79-87）强制归一化到 `[-1, 1]` 立方体**——为 AR 展示设计，游戏场景会破坏原始尺寸。
- **`replicate_cloud.py:71` 固定文件名 `replicate_out.glb`**——并发任务会互相覆盖。

---

## 三、业界方案全景（2026，按成本递增）

### 第 0 级：现成 CC0 资产库（成本 ≈ 0）

| 库 | 协议 | 强项 |
|---|---|---|
| [Quaternius](https://quaternius.com/) | **CC0** | 数百个低多边形角色/自然/建筑，.blend/.fbx/.obj |
| [Kenney](https://kenney.nl) | **CC0** | 游戏素材之王，2D/3D/音频/UI |
| [Poly Pizza](https://poly.pizza/) | 多为 CC0 | 聚合库，Unity/Unreal/Godot 直接导入 |
| [Sketchfab](https://sketchfab.com) | CC0/CC-BY/NC 混合 | 质量跨度最大，按协议过滤 |

**对工作区的意义**：`future-world-3055/public/assets/{models,textures,skybox}` 这个空目录就是为这条路线预留的。塞一批 CC0 低模比再写 500 行 MeshBuilder 性价比高。

### 第 1 级：程序化建模（Blender + Geometry Nodes）

- **Blender**（GPL，全平台）—— 2026 年仍是"免费 3D 建模之王"。
- **Geometry Nodes**：节点式程序化建模，参数化驱动，可输出可复用资产，Blender 5.0 进一步强化。
- **自动化管道**：`blender -b input.blend --python export_glb.py` 无头批量导出，`export_apply=True` 把 GN 烘焙进 glb。
- **对工作区**：现在 TS/GDScript 手写的程序化几何搬到 GN，可复用性、可迭代性、可外包协作性都会上一个台阶。

### 第 2 级：AI 生成（云端，成本极低）

| 工具 | 速度 | 计费 | 商业授权 | 拓扑质量 |
|---|---|---|---|---|
| [**Meshy**](https://www.meshy.ai/pricing) | ~20-60s | Image-to-3D = 20 credits | 免费层 **CC BY 4.0（需署名）**；Pro 起私有 | 中等，[官方有拓扑指南](https://www.meshy.ai/blog/mesh-topology) |
| [**Tripo**](https://developers.tripo3d.ai/en) | ~35s | **$0.10/model** | 付费起完整商业 | **拓扑最干净**，有 premium topology 加价 |
| **Rodin** | ~2min+ | ~$0.30+/model | 商业 | 艺术性强但慢 |
| [**Replicate**](https://replicate.ai)（modelstudio 已用） | 按秒 | TripoSR ~$0.005 | 模型各自协议 | 取决于底层模型 |

**试水建议**：Meshy 免费层（100 credits/月 = 5 个 Image-to-3D 模型）+ Tripo 免费层，零成本验证流程。**量产首选 Tripo**（$0.10/model，拓扑最干净）。

### 第 3 级：AI 生成（本地开源，成本 = 硬件 + 电费）

| 模型 | 出品 | 真实显存 | 推理时间 | 质量 |
|---|---|---|---|---|
| [**SPAR3D**](https://github.com/Stability-AI/stable-point-aware-3d) | Stability AI | **~6GB**，RTX AI PC 优化 | **<1 秒** | SF3D 升级版，带点云引导 + 实时编辑 |
| **TripoSR** | Tripo/Stability | ~6GB | <0.5 秒 | 中等（modelstudio 已接入） |
| [**Stable Fast 3D**](https://huggingface.co/stabilityai/stable-fast-3d) | Stability | ~6GB 但 **gated** | ~0.5 秒 | 中等偏上 |
| [**Hunyuan3D 2.1**](https://github.com/Tencent/Hunyuan3D-2) | 腾讯 | **16GB 起，24GB+ 推荐** | 30-60s（shape）+ 1-3min（texture） | **质量最高**，开源权重 |
| [**TRELLIS.2**](https://microsoft.github.io/TRELLIS.2/) | Microsoft | **16-24GB**，4B 参数 | ~30s | 顶级，PBR 纹理 |

**独立开发者的现实选择**：
- 6-8GB 显存（1660/2060/3060）→ **SPAR3D 或 TripoSR**，质量中等但能用。
- 12GB（3060 12G）→ 上述 + 可能勉强 Hunyuan3D shape。
- 24GB（3090/4090）→ 全都能跑，Hunyuan3D 2.1 是质量上限。
- **没有 NVIDIA GPU** → 只能云端或摄影测量。

### 第 4 级：摄影测量（手机扫描真实物体）

| 工具 | 类型 | 平台 | 成本 |
|---|---|---|---|
| [**RealityScan Mobile**](https://www.realityscan.com/mobile) | 手机 App | iOS/Android | **免费下载** |
| RealityScan 2.0（原 RealityCapture） | 桌面+移动 | 跨平台 | 免费增值 |
| [**Meshroom**](https://meshroom.org/)（AliceVision） | 桌面，**开源** | Win/Linux | **完全免费** |

**Meshroom 硬件门槛**：NVIDIA CUDA 必须 / VRAM 4-6GB 起（推荐 8-24GB）/ RAM 8GB 起（推荐 32GB）/ 30-150 张照片 / 处理时间几十分钟到数小时。AMD 卡需用 [Meshroom CL](https://www.youtube.com/watch?v=vBTr6M7__rQ)。

**对工作区**：`history-mist` 历史文物数字化是天然场景，但需 NVIDIA GPU + 32GB RAM。

---

## 四、业界方案的真实门槛（去掉广告话术）

### 4.1 AI 建模的隐藏成本：拓扑清理

**业界 2026 共识**：AI 生成的模型"won't give you perfect models"，**清理时间应纳入预算**。

标准工作流：
```
AI 生成（5万 tris）→ retopology 减到 5K-10K → 烘焙高模细节到 normal map
→ UV 展开 → 纹理压缩 ASTC 4×4 → 引擎导入
```

- 2026 年出现 **AI retopology 工具**（Meshy 自带、[AI Image to 3D](https://www.aiimageto3d.com/blog/ai-retopology-guide)），声称一键清理，但 [Reddit r/aigamedev](https://www.reddit.com/r/aigamedev/comments/1stcg8j/) 实测仍需手动 touch-up。
- **预算公式**：批量生成 100 个模型 ≠ 100 × 30 秒，而是 100 × (30 秒生成 + 30 分钟清理)。

### 4.2 商业云服务的真实计费

| 服务 | 免费额度 | 单价 | 商业授权 |
|---|---|---|---|
| **Meshy** | 100 credits/月（=5 个模型） | Image-to-3D 20 credits | 免费层 CC BY 4.0（**需署名**）；Pro 起私有 |
| **Tripo** | 免费层限低分辨率 | **$0.10/model** | 付费起完整商业 |
| **Replicate** | 新用户免费额度 | TripoSR ~$0.005 | 模型各自协议 |

**结论**：要避免署名必须付费。试水用免费层，量产选 Tripo。

### 4.3 移动端预算硬约束（2026 业界共识）

来源：[Android Game Dev](https://developer.android.com/games/optimize/textures)、[NVIDIA ASTC](https://developer.nvidia.com/astc-texture-compression-for-game-assets)。

| 资产类型 | 推荐三角形数 |
|---|---|
| 主角（hero） | 20,000-30,000 |
| 次要角色 | 5,000-10,000 |
| 风格化角色 | 1,500-5,500 |
| 环境道具 | <500 |

**场景级铁律**：
- **单帧 40-70 draw calls**（Unity/Android 推荐）
- 单场景 500K-600K 三角形可行（前提：draw call 批合并好）
- **ASTC 4×4**（现代移动 80%+ 支持，4-20x 快于 ETC2），ETC2 作 fallback
- 一张 1024×1024 未压缩纹理 ≈ 3MB VRAM ≈ 65K 多边形模型
- **draw call 和纹理内存比三角形数更致命**

### 4.4 voxel-craft 实测移动端预算（金标准）

来自 `voxel-craft/AGENTS.md`（2026-07-17 桌面 AMD Radeon 实测，3×3 chunk + 视距 6）：

| 指标 | 实测值 | 业界对比 | 评估 |
|---|---|---|---|
| FPS | 60（锁帧） | 30 基线/60 理想 | ✅ 优秀 |
| 显存 | **84.5MB** | 移动端 100-200MB | ✅ 友好 |
| **Draw calls** | **124** | 业界 40-70 | ⚠️ **偏高** |
| 渲染对象 | 718 | — | MultiMesh 批量后 |
| 节点数 | 141 | — | 适中 |
| 帧率自适应 | FPS<35→视距 6→3 | — | ✅ 兜底到位 |

**关键点**：124 draw call 已接近移动端舒适区上限。引入 AI 生成的高模角色（单个 1-5 万面 + 多材质球）会迅速突破 200，触发卡顿。

---

## 五、角色建模深度评估：taixu-dao-world

### 5.1 现状测量（526 行 CharacterModel.ts）

**14+ 分层 Group 的几何细节**（全部 Three.js 基本体）：

| 部件 | 几何 | 关键参数 | 行号 |
|---|---|---|---|
| chest | CapsuleGeometry | (7.2, 18, 8, 18) | L232 |
| face | SphereGeometry | (5.8, 24, 18) | L307 |
| hairCap 半壳 | SphereGeometry | (6.1, 24, 12, 0,2π, 0,π*0.55) | L312 |
| crown | TorusGeometry | (2.35, 0.32, 8, 28) | L324 |
| sleeve×2 | CapsuleGeometry | (2.8, 13, 6, 12) | L402 |
| scarf×2 飘带 | BoxGeometry | (1.6, 18, 0.75) | L435 |
| sheath 剑鞘 | BoxGeometry | (2.6, 36, 2.2) | L449 |
| auraRing×3 | TorusGeometry | (15+i*3.4, 0.46, 8, 96) | L478 |

**关键缺陷**：
- **零骨骼 / 零 SkinnedMesh**：所有动作是 `Math.sin(time)` + lerp 写死（idle/walk/三段连击/剑诀/护体/飞行）。
- **零命名挂点**：背剑硬编码 `position.set(5.8, 31, -5.2)` 挂在 `root` 下（不是 torso），弯腰时不跟随。
- **装备系统只有数据层**（`EquipmentSystem.ts:26`，`Partial<Record<EquipmentSlot, string>>`），**完全没有把装备 id 映射到 3D mesh**。

**材质与纹理**：
- 8 种 `MeshStandardMaterial`（skin/robeOuter/robeInner/trim/hair/boot/jade/scabbard），全 PBR。
- AssetManager 4 种 canvas 程序纹理里**角色专用只有 1 种**（`createFabricDetailTextures`，绘制经纬纹+云纹锦缎）。
- **没有面部贴图**——五官（眼/眉/嘴）全是独立 mesh。

### 5.2 群众渲染（CrowdManager.ts）

- **3 个 InstancedMesh**（body CylinderGeometry + head SphereGeometry + contactShadow Plane），总 draw call **固定 2-3 个**（与 NPC 数量无关）。
- 外观差异仅"颜色（setColorAt）+ 缩放 + 头大小"，**没有体型/性别/贴图差异**。
- 群众数量：balanced=132 / lowSpec=42 / software=20（`PerformanceProfile.ts` 硬编码）。

### 5.3 LOD 体系

- **主角无 LOD**（永远全细节）。
- **NPC 有距离 LOD**（`EncounterManager.ts:171-177`）：detailGroup 可见性切换，detailRange=520/230，auraRange=900/360，**切 mesh visibility + 动画降频，不切 LOD mesh**（`THREE.LOD` 完全没用）。

### 5.4 升级路径矩阵

| 方案 | 可行性 | 改动量 | 美术上限 | 风险 |
|---|---|---|---|---|
| **Mixamo 骨骼动画** | 5/10 | 大（200-400 行，4-5 文件） | 专业动画师级 | 异步加载重构 + 飞行/剑诀 clip 不全 + 尺度重调 |
| **CC0 Modular Men（Quaternius）** | **8/10** | 小（100-200 行，2 文件） | 卡通低模 | 无骨骼，动作仍靠现有程序化代码，升级有限 |
| **AI 生成角色（Tripo/Meshy）** | 3/10 | 中 | 单角色高 | 无分层 / 无挂点 / 无换装 / 拓扑乱 / 动画差 |

**推荐渐进路径**：
1. **群众换 Quaternius Modular Men**（最高 ROI，1 文件改动，NPC 精细度 3→6/10）。
2. **主角加命名挂点**（10-20 行，`Object3D` 命名为 `"socket-right-hand"` 等，让装备系统视觉化）。
3. **主角 Mixamo 化（二期）**：加 GLTFLoader + DRACOLoader 到 `AssetManager`，改 CharacterModel 为 SkinnedMesh + AnimationMixer。

**强烈不推荐**：AI 生成主角——丢失分层、丢失挂点、丢失换装语义，等于把 526 行拼装代码的优势全部归零。

---

## 六、业界角色建模工具现状（2026）

### 6.1 Mixamo：仍是免费

- [Mixamo 官网](https://www.mixamo.com/) 仍在线，免费 auto-rig + FBX 动画下载，**商用免费**。
- 已知问题：偶发 "too many requests" 限流。
- 替代品：**AccuRig**（ActorCore 出品，免费）——比 Mixamo 更现代。

### 6.2 Ready Player Me：**2026-01-31 已关停**

- Netflix 收购后停止服务。
- 替代品：[**MetaPerson**](https://avatarsdk.com/ready-player-me-alternative/)（功能最接近 RPM）、Union Avatars、Genies。
- **对工作区影响为零**——7 个 3D 项目没有任何一个用过 RPM。

### 6.3 AI 全自动角色管线（2026 新兴）

**Tripo** 和 **Meshy** 都已推出 **AI auto-rigging + auto-skinning + motion library** 一站式：

| 工具 | 能力 |
|---|---|
| [Tripo Auto-Rigging](https://www.tripo3d.ai/features/ai-auto-rigging) | 通用 rig，高保真蒙皮，一键 rig+skin+animate |
| [Meshy AI Animation Generator](https://www.meshy.ai/features/ai-animation-generator) | 一键 auto-rig + **600+ 动作预设**，导出 Unity/Unreal/Blender/Maya |
| [Neural4D](https://neural4d.com/features/auto-rigging) | 干净骨架 + 平滑蒙皮权重 |
| AccuRIG（Reallusion，免费） | 与 Rodin 生成模型兼容 |

**含义**：AI 全自动角色管线已成熟——文本/图片→3D→AI 绑骨→AI 蒙皮→选动作→导出 rigged GLB。但（见 5.4）对工作区不适用，分层/挂点/换装语义会丢。

---

## 七、风格统一性治理（批量 AI 建模的最大隐患）

### 7.1 工作区的天然优势

工作区 7 个 3D 项目现在的"统一风格"靠**代码几何天然保证**（同一个 `MeshBuilder.CreateSphere` 调出来的东西必然风格一致）。一旦引入 AI 生成资产，**统一性立刻破裂**。

### 7.2 三种治理策略

| 策略 | 做法 | 效果 |
|---|---|---|
| **prompt + style pillars** | 定义 3-5 个固定描述词（如"low poly stylized, hand-painted, warm palette"），所有生成共用 | Meshy 口碑较好 |
| **image-to-3D + 锁定参考图** | 用一张风格基准图，后续所有生成都以它为输入 | 比纯文本一致 |
| **LoRA 微调**（2026 新兴） | 用项目自己的美术资产训练 LoRA | 远未普及 |

**推荐做法**：严格使用 image-to-3D + 锁定参考图；接受"主角手搓 + 配角 AI 生成"的混合策略（主角保风格锚点，配角容许差异）。

---

## 八、GLB 加载层：从零接入的姿势（工作区全空白）

工作区 **7 个 3D 项目都没有 GLTFLoader**（grep 全空）。任何项目要消费 glb 都要从零加 loader 层。

### 8.1 Babylon.js 项目（wildera / future-world-3055 / sky-carrier / physics-sim）

```typescript
import { loadAssetContainerAsync } from "@babylonjs/core/Loading/sceneLoader";
const container = await loadAssetContainerAsync("assets/models/foo.glb", scene);
container.addAllToScene();
```

- 旧的 `SceneLoader.ImportMeshAsync` 已弃用。
- Babylon 原生支持 PBR（`PBRMetallicRoughness`），glb 的 PBR 通道直接生效。
- 参考：[Babylon loadAssetContainerAsync](https://doc.babylonjs.com/typedoc/functions/BABYLON.loadAssetContainerAsync)、[PBR 应用示例](https://playground.babylonjs.com/#0VHCTL#26)。

### 8.2 Three.js 项目（taixu-dao-world / fusion-power-3d）

```typescript
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { DRACOLoader } from "three/examples/jsm/loaders/DRACOLoader.js";

const loader = new GLTFLoader();
loader.setDRACOLoader(new DRACOLoader().setDecoderPath("/draco/"));
loader.load("assets/models/foo.glb", (gltf) => {
  scene.add(gltf.scene);
  const mixer = new THREE.AnimationMixer(gltf.scene);
  mixer.clipAction(gltf.animations[0]).play();
});
```

- **DRACO 必装**：CC0 / AI 生成的 glb 经常 Draco 压缩。

### 8.3 Godot 项目（voxel-craft）

```gdscript
var gltf := GLTFDocument.new()
var state := GLTFState.new()
gltf.append_from_file("res://models/foo.glb", state)
add_child(state.generate_scene(get_tree()))
```

- 编辑器导入时勾选 **"Generate LODs"** 自动生成 LOD 链。
- 参考：[Godot Importing 3D scenes](https://docs.godotengine.org/en/4.1/tutorials/assets_pipeline/importing_scenes.html)。
- **对 voxel-craft 不建议引入 glb**——体素风格靠 MultiMesh 维持，引入 glb 会破坏 draw call 批合并。

---

## 九、成本-质量决策矩阵

```
                          质量↑
                           │
    Hunyuan3D 2.1 ●       │     Blender 手工
    TRELLIS.2 ●           │     Geometry Nodes ●
                         │                       │
    SPAR3D ●              │     美术外包 ●
                         │                       │
    Meshy/Tripo ●         │                       │
    (含 auto-rig)         │                       │
                         │                       │
    RealityScan ●         │                       │
                         │                       │
    CC0 资产库 ●          │   工作区现状 ●        │
    (Quaternius Modular   │   (纯代码几何 +       │
     Men 带换装语义)       │    分层 Group 拼装)   │
  ───────────────────────┼───────────────────────→ 成本↑
   $0      $0      $0    │  $0       $$     $$$$
   现成    扫描    云端      代码      AI本地   外包
            (含 AI rig)
```

**新增维度：风格一致性维护成本**
- 纯代码几何（工作区现状）：**0**（天然一致）
- CC0 资产：低（同作者包内一致）
- AI 生成：**高**（需要 prompt 锁定 + 参考图 + 可能 LoRA）
- Blender 手工：中（同作者保持一致）
- 美术外包：中-高（需要 style guide）

---

## 十、按工作区项目的升级建议

| 项目 | 当前 | 推荐升级路径 | 理由 |
|---|---|---|---|
| **future-world-3055** | 占位几何 + 空 `assets` 目录 | ① 塞 CC0（Quaternius Ultimate Space Kit） ② 关键英雄单位用 Meshy/Tripo | 目录已预留，太空题材 CC0 最丰富 |
| **voxel-craft** | 顶点色 AO 体素 | **维持现状，不建议换** | 体素美学本身就是风格，换资产破坏统一性；124 draw call 已偏高 |
| **sky-carrier** | 445 次 MeshBuilder 拼零件 | 迁移到 Blender Geometry Nodes 做参数化载具 | 物理参数→几何尺寸的映射最适合节点化，零件多到该上专业工具 |
| **taixu-dao-world** | 分层修士（14+ Group） | 主角保留手搓，**NPC/路人**用 Quaternius Modular Men 替换 | README 自己写了"低成本实例化人群"，正好对口 |
| **wildera** | ThinInstance 建筑 | 关键建筑用 AI 生成（Meshy）→ glb → ThinInstance 模板 | 生存游戏建筑种类多，全手搓不划算 |
| **modelstudio** | TripoSR + Replicate | **补 SPAR3D 后端** + 加 Meshy/Tripo 直连适配器 | SPAR3D 是当前"本地可行 + 质量最高"的平衡点 |
| **history-mist** | 纯文字/知识图谱 | **RealityScan/Meshroom 扫描真实文物** | 历史题材天然适合摄影测量，差异化卖点 |

### sky-carrier Geometry Nodes 迁移评分

基于 445 次 MeshBuilder 调用的深度审计：

| 子项 | 评分 | 理由 |
|---|---|---|
| 单 Builder 几何复杂度 | 3/10 | 全是基本体+变换+阵列 |
| 参数化程度 | 8/10 | 物理求解器驱动，参数 schema 已存在 |
| 零件复用度 | 9/10 | 三层装配已具 node group 雏形 |
| 运行时动态性 | 1/10 | 动画全是 TransformNode 级，不依赖 TS 几何 |
| **整体迁移友好度** | **8.5/10** | 教科书级 GN 迁移候选 |

**关键证据**：全代码库**零 CSG 布尔运算**、**零顶点变形（setVerticesData）**、**零 dispose+rebuild**。参数链 `physics/constants.ts → massBalance.ts → geometry.ts → CarrierSpec.ts → CarrierBuilder.ts` 极其干净（实跑验证：改 `MISSION.attackAircraft` 一个数字，整条链路包括 3D 模型自动重算）。`OPTIMIZATION_NOTES.md`（2026-07-18）已经在抱怨 445 个静态 mesh 的 `freezeWorldMatrix` 性能问题——迁移动机天然存在。

---

## 十一、风险与陷阱清单

| 陷阱 | 影响项目 | 规避 |
|---|---|---|
| **AI 拓扑清理工时被低估** | 任何用 Meshy/Tripo 产资产的项目 | 预算按"30 秒生成 + 30 分钟清理"计 |
| **trimesh 不支持 PBR** | modelstudio | postprocess 加"透传 glb 字节"分支 |
| **postprocess 归一化抹掉尺寸** | modelstudio → 其他项目消费 | 加 `preserve_size` 开关 |
| **Meshy 免费层 CC BY 4.0 需署名** | 商业项目 | 升级 Pro 或用 Tripo 付费 |
| **Hunyuan3D/TRELLIS Windows 踩坑** | 本地 AI 路线 | 用 WSL2 或纯 Linux |
| **Meshroom CUDA only** | history-mist 文物扫描 | 用 Meshroom CL 或云端 |
| **Geometry Nodes 失去运行时动态** | sky-carrier 迁移 | 调研确认 sky-carrier 零顶点变形，风险可控 |
| **replicate_cloud.py:71 并发覆盖 bug** | modelstudio | 新后端用 NamedTemporaryFile |
| **Blender 5.0 GN 资产体系变动** | GN 迁移路线 | 锁定 Blender 4.x LTS |
| **AI 生成角色丢失分层/挂点/换装** | taixu-dao-world | 主角保留手搓，AI 仅做静态展示模型 |
| **voxel-craft draw call 已偏高** | 引入 AI 高模 | 不引入，维持 MultiMesh 体素 |

---

## 十二、下一步行动（按 ROI 排序）

| 优先级 | 动作 | 工时 | 收益 | 风险 |
|---|---|---|---|---|
| **P0** | `future-world-3055` 填 Quaternius CC0 太空包 | 1 小时 | 立即可见，空目录填满 | 0 |
| **P0** | `voxel-craft` 性能基线已文档化（AGENTS.md） | 0 | 移动端预算金标准 | 0 |
| **P1** | `modelstudio` 加 Meshy 后端 + 修 PBR 透传 | 半天 | 工作区通用 AI 建模入口 | trimesh PBR 丢失 |
| **P1** | `taixu-dao-world` 群众换 Quaternius Modular Men | 1-2 天 | NPC 精细度 3→6/10 | InstancedMesh 重构 |
| **P2** | `taixu-dao-world` 主角加命名挂点 | 半天 | 装备系统视觉化基础 | 0 |
| **P2** | `modelstudio` 接 SPAR3D（若有 NVIDIA GPU） | 2-3 天 | 无限免费本地 AI 建模 | 硬件门槛 |
| **P3** | `sky-carrier` Geometry Nodes 迁移 | 4-6 周 | draw call 大降，可外包 | 失去运行时参数化 |
| **P3** | `taixu-dao-world` 主角 Mixamo 化 | 1-2 周 | 专业骨骼动画 | 异步加载重构 |

### modelstudio 加 Meshy 后端的工程清单

新建 1 文件 + 改 6 文件，Web/Server 端零改动（`method` 是宽松 string，UI 选项由 `/probe` 动态渲染）：

| 文件 | 类型 | 行数 | 内容 |
|---|---|---|---|
| `generator/app/backends/meshy_cloud.py` | **新建** | 110-140 | `MeshyCloudBackend` 类 |
| `generator/app/schemas.py:15-21` | 改 | +1 | `GenerationMethod.CLOUD_MESHY = "cloud_meshy"` |
| `shared/types.ts:16-21` | 改 | +1 | 同步加 `'cloud_meshy'` |
| `generator/app/backends/__init__.py` | 改 | +2 | import + `__all__` |
| `generator/app/router.py:9-26` | 改 | +2 | 注册 + `_provider_rank` 归 cloud |
| `generator/app/prober.py:99` | 改 | +20 | `_judge_meshy()`（拷贝 `_judge_cloud_replicate` 模板） |
| `generator/pyproject.toml` | 改 | +1 | 可选 `httpx`（也可零依赖用 stdlib） |

**三个隐藏陷阱**（范本 `replicate_cloud.py` 没解决的）：
1. **HTTP 轮询要自己写**：Meshy 没有官方 Python SDK，需手写 `POST` → 拿 task_id → 循环 `GET` 直到 `succeeded`。
2. **PBR 纹理会丢失**（最严重）：trimesh 的 GLTFExporter 只支持基础 albedo，Meshy 的 PBR 金属/粗糙/法线贴图会降级。需 postprocess 加"透传 glb 字节"分支。
3. **归一化抹掉原始尺寸**：`_center_and_normalize` 会把模型缩放到 `[-1, 1]`，游戏场景里不对。需加 `preserve_size` 开关。

---

## 十三、Sources

### AI 建模评测与对比
- [Best AI for 3D Modeling & CAD – July 2026](https://www.buildmvpfast.com/articles/best-llms-2026-guide/3d-modeling-ai)
- [Best AI 3D Model Generators in 2026 (Medium)](https://generativeai.pub/best-ai-3d-model-generators-in-2026-3eab8a71cc86)
- [Best 3D AI Generator 2026: 5 Tools Tested (YouTube)](https://www.youtube.com/watch?v=1zUWCL5WzAc)
- [Open Source Showdown: TripoSR vs Trellis vs Spar3D](https://www.youtube.com/watch?v=SY62QkN0kwY)
- [State of AI 3D Generation 2026](https://www.3daistudio.com/state-of-ai-3d-generation-2026)

### 开源本地 AI 模型
- [SPAR3D GitHub](https://github.com/Stability-AI/stable-point-aware-3d) / [SPAR3D 公告](https://stability.ai/news-updates/stable-point-aware-3d)
- [Microsoft TRELLIS GitHub](https://github.com/microsoft/TRELLIS) / [TRELLIS.2 项目页](https://microsoft.github.io/TRELLIS.2/)
- [TripoSR GitHub](https://github.com/VAST-AI-Research/TripoSR) / [TripoSR 论文](https://arxiv.org/html/2403.02151v1)
- [TripoSR vs SF3D VRAM 对比](https://www.triposrai.com/)

### 商业云服务
- [Meshy API 文档](https://docs.meshy.ai/en) / [Meshy 定价](https://www.meshy.ai/pricing) / [Meshy 拓扑指南](https://www.meshy.ai/blog/mesh-topology)
- [Tripo 开发者平台](https://developers.tripo3d.com/en/models/v3-1) / [Tripo API 定价](https://developers.tripo3d.ai/en)

### 拓扑清理工作流
- [Evolving 3D Model Topology Practices (Medium)](https://medium.com/@Jamesroha/evolving-3d-model-topology-practices-in-modern-game-development-d81c47ecde3c)
- [AI Retopology Guide](https://www.aiimageto3d.com/blog/ai-retopology-guide)
- [Game-Ready Topology with AI? (YouTube)](https://www.youtube.com/watch?v=3hagi51IxeY)

### Blender 自动化
- [Khronos: Blender as glTF Converter](https://github.khronos.org/glTF-Tutorials/BlenderGltfConverter/)
- [Command-line GLB Export (Gist)](https://github.com/jakelazaroff/til/blob/main/blender/export-a-blender-file-to-glb-from-the-command-line.md)
- [Blender glTF Batch Export](https://devtalk.blender.org/t/gltf-batch-export/16618)
- [Procedural Game Asset Creation with Geometry Nodes (Kodeco)](https://www.kodeco.com/38674958-procedural-game-asset-creation-with-geometry-nodes-in-blender)

### 角色 / 动画
- [Mixamo 官网](https://www.mixamo.com/) / [Adobe HelpX Mixamo](https://helpx.adobe.com/creative-cloud/help/mixamo-rigging-animation.html)
- [Ready Player Me 关停 (Reddit)](https://www.reddit.com/r/Unity3D/comments/1q50157/netflix_acquires_ready_player_me_which_will_end/)
- [MetaPerson 替代品](https://avatarsdk.com/ready-player-me-alternative/)
- [Tripo Auto-Rigging](https://www.tripo3d.ai/features/ai-auto-rigging)
- [Meshy AI Animation Generator](https://www.meshy.ai/features/ai-animation-generator) / [Meshy auto-rigging 教程](https://www.meshy.ai/tutorials/character-auto-rigging-workflow)
- [Neural4D auto-rigging](https://neural4d.com/features/auto-rigging)

### 风格一致性
- [Text-to-3D in 2026: Honest Comparison](https://nhance-school.com/articles/best-ai-3d-generators-2026)
- [Meshy vs Tripo vs Rodin vs Hunyuan (Reddit)](https://www.reddit.com/r/aigamedev/comments/1ucirid/meshy_vs_tripo_vs_rodin_vs_hunyuan_for_game/)
- [Meshy prompt engineering 案例](https://www.meshy.ai/blog/3D-prompt-engineering)

### 摄影测量
- [Meshroom](https://meshroom.org/) / [AliceVision](https://alicevision.org/) / [Meshroom Sketchfab 教程](https://meshroom-manual.readthedocs.io/en/latest/tutorials/sketchfab/sketchfab.html)
- [RealityScan Mobile](https://www.realityscan.com/mobile)
- [Meshroom GPU 讨论](https://github.com/alicevision/Meshroom/discussions/2727)

### 移动端预算
- [Android Game Dev: Textures](https://developer.android.com/games/optimize/textures)
- [NVIDIA ASTC 指南](https://developer.nvidia.com/astc-texture-compression-for-game-assets)
- [Unity: triangles per draw call](https://discussions.unity.com/t/how-many-triangles-polygons-is-a-draw-call-worth/106497)

### 免费资产库
- [Quaternius](https://quaternius.com/) / [Ultimate Space Kit](https://poly.pizza/bundle/Ultimate-Space-Kit-YWh743lqGX)
- [Poly Pizza](https://poly.pizza/) / [Kenney](https://kenney.nl)

### 引擎 GLB 加载
- [Godot: Importing 3D scenes](https://docs.godotengine.org/en/4.1/tutorials/assets_pipeline/importing_scenes.html)
- [Babylon: loadAssetContainerAsync](https://doc.babylonjs.com/typedoc/functions/BABYLON.loadAssetContainerAsync)
- [Babylon: PBR 应用到 gltf mesh](https://forum.babylonjs.com/t/applying-pbr-texture-to-imported-gltf-mesh-file/31819)
