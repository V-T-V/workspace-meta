# M_X_M 工作区总览梳理

> 更新日期：2026-07-16 · 本文档是工作区的**权威梳理**，覆盖所有项目、分类、状态与关联关系。
> 各项目细节见其目录下的 `AGENTS.md`（部分项目已有，其余可按需补建）。

## 一、工作区性质

D:\M_X_M 是一个**多项目工作区，不是 monorepo**：32 个项目相互独立，无统一依赖管理、无 workspace 配置。每个项目有自己的技术栈、运行方式。根目录另有产品化管理元层（规划/状态文档 + 校验脚本）。

---

## 二、项目总表（35 个）

### A. Agent / AI 基础设施线（8 个）
| 项目 | 技术栈 | 定位 | 完成度 |
|------|--------|------|--------|
| [`agentloop`](./agentloop) | TS，零运行时依赖 | Agent 主循环执行内核（10+ 能力全落地） | 高（47文件/33测试） |
| [`cogent`](./cogent) | TS，复用 agentloop | 声明式 Agent 框架（JSON/YAML 蓝图→编译执行） | 中（11文件/5示例） |
| [`e2e-fusion`](./e2e-fusion) | pnpm monorepo + Next.js + Playwright + Prisma | 跨端联合 E2E 验证平台（Web/桌面/API/CLI + AI Agent） | 很高（3apps/15packages） |
| [`agentapp`](./agentapp) | Flutter + Riverpod | 多专业 Agent 助手 App（工作/学习/生活） | 高（27文件/9测试） |
| [`agentresearch`](./agentresearch) | TS，零运行时依赖 | 可运行 ReAct Agent + 研究笔记仓库 | 高（25测试） |
| [`tripplan`](./tripplan) | Vite + TS + 高德 AMap | 旅行规划 Agent（可变行程状态 + 多方案迭代） | 中高 |
| [`stock-ai`](./stock-ai) | TS，零依赖内核 | 跨市场 ReAct Agent（技术面+基本面+舆情+回测） | 中高（36文件） |
| [`go-rmm`](./go-rmm) | Go 1.22+ | 远程自动化执行平台（反向WS中继+Win Agent服务化+6类任务执行器） | 高（P0完成，32文件/9测试/三组件全链路打通） |

### B. AI 应用 / 研究 Agent（3 个）
| 项目 | 技术栈 | 定位 | 完成度 |
|------|--------|------|--------|
| [`history-mist`](./history-mist) | TS + Hono + better-sqlite3 + OpenAI | 历史探索智能体（多源交叉验证+知识图谱+报告导出） | 高（42文件） |
| [`dashan`](./dashan) | TS (Vite)，无运行时依赖 | "大善系统"哲学困境对话（web/server/cli 三态） | 高 |
| [`computer-use-rtc`](./computer-use-rtc) | Node + PowerShell + Win32 C# + GLM 视觉 | RTC 视频通话 E2E 自动化（截图→视觉识别→点击） | 中 |

### C. 技术教育与内容平台（5 个）
| 项目 | 技术栈 | 定位 | 完成度 |
|------|--------|------|--------|
| [`algorithms-atlas`](./algorithms-atlas) | Vite + TS strict | 3000 算法图谱（trace 录制+动画播放器） | 很高（2629文件/334算法） |
| [`kids-games`](./kids-games) | Vite + TS PWA | 3~6 岁儿童 81 个网页小游戏合集 | 很高（81游戏/10测试） |
| [`lightai`](./lightai) | Python 零依赖 + HTML | 轻量 AI 算法库（BM25/贝叶斯/MinHash 替代大模型） | 很高（101 .py/31页面） |
| [`ai-expansion-analysis`](./ai-expansion-analysis) | 纯 HTML/CSS/JS | AI 扩张方向/深度/场景可视化分析站 | 高 |
| [`superpower-system`](./superpower-system) | Vite + TS strict | 个人能力 RPG 化解锁面板（能力树+经验+独立 ReAct agent 教练） | 中（首版34测试全绿） |

### D. 3D 游戏与虚拟世界（4 个）
| 项目 | 技术栈 | 定位 | 完成度 |
|------|--------|------|--------|
| [`wildera`](./wildera) | Vite + TS + Babylon.js 8 (WebGPU) + Havok | 3D 开放世界生存建造游戏（类英灵神殿） | 中高（62文件） |
| [`future-world-3055`](./future-world-3055) | Vite + TS + Babylon.js 7 + Havok | 3055 年星河文明 3D 模拟世界 | 中高（69文件/ECS） |
| [`taixu-dao-world`](./taixu-dao-world) | Vite + TS + Three.js | 太虚万象录：浏览器 3D 仙侠修真开放世界 | 中高（44文件） |
| [`modelstudio`](./modelstudio) | Python + Node + React+Three.js | 2D→3D 模型生成与网格编辑器（M1-M3完成） | 中高（三层架构） |

### E. 研究 / 调研（5 个）
| 项目 | 技术栈 | 定位 | 完成度 |
|------|--------|------|--------|
| [`大模型研究`](./大模型研究) | Markdown + PyTorch | 大模型/DLM 研究笔记（23篇理论+22代码模块） | 高 |
| [`ai-world-research`](./ai-world-research) | 纯 Markdown | AI 大力发展后社会/经济影响的思想实验集（8篇，与 ai-expansion-analysis 互补） | 高（首版8篇完整） |
| [`tiny-edge-models`](./tiny-edge-models) | research.md + HTML + Python | 端侧/微小模型调研 + edge_trainer 训练框架 | 中高 |
| [`agenttrain`](./agenttrain) | Vite + TS Canvas | Mini Metro 风火车调度游戏（+AI 顾问/自动驾驶） | 中高（18文件/7测试） |
| [`web-standards-research`](./web-standards-research) | MD + HTML/JS + Python + Wasm | 现代化 Web 标准横评（**30 标准**：3 批 × 各 5 主题）含 30 个详细可运行 demo（含 4 个🆕进阶版：WebGPU 粒子/Next.js/多人会议/Sobel） + **30 份深度剖析档案**（每标准一份）+ **Web 平台演进深度报告**（598 行） | 高（30档案+30深挖+30 demo+4进阶+深度报告） |
| [`chinese-philosophy-research`](./chinese-philosophy-research) | TS + Hono + better-sqlite3 | 中国儒释道发展研究 agent（内置经典语料 + ReAct 研究 + 对话问答 + 多视角报告） | 初版（M1 进行中） |

### F. 集成 / 元层（2 个）
| 项目 | 技术栈 | 定位 | 完成度 |
|------|--------|------|--------|
| [`platform-demo`](./platform-demo) | JSON 契约 + Node 校验 | cogent+agentloop+e2e-fusion 三平台集成演示（非独立项目） | 低（胶水层） |
| [`pastoral-travel`](./pastoral-travel) | Godot 4.7 + Node + Python | 田园旅行记：拍照+AI识别+村庄建设手游 | 中高（已出3个APK） |

---

## 三、2026-07-13 新增项目专项梳理

本次新增的 6 个项目已单独形成 [NEW_PROJECTS_REVIEW_2026-07-13.md](./NEW_PROJECTS_REVIEW_2026-07-13.md)，不建议把它们全部变成独立主线：

| 项目 | 新定位 | 优先级 |
|---|---|---|
| `logos-formal` + `logos-sim` | 哲学计算实验室：形式证明 + 确定性模拟 | P1 |
| `sky-carrier` | 工程物理仿真与硬科幻任务体验 | P1 |
| `poetry-garden` + `wuwan` | 东方文化内容线：主应用 + 专题长卷 | P1/P2 |
| `agent-coding-research` | AI 技术趋势知识库的研究上游 | P2 |

新增项目的详细判断、产品边界、90 天路线和风险见专项文档。

### 2026-07-16 新增：科学模拟与交互教育线（3 个）

详细梳理见 [NEW_PROJECTS_REVIEW_2026-07-16.md](./NEW_PROJECTS_REVIEW_2026-07-16.md)。

| 项目 | 技术栈 | 当前判断 | 优先级 |
|---|---|---|---|
| `fusion-power-3d` | Vite + TS + Three.js | 三类聚变路线、运行系统和方案对比已形成垂直产品 | P1 |
| `physics-sim` | Vite + TS + Babylon.js + Havok | 通用物理场景容器，物理与渲染分离，适合作为科学模拟底座 | P1 |
| `earth-history-3d` | vinext + React + Canvas 2D | 视觉成熟的地球历史内容原型，尚非真正 3D 或完整知识产品 | P2 |

## 四、项目间关联关系（核心）

### 1. Agent 执行栈依赖链（最重要的关联）
```
         cogent（声明式蓝图）
            │ 编译
            ▼
         agentloop（执行内核 runLoop）  ←─── agentresearch（同源轻量 ReAct）
            │ 可被复用
            ├──────────────┐
            ▼              ▼
        tripplan        stock-ai       （均用 ReAct/Agent 模式，但独立实现）
        dashan(LLM复用)
            │
            ▼
      platform-demo（集成 cogent+agentloop+e2e-fusion 的契约胶水）
            │
            ▼
      e2e-fusion（跨端验证平台，可验证上述 Agent 系统）
```

**关键关联**：
- **`cogent` 依赖 `agentloop`**：cogent 把 JSON/YAML 认知蓝图**编译成 agentloop 的 runLoop() 调用**。这是工作区唯一的强代码依赖。
- **`agentresearch` 与 `agentloop` 同源**：都是最小 ReAct 循环，前者教学/研究、后者工程基座。agentresearch PRODUCT.md M3 计划对齐 agentloop。
- **`platform-demo` 是集成层**：非独立项目，是 cogent + agentloop + e2e-fusion 三者的产品契约 + 垂直切片证据。
- **`agentapp` 独立**：虽是 Agent 产品形态，但不依赖 agentloop/cogent 代码（自含 router agent）。
- **`tripplan`/`stock-ai`/`dashan`/`history-mist`**：各自独立实现 Agent 逻辑（带 StubLLM 离线回退），未直接依赖 agentloop。

### 2. 技术栈聚类
- **Babylon.js 3D 游戏系**：`wildera`(Babylon 8 WebGPU) / `future-world-3055`(Babylon 7) / `taixu-dao-world`(Three.js) —— 相互独立但技术栈相近，可互鉴引擎代码。
- **modelstudio 也用 Three.js**（web 层），但定位是工具不是游戏。
- **Vite + TS strict + 零运行时依赖**：agentloop / agentresearch / dashan / agenttrain 共享此范式（node --test + ESLint + Prettier）。
- **Python 系**：lightai / 大模型研究 / tiny-edge-models(train) / modelstudio(generator)。

### 3. 产品化元层（根目录）
| 文档/目录 | 作用 |
|-----------|------|
| `PROJECT_PROSPECTS_2026-07-07.md` | 项目前景初版梳理 |
| `PROJECT_PROSPECTS_REFRESH_2026-07-08.md` | 含新增项目的刷新版（定义了 5 条产品线） |
| `PRODUCTIZATION_ROADMAP_2026-07-07.md` | TripPlan/AgentApp/KidsGames 产品化路线 |
| `PRODUCTIZATION_STATUS_2026-07-08.md` | 产品化组合状态 |
| `STRATEGIC_PLAN_E2E_AGENT_PLATFORM_2026-07-07.md` | e2e-fusion+agentloop+cogent 战略规划 |
| `project-overview.html` | 工作区项目梳理 HTML 展示页 |
| `scripts/` | 3 个 .mjs 校验脚本（platform/product portfolio check） |
| `.portfolio/` | 产品状态快照 JSON |

### 3.1 研究资产（根目录，基于真实代码核实 + 行业对标）
| 文档 | 作用 |
|-----------|------|
| [`GAME_ENGINE_RESEARCH.md`](./GAME_ENGINE_RESEARCH.md) | Web 游戏引擎选型（基于 7 个项目代码核实 + 2026 行业现状） |
| [`3D_MODELING_RESEARCH.md`](./3D_MODELING_RESEARCH.md) | **低成本 3D 建模方案**（基于 8 个 3D 项目代码核实 + 2026 业界 AI 建模/CC0/摄影测量/Blender GN 全景） |

### 3.2 AI 介入孪生篇
- `ai-expansion-analysis`（C 类，数据仪表盘）回答 **"AI 现在渗透多深"**：18 领域 × L0–L4 × 2018–2027 的可视化分析站，含动态评估引擎。
- `ai-world-research`（E 类，叙事推演）回答 **"AI 之后世界会怎样"**：8 篇思想实验（就业/经济/权力/真相/意义/风险/终局），立足 2026 推演 2026–2040。
- 同主题不同切面，可交叉引用：前者提供现状数据底盘，后者提供未来推演叙事。

### 4. AGENTS.md 覆盖现状
已有顶层 AGENTS.md（13 个）：agentapp / agentloop / agentresearch / agenttrain / dashan / kids-games / modelstudio / logos-formal / logos-sim / poetry-garden / sky-carrier / physics-sim / fusion-power-3d。
`fusion-power-3d` 另有 5 个子模块 AGENTS.md。`earth-history-3d` 尚待补建。

---

## 五、优先级建议（综合 REFRESH 文档）

**第一梯队（最值得产品化）**：
1. `e2e-fusion` —— 平台化潜力最高（真实工程痛点）
2. `agentloop` + `cogent` —— 通用技术底座（执行+声明）
3. `algorithms-atlas` —— 可规模化的技术教育内容平台
4. `history-mist` —— 垂直研究 Agent（知识图谱+报告价值）
5. `fusion-power-3d` —— 当前最成熟的科学工程垂直模拟产品
6. `physics-sim` —— 可沉淀为科学模拟运行时与课程实验底座

**特色项目**：
- `pastoral-travel` —— 唯一已产出可分发产物（3 个 Android APK）
- `wildera` —— 3D 游戏完成度最高

---

## 六、备注
- 根目录 `nul` 文件（51 字节）：Windows 下 `nul` 设备名误写产物，非项目内容，建议清理。
- 工作区无 git 仓库根（各子项目可能各自有 .git）。
- 日期集中度：产品化元层文档均在 2026-07-07~08，本梳理为 2026-07-10。
