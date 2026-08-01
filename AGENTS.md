# M_X_M 工作区 · AGENTS.md

这是多项目工作区，**不是 monorepo**：32 个项目相互独立，无共享代码、无 workspace 配置、无统一依赖管理。每个项目有自己的技术栈与运行方式。

> **完整梳理（含关联关系图、优先级、状态）见 [`WORKSPACE_OVERVIEW.md`](./WORKSPACE_OVERVIEW.md)。**
> 进入任何子项目前，先读它的 `AGENTS.md`（部分已有，其余待补）。

## 项目导航（按产品线分组）

> 注：本表登记的主要项目，实际工作区目录数（含未登记的实验/草稿）由 `workspace-ops scan` 给出权威统计（当前约 60 个）。

### A. Agent / AI 基础设施
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`agentloop`](./agentloop) | TS 零依赖 | Agent 主循环执行内核 | [→](./agentloop/AGENTS.md) |
| [`cogent`](./cogent) | TS，复用 agentloop | 声明式 Agent 框架（蓝图→编译执行） | [→](./cogent/AGENTS.md) |
| [`e2e-fusion`](./e2e-fusion) | pnpm + Next.js + Playwright | 跨端 E2E 验证平台 | 待补 |
| [`agentapp`](./agentapp) | Flutter + Riverpod | 多专业 Agent 助手 App | [→](./agentapp/AGENTS.md) |
| [`agentresearch`](./agentresearch) | TS 零依赖 | ReAct Agent + 研究笔记 | [→](./agentresearch/AGENTS.md) |
| [`tripplan`](./tripplan) | Vite + TS + 高德 | 旅行规划 Agent | [→](./tripplan/AGENTS.md) |
| [`stock-ai`](./stock-ai) | TS 零依赖 | 跨市场 ReAct Agent | [→](./stock-ai/AGENTS.md) |
| [`go-rmm`](./go-rmm) | Go 1.22+ | 远程自动化执行平台（反向WS中继+Win Agent服务化+6类任务执行器） | [→](./go-rmm/AGENTS.md) |
| [`auto-finance-assistant`](./auto-finance-assistant) | Go 1.25 + Vue 3 + SQLite + Ollama | 汽车金融本地智能客服（单EXE+FAQ短路+FTS/向量RAG+金融计算+服务化，M1-M9全完成） | [→](./auto-finance-assistant/AGENTS.md) |
| [`go-agent-research`](./go-agent-research) | Go 1.25 零依赖 | Agent 范式研究（73范式/56家族 + 12大基础设施：trace/bench/combo/ablation/mockgen/metaselect/llmadapt/toolkit/iout/benchviz可视化/unified注册表/webui导航站 + 横向基准热力图/雷达图/排名表，含2025前沿AZR/MoA/DeepResearch/JEPA/Dreamer/TestTimeCompute/GRPO，纯标准库+Mock/Ollama，836测试） | [→](./go-agent-research/AGENTS.md) |
| [`rust-agent-research`](./rust-agent-research) | Rust 1.97 + tokio | go-agent-research 的 Rust 全量移植（70范式/56家族**全部完整实现**，84 crate，430测试全绿，clippy/fmt零warning，async/await+Arc+Cargo workspace，对齐Go版设计） | [→](./rust-agent-research/AGENTS.md) |
| [`gpu-mesh`](./gpu-mesh) | Go 1.22+ | 异地分布式 GPU 算力调度平台（反向WS穿透NAT+Win服务化+GPU监控仪表盘+让位调度+Ollama/llama.cpp双引擎+OpenAI网关+GPU感知调度+批量Map-Reduce+LoRA训练+mTLS/审计/多租户，全6 Phase完成） | [→](./gpu-mesh/AGENTS.md) |
| [`realtime-digital-human`](./realtime-digital-human) | Python 3.10 + FastAPI + asyncio | 单机实时数字人（ASR→LLM→TTS→唇形 流式管线重叠，首响应<1.5s，4060 8GB 可跑） | [→](./realtime-digital-human/AGENTS.md) |

### B. AI 应用 / 研究 Agent
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`history-mist`](./history-mist) | TS + Hono + sqlite | 历史探索智能体（知识图谱+报告） | [→](./history-mist/AGENTS.md) |
| [`dashan`](./dashan) | TS Vite + Node | "大善系统"哲学对话（LLM困境+善恶簿+3结局+8分类+收藏夹+统计里程碑+CLI/网页双形态，79测试） | [→](./dashan/AGENTS.md) |
| [`computer-use-rtc`](./computer-use-rtc) | Node + PowerShell + GLM 视觉 | RTC 视频自动化（截图→识别→点击） | [→](./computer-use-rtc/AGENTS.md) |

### C. 技术教育与内容平台
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`algorithms-atlas`](./algorithms-atlas) | Vite + TS strict | 3000 算法图谱（动画演示） | [→](./algorithms-atlas/AGENTS.md) |
| [`kids-games`](./kids-games) | Vite + TS PWA | 3~6 岁 529 个儿童小游戏（30成就+自适应难度+PWA+家长报告+反馈系统，91测试） | [→](./kids-games/AGENTS.md) |
| [`lightai`](./lightai) | Python 零依赖 | 轻量 AI 算法库（替代大模型） | [→](./lightai/AGENTS.md) |
| [`ai-expansion-analysis`](./ai-expansion-analysis) | 纯 HTML/JS | AI 扩张可视化分析站 | [→](./ai-expansion-analysis/AGENTS.md) |
| [`frontend-toolbox`](./frontend-toolbox) | Vite + TS strict | 纯前端开发者工具箱（47 工具：JSON/CSV/编码/加密/文本/图片批量压缩/二维码/JS运行/AST/代码分析，懒加载+本地+双击打开） | [→](./frontend-toolbox/AGENTS.md) |

### D. 3D 游戏与虚拟世界
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`wildera`](./wildera) | Vite + Babylon.js 8 | 3D 开放世界生存建造游戏 | [→](./wildera/AGENTS.md) |
| [`future-world-3055`](./future-world-3055) | Vite + Babylon.js 7 | 3055 星河文明 3D 模拟 | [→](./future-world-3055/AGENTS.md) |
| [`taixu-dao-world`](./taixu-dao-world) | Vite + Three.js | 太虚万象录仙侠修真世界 | [→](./taixu-dao-world/AGENTS.md) |
| [`modelstudio`](./modelstudio) | Python+Node+React+Three | 2D→3D 模型生成与编辑器 | [→](./modelstudio/AGENTS.md) |

### E. 研究 / 调研
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`大模型研究`](./大模型研究) | MD + PyTorch | 大模型研究笔记（23理论+22代码） | [→](./大模型研究/AGENTS.md) |
| [`ai-world-research`](./ai-world-research) | MD 研究资产 | AI 大力发展后社会/经济影响思想实验集（8 篇，ai-expansion-analysis 的孪生叙事篇） | [→](./ai-world-research/AGENTS.md) |
| [`tiny-edge-models`](./tiny-edge-models) | MD + HTML + Python | 端侧模型调研+训练框架 | [→](./tiny-edge-models/AGENTS.md) |
| [`agenttrain`](./agenttrain) | Vite + TS Canvas | Mini Metro 风火车调度游戏（8倍大地图+摄像机缩放/平移+6道具插件化+20成就+音效+迷你地图+难度档+最高分+AI顾问/自驾+教程/暂停菜单+视口剔除，208测试） | [→](./agenttrain/AGENTS.md) |

### F. 集成 / 元层
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`platform-demo`](./platform-demo) | JSON 契约 | cogent+agentloop+e2e-fusion 集成胶水 | [→](./platform-demo/AGENTS.md) |
| [`pastoral-travel`](./pastoral-travel) | Godot + Node + Python | 田园旅行记手游（已出APK） | [→](./pastoral-travel/AGENTS.md) |
| [`generic-admin`](./generic-admin) | Go 1.25 + Vue 3 + SQLite | 通用 schema-driven 管理后台（多导出器可插拔，给无后端项目接入） | [→](./generic-admin/AGENTS.md) |

### G. 新增内容与工程实验
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`agent-coding-research`](./agent-coding-research) | MD 研究资产 | AI 编程趋势研究（14篇md + 35份AICon PPT分析） | [→](./agent-coding-research/AGENTS.md) |
| [`app-game-research`](./app-game-research) | MD 研究资产 | 手机游戏外部技术调研（市场/引擎/Godot源码剖析） | [→](./app-game-research/AGENTS.md) |
| [`logos-formal`](./logos-formal) | Lean 4 | 形式化哲学证明库 | [→](./logos-formal/AGENTS.md) |
| [`logos-sim`](./logos-sim) | TS 零依赖 | 哲学场景确定性模拟 | [→](./logos-sim/AGENTS.md) |
| [`poetry-garden`](./poetry-garden) | Vite + TS Canvas | 古诗词意境学习应用 | [→](./poetry-garden/AGENTS.md) |
| [`sky-carrier`](./sky-carrier) | Vite + Babylon.js | 硬科幻工程物理与任务模拟 | [→](./sky-carrier/AGENTS.md) |
| [`superpower-system`](./superpower-system) | Vite + TS strict | 个人能力 RPG 化解锁面板（含独立 ReAct agent 教练） | [→](./superpower-system/AGENTS.md) |
| [`wuwan`](./wuwan) | 原生 HTML/CSS/JS + Python 生成器 | "金无足赤"中文谚语长卷（2000 静态页） | [→](./wuwan/AGENTS.md) |
| [`voxel-craft`](./voxel-craft) | Godot 4.7 GDScript | 手机 3D 我的世界风格体素生存游戏 | [→](./voxel-craft/AGENTS.md) |
| [`mobile-game-workflow`](./mobile-game-workflow) | Bash/GDScript/Kotlin/CI | Godot 手游构建流水线工具集 | [→](./mobile-game-workflow/AGENTS.md) |

### H. 科学模拟与交互教育
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`physics-sim`](./physics-sim) | Vite + Babylon.js + Havok | 通用物理模拟器平台 | [→](./physics-sim/AGENTS.md) |
| [`fusion-power-3d`](./fusion-power-3d) | Vite + Three.js | 三路线核聚变电站模拟 | [→](./fusion-power-3d/AGENTS.md) |
| [`earth-history-3d`](./earth-history-3d) | Next.js + React | 地球地质年代史（名含3d但实际是时间线页面，无3D） | [→](./earth-history-3d/AGENTS.md) |

### I. 研究与调研（新增）
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`web-game-research`](./web-game-research) | MD + CSV + bash/node 脚本 | Web 游戏引擎与开源游戏研究（调研项目，含样本 clone） | [→](./web-game-research/AGENTS.md) |
| [`web-standards-research`](./web-standards-research) | MD + HTML/JS + Python + Wasm | 现代化 Web 标准横评（**30 标准**：3 批 × 各 5 主题）含 30 个详细可运行 demo（含 4 个🆕进阶版：WebGPU 粒子/Next.js/多人会议/Sobel）+ **30 份深度剖析档案**（每标准一份）+ **Web 平台演进深度报告**（598 行） | [→](./web-standards-research/AGENTS.md) |
| [`chinese-philosophy-research`](./chinese-philosophy-research) | TS + Hono + better-sqlite3 | 中国儒释道发展研究 agent（内置经典语料 + ReAct 研究 + 多视角报告） | [→](./chinese-philosophy-research/AGENTS.md) |
| [`language-research`](./language-research) | MD 研究资产 | AI 原生语言能力边界研究（Rust/Go/Python/TS 分层共生） | 待补 |
| [`on-device-llm`](./on-device-llm) | MD 研究资产 | 端侧大模型本地化部署研究（推理引擎实测/量化压缩/端侧选型/全栈综述） | 待补 |

### J. 新增项目（未在原始导航注册）
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`city-hunt`](./city-hunt) | Godot 4.7 GDScript | 3D 开放世界城市动作射击（GTA 风简化·体素方块风·手机端） | [→](./city-hunt/AGENTS.md) |
| [`space-shooter`](./space-shooter) | Rust + Bevy 0.15 ECS | 3D 太空射击（Star Fox 风） | [→](./space-shooter/AGENTS.md) |
| [`nexus_app`](./nexus_app) | Flutter | 可视化无限进度超级智能 App（多 Agent·思考+进度可视化） | [→](./nexus_app/AGENTS.md) |
| [`mesh-bridge`](./mesh-bridge) | Node + TS | 低成本 3D 建模聚合平台（云 API + CC0 库 + 本地桥接 + 多 provider 对比） | 待补 |
| [`fog-explorer`](./fog-explorer) | Rust + Bevy ECS | Bevy 探索实验（验证引擎特性与 ECS 模式） | 待补 |
| [`kids-park`](./kids-park) | Godot 4.7 GDScript | 儿童 3D 乐园探索（动森简化版·4 区域·45+ 玩法·55 脚本） | [→](./kids-park/AGENTS.md) |

### K. 基础设施与工程工具
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`consensus-atlas`](./consensus-atlas) | Go 1.25 零依赖 | 分布式系统教学库（12 算法：Raft/Multi-Paxos/PBFT/Gossip/Bully+Ring/Vector Clock/2PC/CRDT/拜占庭将军OM/Chandy-Lamport快照/Viewstamped/ZAB，+跨算法bench） | [→](./consensus-atlas/AGENTS.md) |
| [`workspace-ops`](./workspace-ops) | Go 1.25 + Vue 3 + SQLite | 工作区级管理工具（scan/report/serve/test 四子命令，扫描 60+ 项目，含实跑测试采集 + REST API + Web 看板） | [→](./workspace-ops/AGENTS.md) |
| [`flow-pipe`](./flow-pipe) | Go 1.25 + SQLite | 轻量数据管道/ETL（11 连接器+DAG编排+retry重试+dead_letter死信+状态恢复+-schedule定时，REST API） | [→](./flow-pipe/AGENTS.md) |
| [`lang-impl`](./lang-impl) | Go 1.25 零依赖 | 玩具编程语言 "M" 编译器（lex→parse→interpret + REPL + WASM后端，node验证 add(3,4)=7） | [→](./lang-impl/AGENTS.md) |
| [`crypto-atlas`](./crypto-atlas) | Go 1.25 零依赖 | 密码学教学库（10 算法：凯撒/维吉尼亚/XOR/AES/DES/SHA-256/MD5/RSA/DH/HMAC，+core测试） | [→](./crypto-atlas/AGENTS.md) |
| [`regex-engine`](./regex-engine) | Go 1.25 零依赖 | 正则表达式引擎（Thompson NFA 抗ReDoS，Match/FindAll/ReplaceAll/分组捕获 FindAllWithGroups） | [→](./regex-engine/AGENTS.md) |
| [`ai-safety-atlas`](./ai-safety-atlas) | Go 1.25 零依赖 | AI 安全测试库（提示注入/越狱检测+多轮上下文分析+31红队用例+批量报告，P100%/R78.6%/F1=0.88） | [→](./ai-safety-atlas/AGENTS.md) |
| [`obs-lite`](./obs-lite) | Go 1.25 零依赖 | 可观测性平台（metrics counter/gauge/histogram + trace span树 + HTTP /metrics Prometheus端点） | [→](./obs-lite/AGENTS.md) |

## 关键关联（详见 WORKSPACE_OVERVIEW.md）
- **cogent → 依赖 → agentloop**：编译蓝图为 agentloop 的 runLoop() 调用（唯一强代码依赖）
- **agentresearch 与 agentloop 同源**（最小 ReAct 循环的两种形态）
- **platform-demo** 是 cogent+agentloop+e2e-fusion 的集成契约层
- **wildera / future-world-3055** 同用 Babylon.js（可互鉴引擎）
- **Godot 手游线**：app-game-research（调研）→ mobile-game-workflow（构建工具）→ voxel-craft / pastoral-travel / city-hunt / kids-park（真游戏）；app-game-research 依赖 godot-src（Godot 引擎源码副本，非工作区项目）做源码剖析
- **logos 三件套**：logos-formal（Lean 形式化证明）/ logos-sim（确定性模拟）/ dashan（哲学对话）覆盖哲学的证明—模拟—对话三态
- **哲学三态扩展**：logos-formal/sim + dashan（哲学对话）↔ chinese-philosophy-research（三教历史/学理研究 + 经典检索 agent）
- **AI 介入孪生篇**：`ai-expansion-analysis`（数据仪表盘——现在渗透多深）↔ `ai-world-research`（叙事推演——之后世界会怎样），同主题不同切面，可交叉引用
- **web-game-research** 是纯研究项目，为 7 个游戏项目提供外部引擎选型参照
- **K 组基础设施线**：`consensus-atlas` 与 `go-agent-research` 同范式（纯标准库教学库，`internal/<单元>/` 5 件套 + NOTES.md + demo 离线可跑）；`workspace-ops` 服务于整个工作区（扫描全部项目，自用工具）；`flow-pipe` 的连接器插件化对齐 `generic-admin/export` 的 Interface+Registry、config/storage 三件套复用 `workspace-ops` 同源实现
- 其余项目相互独立

## 非项目目录（不算工作区作品）
- `godot-src/`：Godot 4.7 引擎官方源码副本（git remote 指向 godotengine/godot），供 app-game-research 剖析用，非工作区项目
- `bevy-src/`：Bevy 引擎源码副本，供 space-shooter / fog-explorer 参考，非工作区项目
- `relay/`：go-rmm 的中继服务器子模块（在 go-rmm/AGENTS.md 内记录），非独立项目
- `proto/`：go-rmm 的协议层子模块（在 go-rmm/AGENTS.md 内记录），非独立项目
- `export/`、`scripts/`：工作区级工具脚本目录，非独立项目

## 工作区约定
- 各项目独立运行，**不要跨项目改代码**除非明确要求。
- 根目录有产品化管理元层：`*.md` 规划文档 + `scripts/` 校验脚本 + `.portfolio/` 状态快照。
- 根目录有研究资产文档：[`GAME_ENGINE_RESEARCH.md`](./GAME_ENGINE_RESEARCH.md)（游戏引擎选型）、[`3D_MODELING_RESEARCH.md`](./3D_MODELING_RESEARCH.md)（低成本 3D 建模方案），均基于真实代码核实 + 行业对标，进入 3D/游戏子项目前可先读。
- 根目录无 workspace/monorepo 配置；现有根 `package.json` 仅含独立 Havok 依赖，不作为统一依赖管理入口。
- 多数 TS 项目：Node ≥ 20.19，ESM，TS 严格，`node --test`，ESLint + Prettier。
