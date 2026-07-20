# e2e-fusion + agentloop/cogent 细化梳理与未来规划

更新日期：2026-07-07

## 0. 一句话战略

把 `e2e-fusion` 做成“跨端软件验证平台”，把 `agentloop` 做成“可靠 Agent 执行内核”，把 `cogent` 做成“声明式 Agent/Workflow 配置层”。三者组合后形成一条完整链路：

```text
测试需求 / 产品文档 / 历史失败
        ↓
cogent 声明式测试 Agent / 工作流
        ↓
agentloop 可靠执行：工具调用、轨迹、审批、预算、评估、恢复
        ↓
e2e-fusion drivers/orchestrator/runner/platform
        ↓
跨 Web / Electron / Windows / API / CLI 的执行、报告、回放、自愈
```

核心原则：`e2e-fusion` 不应长期维护一套独立简化 Agent 循环；应逐步迁移到 `agentloop` + `cogent`，把自身精力放在测试领域能力、驱动、报告、平台和企业工作流上。

## 1. 当前资产盘点

### 1.1 `e2e-fusion`

定位：跨端联合 E2E 验证平台。

现有结构：

- `apps/platform`：Next.js 管理台，已有 cases、runs、agent、stats 等页面和 API。
- `apps/runner`：CLI 入口。
- `apps/worker`：队列/worker 入口雏形。
- `packages/core`：核心契约，定义 Target、Driver、Action、TestCase、Tool、RunnerEvent 等。
- `packages/case-parser`：YAML DSL 解析、变量插值。
- `packages/drivers`：web、electron、native-win、desktop-use、http、process、data 等驱动。
- `packages/orchestrator`：三种模式编排：`dual-consistency`、`data-flow`、`multi-entry`。
- `packages/runner-core`：解析、执行、报告输出。
- `packages/reporter`：JSON/HTML/JUnit 报告。
- `packages/storage`：Prisma + SQLite 数据层。
- `packages/scheduler`：memory/redis queue、cron。
- `packages/auth`：RBAC、API key、审计。
- `packages/knowledge`：文档摄取、切片、向量检索。
- `packages/ai-provider`：OpenAI 兼容 LLM/embedding 抽象。
- `packages/agent-core`：当前自有 Agent 循环，支持生成、探索、自愈、分析。

优势：

- 工程平台骨架完整，不是单 demo。
- DSL、driver、orchestrator、reporter、storage、platform 分层清晰。
- 自测体系较强，文档列出约 171 个测试，覆盖 12 个包。
- 已有 `desktop-use` 能力，能承接 `computer-use-rtc` 的视觉桌面自动化经验。

短板：

- `agent-core` 的 Agent 循环相对 `agentloop` 简化，长期会产生重复维护。
- AI 能力目前更多是“生成 YAML”，尚未深入执行闭环：探索、定位器自愈、失败归因、回放评估。
- 平台产品形态仍偏工程内部工具，需要明确首个垂直落地场景。
- examples 很多，但缺少一个强演示故事：从文档到用例、执行、失败、自愈、报告的完整闭环。

### 1.2 `agentloop`

定位：可靠 Agent 执行内核。

现有能力：

- 主循环 `runLoop()`：LLM ↔ tools 的单线程循环。
- 上下文工程：自动压缩、token 预算。
- 流式输出：SSE delta 聚合。
- 可观测：span tree、trace、usage、成本估算。
- OTel 导出：零依赖 OTLP/HTTP，`gen_ai.*` 属性。
- 并行 sub-agent：`delegate` / `delegate_parallel`，支持 fan-out/fan-in。
- 持久化会话：文件 store，原子写。
- HITL 审批：高风险工具执行前确认。
- 轨迹持久化与回放：trace-store、trajectory。
- LLM-as-judge 评估：工具选择、参数质量、效率、恢复、安全等维度。
- MCP 相关模块：adapter/client/protocol/registry。
- 内置工具：calculator、datetime、iterate、http_get、web_search 等。

优势：

- 能力更接近“可运营 Agent runtime”。
- 零运行时依赖，容易嵌入。
- 测试数量多，覆盖预算、压缩、并发、MCP、轨迹、评估等核心模块。

短板：

- 目前作为独立项目，和 `e2e-fusion` 的包管理、类型系统、事件协议尚未统一。
- 工具协议和 `e2e-fusion/packages/core/src/tool.ts` 的 Tool 协议相似但不完全一致。
- 缺少“领域工具集”最佳实践，例如测试工具、浏览器工具、桌面工具的标准封装。

### 1.3 `cogent`

定位：声明式 Agent 框架。

现有能力：

- `Agent` 声明：system、builtin tools、custom tools、constraints、verification。
- `Workflow` 声明：单 agent、parallel、dependsOn、when、output、`{{ }}` 运行时插值。
- 模板变量：`${var}`。
- 继承：`extends` 深合并、环检测。
- 自定义工具：内联 JS 沙箱执行。
- 验证：客观断言 + 可选 LLM judge。
- 示例：researcher、analyst、coder、searcher、math-specialist、map-reduce、report-pipeline 等。

优势：

- 把 Agent 定义从代码变成 JSON/YAML，可配置、可复用、可审计。
- 非常适合 `e2e-fusion` 的测试 Agent：生成、探索、自愈、分析都可以声明化。

短板：

- 当前通过相对路径引用 `../agentloop/src/...`，工程边界还不成熟。
- workflow 语义适合轻编排，但还没有面向测试平台的领域节点，如 `runCase`、`ingestDocs`、`healSelector`。
- Agent spec 还没有版本迁移、schema 发布、UI 表单编辑能力。

## 2. 战略定位拆分

### 2.1 `e2e-fusion` 不做什么

- 不做通用 Agent 框架。
- 不重复实现长期 Agent runtime。
- 不一开始承诺“完全自动生成所有测试并自愈所有失败”。
- 不把 AI 作为绕过工程确定性的替代品。

### 2.2 `e2e-fusion` 应该专注什么

- 跨端目标管理：Web、Electron、Windows、API、CLI、desktop-use。
- 测试 DSL 和运行时：用例、环境、变量、密钥、target、step、assertion、teardown。
- Driver 能力：启动、连接、执行、快照、产物、清理。
- Orchestrator：多端一致性、跨端数据流、多入口契约。
- 结果可信：事件流、报告、截图、日志、trace、JUnit、历史趋势。
- 平台能力：用例管理、执行调度、权限、审计、CI、通知。

### 2.3 `agentloop` 应该专注什么

- Agent 执行可靠性。
- 工具调用规范。
- 上下文、预算、审批、恢复、轨迹、评估。
- 运行时可观测性和可回放性。
- 与 `e2e-fusion` driver 工具集对接。

### 2.4 `cogent` 应该专注什么

- Agent/Workflow 声明格式。
- 继承、模板、验证、编排。
- 将测试领域的 Agent 角色配置化：
  - `test-generator`
  - `feature-explorer`
  - `selector-healer`
  - `failure-analyzer`
  - `case-reviewer`
  - `release-risk-analyst`

## 3. 推荐目标产品形态

### 3.1 产品名级叙事

E2E Fusion：AI-assisted Cross-Endpoint Validation Platform。

中文叙事：AI 辅助跨端联合验证平台。

它解决的问题不是“写 Playwright 脚本”，而是：

- 一个功能在 Web、桌面客户端、API、后台数据之间是否一致？
- 本地程序、进程、日志、文件、数据库和 UI 是否构成完整业务闭环？
- 失败后能否快速知道是环境、数据、选择器、时序、业务回归还是被测程序崩溃？
- AI 能否把文档、截图、历史失败转成测试资产，而不是只生成一次性脚本？

### 3.2 目标用户

优先级最高：

- 桌面客户端 + Web 管理端 + API 后端并存的业务团队。
- 有 Windows 客户端、Electron、内网系统、一体机、金融/政企终端的团队。
- 测试团队需要把 API、UI、日志、文件、数据库一起验证。

暂不优先：

- 只测普通 Web 页面的团队。纯 Web Playwright 已有成熟生态，差异化弱。
- 追求低代码炫酷 UI 但不关心执行可信度的场景。

## 4. 三者集成设计

### 4.1 统一 Agent 执行层

现状：

- `e2e-fusion/packages/agent-core/src/agent.ts` 有自己的 Agent 类。
- `agentloop/src/loop.ts` 已经提供更完整 runLoop。

建议：

```text
短期：保留 agent-core API 外观，内部可选调用 agentloop adapter
中期：agent-core 变为 e2e 领域封装，移除自有循环
长期：agent-core = e2e tools + prompts + cogent manifests + platform adapters
```

迁移接口建议：

```text
@e2e-fusion/agent-core
├─ createE2eTools(drivers, ctx): agentloop ToolDef[]
├─ runE2eAgent(manifestOrRole, goal, ctx): AgentRunResult
├─ generateCase(intent, docs): YAML
├─ exploreFeature(target): FeatureMap
├─ healFailure(runId, step): HealProposal
└─ analyzeFailure(runId): FailureAnalysis
```

### 4.2 统一工具协议

`e2e-fusion` 当前 Tool 协议：

- `name`
- `description`
- `inputSchema`
- `run(ctx, input) -> ToolResult`

`agentloop` 工具协议需要和它建立 adapter。建议新增：

```text
packages/agent-core/src/tool-adapter.ts
```

职责：

- 将 `@e2e-fusion/core` Tool 转换为 `agentloop` ToolDef。
- 将 tool result 中的 artifact 转回 runner event。
- 统一错误、审批、日志、trace attributes。

命名规范：

```text
web.goto
web.click
web.snapshot
api.request
client.click
desktop.look
desktop.click
runner.readFile
data.query
```

### 4.3 用 `cogent` 描述测试 Agent

新增目录建议：

```text
e2e-fusion/
  agents/
    test-generator.yaml
    feature-explorer.yaml
    selector-healer.yaml
    failure-analyzer.yaml
    run-reviewer.yaml
  workflows/
    doc-to-tests.yaml
    explore-to-cases.yaml
    failure-to-heal.yaml
    nightly-risk-analysis.yaml
```

示例工作流：

```yaml
apiVersion: cogent/v1
kind: Workflow
metadata:
  name: failure-to-heal
spec:
  steps:
    - name: collect
      agent: run-reviewer
      input: "读取 run {{ runId }} 的失败步骤、截图、DOM、日志"
      output: failureContext
    - name: analyze
      agent: failure-analyzer
      input: "{{ failureContext }}"
      output: analysis
    - name: heal
      agent: selector-healer
      input: "{{ analysis }}"
      when: "analyze.success == true"
      output: patch
```

### 4.4 平台 UI 集成

平台不应该只提供“输入意图 -> 生成 YAML”。建议逐步变成 Agent Workbench：

- 文档区：上传 Markdown/OpenAPI/URL/PDF，入知识库。
- 探索区：选择 target，运行 feature explorer，展示页面/功能地图。
- 生成区：从功能地图选择要覆盖的路径，生成 YAML。
- 执行区：运行 YAML，实时 SSE 展示。
- 分析区：失败归因、自愈建议、人工确认应用 patch。
- 审计区：显示 agentloop trace、工具调用、token、审批记录。

## 5. 未来路线图

### 阶段 0：收敛边界，避免重复建设

时间：1 周。

目标：

- 明确 `agentloop`/`cogent` 是 Agent 主线，`e2e-fusion/agent-core` 是领域封装。
- 建立集成 ADR 文档。
- 梳理当前 `agent-core` 哪些能力迁移到 agentloop，哪些保留。

交付物：

- `docs/adr/0001-agent-runtime-selection.md`
- `packages/agent-core/src/tool-adapter.ts` 设计草案
- `agents/` 和 `workflows/` 目录规划

验收：

- 不再新增新的 e2e 自有 Agent 循环能力。
- 新 AI 功能默认评估能否用 agentloop/cogent 实现。

### 阶段 1：执行内核稳定化

时间：2-3 周。

目标：

- 保证 `e2e-fusion` 的非 AI 主链路稳定：parse -> orchestrate -> driver -> report -> platform run。
- 做 3 个黄金 demo。

黄金 demo：

1. API + CLI：启动 todo 后端，注册/登录/创建数据，验证文件持久化。
2. Web + API：Web 操作产生数据，API 验证契约。
3. Desktop-use：记事本或业务 Windows 程序，视觉点击 + 输入 + 视觉断言。

交付物：

- `examples/golden/`
- 平台首页一键运行 demo
- 报告页展示 step、artifact、日志、断言、失败原因

验收：

- `pnpm build`、`pnpm test` 可靠。
- demo 可在干净环境按 README 跑通。
- 报告能让用户不看终端也理解失败。

### 阶段 2：agentloop 接入 e2e 工具

时间：2-4 周。

目标：

- 将 e2e driver 工具暴露给 agentloop。
- 用 agentloop 替代 `agent-core` 内部循环的至少一个能力。

优先迁移顺序：

1. `analyze(failureInfo)`：最少工具依赖，收益明显。
2. `heal(oldSelector, domSnapshot)`：可接入 trace 和人工审批。
3. `generate(intent)`：加入知识库上下文和 DSL 校验闭环。
4. `explore(entry, tools)`：最后迁移，因为它最依赖 driver 工具和安全护栏。

交付物：

- `@e2e-fusion/agent-core` 内部 `runWithAgentLoop()`
- Tool adapter
- Agent trace 存储到 TestRun / AgentSession / AgentStep
- 平台展示工具调用轨迹

验收：

- AI 失败分析结果包含 trace id。
- 工具调用出错不会让平台 run 卡死。
- 可以限制 max steps、token budget、审批策略。

### 阶段 3：cogent 声明式测试 Agent

时间：3-5 周。

目标：

- 将测试 Agent 角色声明化。
- 将“文档 -> 功能地图 -> 用例 -> 执行 -> 分析”变成 workflow。

推荐 Agent：

- `test-generator`：输入需求/功能点，输出 DSL YAML。
- `dsl-reviewer`：检查 YAML 是否符合平台 DSL、是否有危险动作。
- `feature-explorer`：使用 driver tools 探索应用，输出 feature map。
- `selector-healer`：输入失败选择器、DOM、截图描述，输出候选修复。
- `failure-analyzer`：输入 run 产物，输出根因分类。
- `case-minimizer`：把失败用例缩减成最小复现。

推荐 Workflow：

- `doc-to-cases`
- `explore-to-cases`
- `failure-to-heal`
- `nightly-risk-analysis`
- `release-smoke-suite`

验收：

- Agent YAML 可在平台 UI 查看、编辑、版本化。
- 每个 workflow 有可重复执行的示例。
- LLM 输出必须经过 DSL parser 校验，不合格自动回填错误修正，最多重试 N 次。

### 阶段 4：生产化

时间：1-2 个月。

目标：

- 从本地 demo 变成团队可用平台。

能力清单：

- 项目/环境/用例/执行历史管理。
- RBAC 与 API key。
- Schedule / cron / worker queue。
- CI 集成：GitHub Actions、GitLab CI、Jenkins。
- 通知：Webhook、企业微信/飞书/Slack。
- 产物归档：截图、视频、trace、日志、HTML/JUnit。
- Flaky 分析：失败率、重试、趋势。
- 审计：AI 做过什么、谁批准过什么、改了哪些用例。

验收：

- 一个团队能每天定时跑 smoke suite。
- 失败能自动生成可读报告并推送。
- AI 建议必须可审计、可回滚、可人工确认。

### 阶段 5：企业化和商业化

时间：3-6 个月。

目标：

- 多租户、分布式 runner、私有化部署、插件市场。

能力清单：

- Runner agent 分布式注册。
- Windows runner 节点池。
- K8s 部署。
- Postgres 替代 SQLite。
- KMS/Secret 管理。
- 租户隔离。
- 插件 SDK：driver、assertion、reporter、AI workflow。
- Marketplace：内置 SAP/CRM/桌面程序/浏览器兼容性模板。

## 6. 技术架构目标图

```text
┌────────────────────────────────────────────────────────────┐
│ Platform UI                                                  │
│ Cases · Runs · Reports · Agent Workbench · Admin · Audit     │
└───────────────┬──────────────────────────────┬─────────────┘
                │ REST/SSE                      │
┌───────────────▼──────────────┐       ┌────────▼────────────┐
│ Runner/Worker                 │       │ Agent Service        │
│ parse/orchestrate/report      │       │ cogent workflows     │
└───────────────┬──────────────┘       └────────┬────────────┘
                │                               │
┌───────────────▼──────────────┐       ┌────────▼────────────┐
│ e2e-fusion core               │       │ agentloop runtime    │
│ Target/Driver/Action/Tool     │◄─────►│ trace/budget/HITL    │
└───────────────┬──────────────┘       └────────┬────────────┘
                │                               │
┌───────────────▼────────────────────────────────▼────────────┐
│ Drivers as Tools                                             │
│ web · electron · native-win · desktop-use · api · cli · data │
└───────────────┬──────────────────────────────────────────────┘
                │
┌───────────────▼──────────────────────────────────────────────┐
│ Systems Under Test                                            │
│ Web app · Desktop app · API · DB · filesystem · process       │
└──────────────────────────────────────────────────────────────┘
```

## 7. 核心设计决策

### 7.1 AI 默认是建议者，不是自动提交者

AI 可以：

- 生成用例草稿。
- 生成选择器修复建议。
- 生成失败归因。
- 生成用例覆盖建议。

AI 不应默认：

- 自动修改生产用例。
- 自动点击高风险 destructive 操作。
- 自动隐藏失败。
- 自动判定业务正确。

所有自动应用都应满足：

- 有 trace。
- 有 diff。
- 有人工确认或明确策略。
- 可回滚。

### 7.2 DSL 是可信边界

LLM 输出必须进入 DSL parser，而不是直接执行。

流程：

```text
LLM output YAML
  ↓
case-parser parse + schema validate
  ↓
危险动作检查
  ↓
dry-run / plan 展示
  ↓
人工保存或执行
```

### 7.3 Driver 是平台护城河

普通 AI 测试工具容易停留在 Web 页面。`e2e-fusion` 的差异化在跨端 driver：

- Web：Playwright。
- Electron：Playwright `_electron`。
- Native Windows：WinAppDriver/UIA。
- Desktop-use：视觉模型 + 坐标操作。
- API：HTTP 契约。
- CLI/Process：启动、日志、退出码、文件。
- Data：SQLite/Postgres/MSSQL/MySQL/File。

未来投资应优先给 driver 的稳定性、产物采集和失败诊断，而不是只做 UI。

### 7.4 Trace 是 AI 可运营的基础

Agent 相关的每次运行必须记录：

- 输入目标。
- 使用的 manifest/workflow 版本。
- LLM 请求模型、token、耗时。
- 工具调用名称、参数、结果。
- 截图/DOM/日志/文件产物。
- 审批结果。
- 最终建议和是否被应用。

这既是调试，也是企业客户信任的基础。

## 8. 关键数据模型补充建议

`e2e-fusion` docs 中已经规划 MAI 数据表。建议优先落地：

```text
AgentManifest
- id
- projectId
- name
- kind: agent | workflow
- version
- contentYaml
- createdAt
- updatedAt

AgentRun
- id
- projectId
- testRunId?
- manifestId?
- goal
- status
- model
- tokenInput
- tokenOutput
- cost
- tracePath / traceJson
- startedAt
- endedAt

AgentStep
- id
- agentRunId
- seq
- toolName?
- toolInputJson?
- observation?
- artifactIds?
- ok
- durationMs

HealProposal
- id
- testRunId
- stepIndex
- oldSelector
- newSelector
- confidence
- status: proposed | applied | rejected
- diff
```

## 9. 项目协同方式

### 9.1 短期目录关系

保持独立目录，但建立 adapter：

```text
D:\M_X_M
├─ agentloop      执行内核
├─ cogent         声明层
└─ e2e-fusion     测试平台
```

短期可用相对路径或 workspace link，但要避免隐式复制代码。

### 9.2 中期 Monorepo 方案

如果这三者确认长期协同，建议建立新上层 workspace：

```text
ai-verification-suite/
├─ packages/
│  ├─ agentloop
│  ├─ cogent
│  ├─ e2e-fusion-core
│  ├─ e2e-fusion-drivers
│  └─ e2e-fusion-platform
└─ examples/
```

但不要过早搬迁。当前更重要的是先验证集成价值。

## 10. 风险与应对

| 风险 | 表现 | 应对 |
|---|---|---|
| AI 结果不稳定 | 生成 YAML 不可执行，自愈误判 | DSL 校验、dry-run、重试、人工确认、置信度 |
| 平台范围过大 | Web、桌面、AI、调度、权限同时推进 | 先做 3 个黄金 demo，锁定一个垂直场景 |
| Agent 循环重复 | e2e agent-core 与 agentloop 分叉 | agent-core 变领域封装，runtime 统一到 agentloop |
| Driver 不稳定 | 假绿、误报、环境卡死 | step timeout、readyWhen、产物采集、driver contract tests |
| Desktop-use 成本高 | 视觉模型慢、坐标不准 | 只用于 DOM/CDP 不可达场景；优先 UIA/Playwright |
| 企业落地难 | 安全、审计、密钥、权限缺失 | HITL、RBAC、审计、KMS、trace 持久化 |

## 11. 近期最应该做的 10 件事

1. 在 `e2e-fusion/docs` 写 ADR：Agent runtime 选择 `agentloop`。
2. 写 `@e2e-fusion/agent-core` Tool adapter 草案。
3. 把 `failure-analyzer` 迁移成 agentloop 执行，保留原 API。
4. 给 AI 生成 YAML 增加 parse/validate 自动回填修正。
5. 新增 `agents/test-generator.yaml`、`agents/failure-analyzer.yaml`。
6. 新增 `workflows/failure-to-heal.yaml`。
7. 平台 Run 详情页增加 Agent Trace 入口。
8. 整理 3 个黄金 demo，并在 README 顶部展示。
9. 把 `computer-use-rtc` 的经验沉淀进 `desktop-use` driver 文档。
10. 每个 AI 建议都输出 diff 和风险等级，默认人工确认。

## 12. 最终路线建议

第一主线：`e2e-fusion`

- 目标是平台产品。
- 衡量标准是团队能不能真实跑测试、看报告、定位问题。
- AI 是增强层，不能影响非 AI 主链路稳定。

第二主线：`agentloop`

- 目标是可靠 Agent runtime。
- 衡量标准是工具调用是否可控、轨迹是否可回放、预算/审批/恢复是否可靠。
- 它应该服务多个产品，`e2e-fusion` 是第一个强场景。

第三主线：`cogent`

- 目标是声明式 Agent 配置和工作流。
- 衡量标准是 Agent 是否能版本化、审计、复用、由平台 UI 管理。
- 它让 `e2e-fusion` 的 AI 能力从“代码里的 prompt”升级为“可管理的测试 Agent 资产”。

如果执行顺序只能选一个：先把 `e2e-fusion` 的失败分析和自愈建议迁到 `agentloop`，这是三者协同的最小闭环，风险低、价值清晰、容易演示。

