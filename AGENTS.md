# M_X_M 工作区 · AGENTS.md

这是多项目工作区，**不是 monorepo**：32 个项目相互独立，无共享代码、无 workspace 配置、无统一依赖管理。每个项目有自己的技术栈与运行方式。

> **完整梳理（含关联关系图、优先级、状态）见 [`WORKSPACE_OVERVIEW.md`](./WORKSPACE_OVERVIEW.md)。**
> 进入任何子项目前，先读它的 `AGENTS.md`（部分已有，其余待补）。

## 项目导航（38 个，按产品线分组）

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
| [`go-agent-research`](./go-agent-research) | Go 1.25 零依赖 | Agent 范式研究（29范式 + 4大深化基础设施：trace可观测/mockgen智能脚本/bench跨范式基准/llmadapt真实LLM适配 + 选型指南，含4个完整版重范式，纯标准库+Mock/Ollama） | [→](./go-agent-research/AGENTS.md) |
| [`rust-agent-research`](./rust-agent-research) | Rust 1.97 + tokio | go-agent-research 的 Rust 全量移植（26范式+底座，async/await+Arc+Cargo workspace，对齐Go版设计） | [→](./rust-agent-research/AGENTS.md) |
| [`gpu-mesh`](./gpu-mesh) | Go 1.22+ | 异地分布式 GPU 算力调度平台（反向WS穿透NAT+Win服务化+GPU监控仪表盘+让位调度+Ollama/llama.cpp双引擎+OpenAI网关+GPU感知调度+批量Map-Reduce+LoRA训练+mTLS/审计/多租户，全6 Phase完成） | [→](./gpu-mesh/AGENTS.md) |
| [`realtime-digital-human`](./realtime-digital-human) | Python 3.10 + FastAPI + asyncio | 单机实时数字人（ASR→LLM→TTS→唇形 流式管线重叠，首响应<1.5s，4060 8GB 可跑） | [→](./realtime-digital-human/AGENTS.md) |

### B. AI 应用 / 研究 Agent
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`history-mist`](./history-mist) | TS + Hono + sqlite | 历史探索智能体（知识图谱+报告） | [→](./history-mist/AGENTS.md) |
| [`dashan`](./dashan) | TS Vite | "大善系统"哲学对话 | [→](./dashan/AGENTS.md) |
| [`computer-use-rtc`](./computer-use-rtc) | Node + PowerShell + GLM 视觉 | RTC 视频自动化（截图→识别→点击） | [→](./computer-use-rtc/AGENTS.md) |

### C. 技术教育与内容平台
| 项目 | 技术栈 | 一句话 | AGENTS.md |
|------|--------|--------|-----------|
| [`algorithms-atlas`](./algorithms-atlas) | Vite + TS strict | 3000 算法图谱（动画演示） | [→](./algorithms-atlas/AGENTS.md) |
| [`kids-games`](./kids-games) | Vite + TS PWA | 3~6 岁 81 个儿童小游戏 | [→](./kids-games/AGENTS.md) |
| [`lightai`](./lightai) | Python 零依赖 | 轻量 AI 算法库（替代大模型） | [→](./lightai/AGENTS.md) |
| [`ai-expansion-analysis`](./ai-expansion-analysis) | 纯 HTML/JS | AI 扩张可视化分析站 | [→](./ai-expansion-analysis/AGENTS.md) |

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
| [`agenttrain`](./agenttrain) | Vite + TS Canvas | Mini Metro 风调度游戏(+AI顾问) | [→](./agenttrain/AGENTS.md) |

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

## 关键关联（详见 WORKSPACE_OVERVIEW.md）
- **cogent → 依赖 → agentloop**：编译蓝图为 agentloop 的 runLoop() 调用（唯一强代码依赖）
- **agentresearch 与 agentloop 同源**（最小 ReAct 循环的两种形态）
- **platform-demo** 是 cogent+agentloop+e2e-fusion 的集成契约层
- **wildera / future-world-3055** 同用 Babylon.js（可互鉴引擎）
- **Godot 手游线**：app-game-research（调研）→ mobile-game-workflow（构建工具）→ voxel-craft / pastoral-travel（真游戏）；app-game-research 依赖 godot-src（Godot 引擎源码副本，非工作区项目）做源码剖析
- **logos 三件套**：logos-formal（Lean 形式化证明）/ logos-sim（确定性模拟）/ dashan（哲学对话）覆盖哲学的证明—模拟—对话三态
- **哲学三态扩展**：logos-formal/sim + dashan（哲学对话）↔ chinese-philosophy-research（三教历史/学理研究 + 经典检索 agent）
- **AI 介入孪生篇**：`ai-expansion-analysis`（数据仪表盘——现在渗透多深）↔ `ai-world-research`（叙事推演——之后世界会怎样），同主题不同切面，可交叉引用
- **web-game-research** 是纯研究项目，为 7 个游戏项目提供外部引擎选型参照
- 其余项目相互独立

## 非项目目录（不算工作区作品）
- `godot-src/`：Godot 4.7 引擎官方源码副本（git remote 指向 godotengine/godot），供 app-game-research 剖析用，非工作区项目

## 工作区约定
- 各项目独立运行，**不要跨项目改代码**除非明确要求。
- 根目录有产品化管理元层：`*.md` 规划文档 + `scripts/` 校验脚本 + `.portfolio/` 状态快照。
- 根目录有研究资产文档：[`GAME_ENGINE_RESEARCH.md`](./GAME_ENGINE_RESEARCH.md)（游戏引擎选型）、[`3D_MODELING_RESEARCH.md`](./3D_MODELING_RESEARCH.md)（低成本 3D 建模方案），均基于真实代码核实 + 行业对标，进入 3D/游戏子项目前可先读。
- 根目录无 workspace/monorepo 配置；现有根 `package.json` 仅含独立 Havok 依赖，不作为统一依赖管理入口。
- 多数 TS 项目：Node ≥ 20.19，ESM，TS 严格，`node --test`，ESLint + Prettier。
