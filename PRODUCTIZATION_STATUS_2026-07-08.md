# 产品化组合状态：新增项目深化版

更新日期：2026-07-09

## 总体判断

当前应把项目组合从“前景排序”升级为“产品化门禁 + 路线推进”。最值得持续投入的新增项目已经形成 4 条线：

1. **AI 工程平台线**：`e2e-fusion`、`agentloop`、`cogent`
2. **垂直研究 Agent 线**：`history-mist`、`agentresearch`
3. **可规模化内容平台线**：`algorithms-atlas`、`ai-expansion-analysis`、`tiny-edge-models`
4. **3D 长线原型线**：`wildera`、`future-world-3055`

新的资源策略是：P0 项目要形成可演示、可验证、可复用的产品底座；P1 项目要形成内容增长和报告输出能力；P2 项目只保留垂直切片，不继续横向扩张系统。

## 当前优先级

| 优先级 | 项目 | 当前定位 | 产品化状态 | 下一步 |
| --- | --- | --- | --- | --- |
| P0 | `e2e-fusion` | AI 驱动跨端 E2E 验证平台 | 已补 `PRODUCT.md`、产品门禁、平台验证脚本修复 | 打磨 Web + API 真实示例，统一 Node/pnpm 环境验证 |
| P0 | `agentloop` | Agent 执行内核 | 已补 `PRODUCT.md`、产品门禁，测试/类型/lint 通过 | 把 trace replay、eval、HITL、memory 做成稳定核心能力 |
| P0 | `cogent` | 声明式 Agent 编排 | 已补 `PRODUCT.md`、产品门禁，测试/类型/lint 通过 | 强化 schema 版本、条件路由、模板复用 |
| P0 | `algorithms-atlas` | 技术教育内容平台 | 已补学习路径、产品文档、产品门禁，测试/类型/lint/build 通过 | 继续扩展图、DP、字符串和面试学习路线 |
| P0 | `history-mist` | 历史研究 Agent | 已补离线产品 demo、报告样例、产品文档、产品门禁 | 来源分级、争议观点、知识图谱复用形成固定研究流程 |
| P1 | `agentresearch` | 研究工作台 + 轻量 ReAct Agent | 已补 `PRODUCT.md`、产品门禁、离线研究 demo、结构化摘要/引用、状态快照，测试/类型/lint 通过 | trace replay 合约、真实研究报告模板 |
| P1 | `wildera` | 3D 生存建造长线 | 已补产品门禁和状态快照，类型/build 通过 | 聚焦“探索-采集-建造-遭遇-存档恢复”可玩闭环 |
| P1 | `ai-expansion-analysis` | AI 趋势雷达 | 已补产品门禁、报告导出、edge AI 专题 | 做成趋势研究门户，补 HTML/历史趋势页面 |
| P1 | `tiny-edge-models` | 端侧 AI 知识库 | 已补产品门禁并接入趋势雷达 | 与 `ai-expansion-analysis` 合并成 edge AI 垂直栏目 |
| P2 | `future-world-3055` | 长期旗舰 3D 原型 | 已补城市垂直切片任务和产品门禁 | 只做城市探索切片，暂缓太空/派系/文明扩张 |

## 路线优化

### 1. 平台底座先合流

`e2e-fusion`、`agentloop`、`cogent` 不应分散发展。更合理的产品叙事是：

- `agentloop` 负责可靠执行：step、trace、eval、HITL、memory。
- `cogent` 负责声明式蓝图：agent、tool、workflow、schema、policy。
- `e2e-fusion` 负责把这套 Agent 能力落到跨端 E2E 验证场景。

短期目标：做一个统一 demo：用 `cogent` 声明测试 Agent，用 `agentloop` 执行与回放，用 `e2e-fusion` 驱动 Web/API 场景并输出报告。

### 2. 内容平台分两类推进

`algorithms-atlas` 是教育型内容平台，增长单位是“算法卡片 + 动画 trace + 测试”。应继续规模化。

`ai-expansion-analysis` 和 `tiny-edge-models` 是研究型内容平台，增长单位是“趋势主题 + 数据来源 + 报告导出”。两者应合并为同一知识库，而不是并行维护两个门户。

### 3. 3D 项目只保留一个主推

`wildera` 当前闭环最好，应作为唯一主推 3D 长线。`future-world-3055` 保留为旗舰设定和技术原型，但近期只允许推进城市切片，避免系统继续膨胀。

### 4. history-mist 做成垂直 Agent 标杆

`history-mist` 的产品感来自“知识图谱 + 考据 + 争议呈现 + 报告导出”，不是聊天。后续所有功能都应服务于研究闭环：

- 问题识别
- 资料检索与来源分级
- 观点冲突呈现
- 知识图谱沉淀
- 可导出的研究报告

### 5. agentresearch 作为轻量研究 Agent 沙箱

`agentresearch` 不应与 `history-mist` 重叠成另一个历史研究产品。它更适合做通用研究工作台和 Agent 工程教学样板：

- ReAct 主循环和工具调用保持短小、可解释。
- `daily/` 与 `notes/` 作为研究记忆和内容沉淀层。
- StubLLM 保证无 API key 也能演示 Agent 工具闭环。
- 已补 `offline-research-workflow` 标准研究任务 fixture，并导出 `research-summary.json/md`；后续应强化 trace replay 合约和真实研究报告模板，再与 `agentloop` 的可靠执行模型对齐。

## 30 天执行顺序

### 第 1 周：统一门禁

- 用 `algorithms-atlas` 产品门禁持续检查算法数量、分类覆盖、学习路径和测试面。
- 用 `history-mist` 产品门禁持续检查 demo fixture、报告导出、知识图谱落盘。
- 用 `agentresearch` 产品门禁持续检查 ReAct 核心、工具面、笔记流、测试和 CI。
- 用根级 `scripts/portfolio-product-check.mjs --write` 汇总组合状态。

### 第 2 周：平台统一 demo

- `cogent` 输出一个 E2E 测试 Agent blueprint。
- `agentloop` 执行该 blueprint 并生成 trace/eval。
- `e2e-fusion` 接入该执行结果，形成统一报告。

### 第 3 周：内容增长

- `algorithms-atlas` 补齐 3 条学习路径：图论、动态规划、字符串。
- `ai-expansion-analysis` 输出 HTML 趋势报告。
- `tiny-edge-models` 增加模型选择决策树并作为 edge AI 专题入口。

### 第 4 周：3D 垂直切片

- `wildera` 固化一个 10 分钟可玩任务链。
- `future-world-3055` 只补城市任务、扫描交互、存档恢复，不扩新系统。

## 新增组合门禁

根目录新增：

```bash
node scripts/portfolio-product-check.mjs
node scripts/portfolio-product-check.mjs --write
node scripts/platform-product-check.mjs
node scripts/platform-product-check.mjs --write
```

门禁关注 3 类事实：

- 是否有 `PRODUCT.md`
- 是否有产品门禁或可重复 demo 入口
- 是否有最近一次状态快照

这不是替代子项目测试，而是用于判断组合层面哪些项目已经进入可持续推进状态。

`platform-demo/agent-e2e-platform-contract.json` 进一步固定 P0 平台合流路径：`cogent` 负责声明，`agentloop` 负责执行，`e2e-fusion` 负责跨端验证证据和报告。下一步要把该契约落成 `web-api-agent-validation` 单命令 demo。

2026-07-09 更新：组合门禁已纳入 `agentresearch`，当前为 10/10 product docs、10/10 executable gates、10/10 healthy snapshots。它作为 P1 研究 Agent 工作台推进，不替代 `history-mist` 的垂直历史研究定位。`agentresearch` 已新增 `npm run product:demo`，可离线生成 `data/product-demo/research-workflow.json`、`research-summary.json` 和 `research-summary.md`，证明笔记检索、计算工具、结构化结论、引用和最终答案闭环。

## 结论

新增项目里，`algorithms-atlas` 和 `history-mist` 是最值得继续放大的两个产品；`wildera` 是唯一应主推的 3D 长线；`future-world-3055` 应作为旗舰原型收敛切片；`ai-expansion-analysis + tiny-edge-models` 应合并成技术趋势知识库。平台层则继续以 `e2e-fusion + agentloop + cogent` 作为主轴推进。
