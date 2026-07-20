# D:\M_X_M 项目前景梳理

更新日期：2026-07-07

## 一、总体判断

当前工作区共有 17 个顶层项目，主线已经从“单点 AI demo”扩展为四类方向：

1. Agent 基础设施与 Agent 产品：`agentloop`、`cogent`、`tripplan`、`agentapp`、`agentresearch`
2. AI 工程平台与自动化测试：`e2e-fusion`、`computer-use-rtc`
3. 轻量/端侧 AI：`lightai`、`tiny-edge-models`
4. 可视化、游戏和内容型产品：`kids-games`、`wildera`、`future-world-3055`、`taixu-dao-world`、`modelstudio`、`dashan`、`agenttrain`、`ai-expansion-analysis`

最值得继续投入的项目不是“看起来最大”的，而是同时满足三个条件的：市场需求明确、技术壁垒可沉淀、现有代码已经形成可运行骨架。按这个标准，优先级建议如下。

## 二、最有发展前景的项目

| 优先级 | 项目 | 前景判断 | 核心发展方向 |
|---|---|---|---|
| S | `e2e-fusion` | 最有平台化潜力，适合做长期主线 | AI 驱动的跨端 E2E 验证平台 |
| S | `agentloop` + `cogent` | 可沉淀为内部 Agent 框架底座 | 从命令式循环走向声明式 Agent 编排 |
| S | `lightai` | 差异化清晰，适合做“低成本 AI 工具箱” | 轻量算法替代不必要的大模型调用 |
| A | `tripplan` | 具备产品样板价值 | 状态驱动的垂直 Agent 应用 |
| A | `tiny-edge-models` | 方向前沿，和端侧 AI 趋势吻合 | 端侧小模型研究 + 训练流水线 |
| A | `agentapp` | 移动端入口明确 | 个人多 Agent 助手 App |
| A | `kids-games` | 内容规模大，适合静态产品化 | 儿童教育小游戏合集 |
| B | `modelstudio` | 技术想象力强，但工程成本高 | 2D 到 3D 生成与编辑工具 |
| B | `wildera` / `future-world-3055` / `taixu-dao-world` | 展示性强，商业化路径更长 | 3D 世界原型与游戏技术沉淀 |

## 三、重点项目发展方向

### 1. `e2e-fusion`：跨端验证平台

**为什么有前景**

跨 Web、Electron、原生 Windows、API、CLI 的验证需求是真实工程痛点；项目已经是 pnpm monorepo，包含 platform、runner、worker、drivers、orchestrator、storage、reporter、agent-core 等模块，具备平台雏形。相比普通 Playwright 项目，它的差异点在“多端一致性 + AI 测试生成/自愈 + 报告平台”。

**建议发展方向**

- 先聚焦一个真实业务场景，例如“Web 管理台 + Windows 客户端 + API 数据一致性”。
- 把 DSL、Runner、报告链路打磨成稳定闭环：写用例、执行、失败定位、报告回放。
- AI Agent 不要一开始追求全自动，应先做“文档转用例草稿”“失败摘要”“选择器修复建议”。
- 增加可演示的模板项目和一键 demo，降低理解成本。

**下一阶段里程碑**

- M1：固定 3 个黄金示例用例，保证 `pnpm build`、`pnpm test`、runner 示例稳定。
- M2：平台端支持用例 CRUD、执行历史、报告查看。
- M3：接入一个真实 Windows/桌面程序验证流。
- M4：AI 生成测试和失败分析只覆盖高置信场景。

### 2. `agentloop` + `cogent`：Agent 基础设施

**为什么有前景**

`agentloop` 已经覆盖上下文压缩、流式、可观测、OTel、并行 sub-agent、记忆、HITL、轨迹评估等能力；`cogent` 在此基础上做 JSON/YAML 声明式 Agent。二者组合后可以从“写代码调 loop”升级到“声明 Agent 蓝图并编译执行”，这是可复用资产。

**建议发展方向**

- 将 `agentloop` 定位为执行内核，只关心可靠循环、工具调用、状态、评估。
- 将 `cogent` 定位为声明层，负责 agent spec、workflow、extends、template、validate。
- 抽出稳定 schema，形成 `agent.yaml`/`workflow.yaml` 规范。
- 做 3 个垂直样例：研究 Agent、客服 Agent、测试 Agent。

**下一阶段里程碑**

- M1：统一 `agentloop` 与 `cogent` 的工具 schema 和 trace 格式。
- M2：`cogent validate` 给出可读错误和修复建议。
- M3：支持 workflow 可视化或 trace 回放。
- M4：把 `tripplan` 或 `e2e-fusion` 的 Agent 部分迁移为声明式样例。

### 3. `lightai`：轻量 AI 算法工具箱

**为什么有前景**

它的定位很清楚：不是所有问题都要上大模型。BM25、朴素贝叶斯、MinHash/SimHash、Z-score/IQR、决策树等轻量算法在高频、确定性、可解释、低成本业务中有长期价值。项目已经有 Python 库、Web 前端、测试和多个场景页面。

**建议发展方向**

- 继续坚持“核心零依赖”，这是项目特色。
- 从算法展示转向业务工具：搜索、去重、分类、异常检测、推荐、规则引擎。
- 每个场景都提供输入样例、效果解释、何时不用 LLM 的判断标准。
- 做成内部可复用 Python 包 + Web Playground。

**下一阶段里程碑**

- M1：补齐每个算法的 README、复杂度、适用边界。
- M2：统一 Web demo 的数据导入、结果导出和解释面板。
- M3：增加真实业务数据模板，如工单分类、日志异常、商品去重。
- M4：和大模型做成本/延迟/稳定性对比报告。

### 4. `tripplan`：状态驱动旅行规划 Agent

**为什么有前景**

它不是让 LLM 一次性生成 JSON，而是让 Agent 通过工具修改可变行程状态，并用确定性逻辑重排时间链。这种“Agent 操作状态而不是编文本”的模式很适合做垂直产品，也能反哺 `agentloop`/`cogent`。

**建议发展方向**

- 把它作为垂直 Agent 产品样板，而不是泛旅行聊天机器人。
- 强化地图、时间链、预算、交通、用户偏好和多人同行约束。
- 增加“方案比较”和“修改原因解释”，让用户信任每次调整。
- 支持导出日程、地图路线、费用清单。

**下一阶段里程碑**

- M1：完善北京/上海/杭州等种子城市离线演示。
- M2：加入预算、开放时间、排队时间、餐饮偏好约束。
- M3：支持多方案生成与对比。
- M4：将 Agent spec 化，作为 `cogent` 的标杆样例。

### 5. `tiny-edge-models`：端侧小模型研究与训练

**为什么有前景**

端侧 AI 是明确趋势，项目已经从研究报告扩展到 `train/` 训练流水线，覆盖图像分类、检测、关键词唤醒、文本分类等方向。它和 `lightai` 能形成互补：一个强调经典轻量算法，一个强调小模型训练与部署。

**建议发展方向**

- 从报告项目升级为“端侧模型生成器”。
- 明确目标设备档位：MCU、手机、普通 PC、浏览器 WASM/WebGPU。
- 优先做小而完整的闭环：数据配置、训练、量化、ONNX 导出、端侧推理 demo。
- 输出模型卡：大小、延迟、准确率、适用设备。

**下一阶段里程碑**

- M1：跑通一个文本分类或图像分类最小训练例子。
- M2：加入 INT8/INT4 量化与 ONNXRuntime 推理验证。
- M3：做浏览器端或本地 CLI 推理 demo。
- M4：与 `lightai` 建立“轻算法优先，小模型兜底，大模型最后”的决策流程。

### 6. `agentapp`：移动端多 Agent 助手

**为什么有前景**

移动端是用户最自然的 AI 入口，项目已经有 Flutter、Riverpod、SQLite、安全存储、流式对话、长期记忆和多 Agent 路由设计。它适合作为前台产品入口，复用后续 `agentloop`/`cogent` 的能力。

**建议发展方向**

- 不要只做通用聊天，重点做“本地长期记忆 + 场景助手”。
- 增加手机端自然能力：通知、日程、剪贴板、分享入口、语音输入。
- 支持本地/云端混合模型配置。
- 做隐私卖点：数据本地存储、密钥本机保存。

**下一阶段里程碑**

- M1：稳定 Android 端核心聊天、历史、设置。
- M2：加入记忆管理和用户可编辑记忆。
- M3：接入 2-3 个手机原生工具。
- M4：将 Agent 配置改造成声明式，和 `cogent` 对齐。

### 7. `kids-games`：儿童教育小游戏合集

**为什么有前景**

它已经从 49 个扩展到 README 宣称 81 个游戏，规模足够形成内容产品。零第三方游戏引擎、Vite + TS + Canvas/DOM/SVG 的静态形态也适合部署、分发和长期维护。

**建议发展方向**

- 先统一 README 和 `package.json` 描述，目前 package 仍写 8 个游戏。
- 把 81 个游戏按年龄、能力、难度、时长做结构化索引。
- 增加家长面板：进度、偏好、练习建议、屏幕时间。
- 建立核心玩法质量标准，避免数量增长导致体验稀释。

**下一阶段里程碑**

- M1：核对实际游戏注册表，统一文档数字。
- M2：每个游戏加入目标能力、难度、推荐年龄、完成状态。
- M3：增加本地进度和家长报告。
- M4：做 PWA 离线安装。

## 四、可作为展示/中期孵化的项目

### `modelstudio`

方向有想象力：2D 图片到 3D 模型生成，再在浏览器编辑并导出 glb。但工程链路重，依赖 Node、Fastify、Python、PyTorch、Three.js/R3F。建议暂时作为技术样机推进，先保证“上传图片 -> 生成/占位 glb -> 浏览器编辑 -> 导出”的端到端稳定。

### `wildera`

WebGPU 开放世界生存建造方向展示性强，已有 Babylon.js 8、Havok、chunk terrain、weather、e2e 报告等信号。建议把它定位为 3D 技术验证场：地形流式加载、物理、建造、天气、存档、AI 生物。商业化要谨慎，开放世界游戏成本很高。

### `future-world-3055`

世界观和系统野心强，适合做科幻 3D 世界原型。建议优先打磨“一个高质量可玩切片”，例如学院城市 + 一个遗迹任务 + 一次星际跃迁，而不是继续横向扩展设定。

### `taixu-dao-world`

仙侠开放世界题材有内容辨识度。建议和 `future-world-3055`、`wildera` 共享 3D 世界基础能力：角色移动、相机、地形、任务、交互、性能监控。避免三个 3D 项目重复造底层轮子。

## 五、建议收敛或低优先级维护的项目

| 项目 | 建议 |
|---|---|
| `agentresearch` | 保留为研究笔记和最小 Agent 教学项目，不作为产品主线。当前 Git 中大量未跟踪文件，应先决定是否入库。 |
| `agenttrain` | 作为 Canvas 游戏与 AI 顾问实验保留，可并入游戏/教育方向，不建议单独长期投入。 |
| `dashan` | 创意强、适合展示，但商业和工程复用有限。可作为内容型 demo 或 prompt/叙事实验。 |
| `ai-expansion-analysis` | 适合作为静态报告页模板，后续可并入研究展示，不建议作为核心工程。 |
| `computer-use-rtc` | 业务针对性很强，适合沉淀到 `e2e-fusion` 的 Windows/视觉自动化插件中。 |

## 六、推荐路线图

### 近期 2 周

- 固化主线：`e2e-fusion`、`agentloop`、`cogent`、`lightai`
- 清理文档：统一 `kids-games` 数量描述，更新根目录总览
- 版本管理：确认 `agentresearch` 未跟踪文件是否全部入库
- 做 3 个可演示入口：`e2e-fusion` demo、`lightai` Web、`tripplan` demo

### 1-2 个月

- `e2e-fusion` 打通平台端执行与报告闭环
- `agentloop`/`cogent` 抽出稳定 Agent spec
- `lightai` 完成业务场景模板
- `tripplan` 支持多方案、预算、偏好约束
- `tiny-edge-models` 跑通一个训练到端侧推理闭环

### 3-6 个月

- 形成一个“AI 工程工具套件”叙事：
  - `agentloop`/`cogent`：Agent 开发
  - `e2e-fusion`：Agent/软件测试
  - `lightai`/`tiny-edge-models`：低成本与端侧 AI
  - `agentapp`/`tripplan`：前台产品样板
- 3D 游戏类项目只保留一个主技术栈，其他作为世界观或素材实验。

## 七、最终建议

如果只能选 3 个长期投入：

1. `e2e-fusion`：最像可平台化产品，工程价值最高。
2. `agentloop` + `cogent`：最适合沉淀通用 Agent 技术资产。
3. `lightai`：差异化最清楚，能避开“大模型套壳”同质化。

如果还能保留 2 个产品样板：

4. `tripplan`：验证状态驱动 Agent 应用。
5. `agentapp`：作为移动端入口。

如果做展示和内容增长：

6. `kids-games`：最适合静态部署和内容型增长。

