# auto-finance-assistant · AGENTS.md

## 项目内容（What）

Go 实现的**汽车金融本地智能客服系统**——单 EXE 服务 + SQLite + Ollama 本地模型 + Vue 前端，运行在一台 Windows 电脑上。9 个 Milestone 全部完成。

```
浏览器(Vue 聊天页) ──HTTP/SSE──► Go 单 EXE 服务
                                    ├─ 客服问答编排（脱敏→FAQ短路→RAG检索→grounded answer→落库）
                                    ├─ FAQ 匹配器（精确/关键词/编辑距离，104ms 短路）
                                    ├─ RAG（FTS5 trigram + 向量融合 + 置信度 + 拒答）
                                    ├─ 文档导入（6格式解析 + 分块 + 发布）
                                    ├─ 金融计算（int64 定点）
                                    ├─ 单并发 LLM 队列
                                    ├─ SQLite（WAL + FTS5）
                                    ├─ 备份管理 + 服务化（kardianos/service）
                                    └─ Vue 静态资源（//go:embed）
                                          ↑
                              Ollama（qwen2.5:1.5b 开发 / qwen3.5:4b 生产）
```

**做**：本地化汽车金融客服问答、SSE 流式、FAQ 短路、FTS+向量 RAG、来源展示、低置信拒答、金融试算、文档导入、会话落库、PII 脱敏、单并发队列、Ollama 降级容错、Windows 服务化、备份恢复、评测 runner。
**不做**：OCR、复杂扫描件 PDF、企业 SSO、多用户权限、云模型兜底、多门店、自动训练、Reranker、语音客服。

## 目标（Goal）

- **G1**：单 EXE 启动 → SQLite 自动建库迁移 → Vue 聊天页 → 向本地 Ollama 模型提问 → SSE 流式回答 → 会话落库。
- **G2**：FAQ 高置信短路（<500ms 不调模型）、RAG 基于知识库回答（带来源）、低置信/无依据拒答。
- **G3**：CPU 开发机验证链路正确性；生产 RTX 4060 + Qwen3.5 4B 验证性能。
- **成功标准**：`make build` 产单 EXE，CPU 上跑通全部闭环，`go test` 全绿，评测 runner 可运行。

## 当前情况（Status）

- **完成度**：**M1-M9 全部完成**，CPU 验证闭环跑通。
- **代码规模**：65 Go 文件 + 12 Vue 文件（6 页面）；9 测试文件全绿；单 EXE 17.5MB。
- **M1 工程基础**：Go 单 EXE + `modernc.org/sqlite` + migration 按阶段 gating + slog JSON
- **M1 基础聊天**：Ollama 流式 NDJSON + SSE 事件 + 会话/消息 CRUD + 单并发队列 + PII 脱敏 + 降级容错
- **M2 FAQ**：多级匹配器（精确/关键词/编辑距离）+ 短路（104ms）+ CRUD + 批量导入 + 测试匹配
- **M3 文档导入**：6 格式解析（txt/md/html/docx/xlsx/pdf）+ 分块（300-800字/80重叠）+ 发布/停用 + hash 去重
- **M4 FTS5 RAG**：trigram 检索 + 证据上下文 + grounded answer + 来源展示 + 低置信拒答
- **M5 金融计算**：等额本息/等额本金/首付（int64 分定点）+ 单测 + API + 免责声明
- **M6 向量检索**：Ollama Embed（nomic-embed-text 768维）+ 内存余弦索引 + FTS/向量融合 + 批量向量化 + 启动加载（CPU 验证：3片段向量化 1.6s，语义查询召回正确）
- **M7 管理审计**：feedback/audit repo + 反馈/审计/拒答列表 + metrics + 单管理员认证
- **M8 服务化**：`kardianos/service` install/start/stop/uninstall/status + scripts/*.ps1 + 启动检查
- **M9 备份评测**：WAL checkpoint 备份 + retention + 评测 runner（JSONL 数据集）

## 技术栈与架构

- **语言**：Go 1.25.6
- **核心依赖**：
  - `modernc.org/sqlite` —— 纯 Go SQLite（免 CGO，FTS5 trigram）
  - `gopkg.in/yaml.v3` —— 配置
  - `github.com/google/uuid` —— ID
  - `github.com/kardianos/service` —— Windows 服务化
  - 前端：`vue@3 + vue-router + pinia` + dev `vite + typescript`
- **零框架原则**：无 Web 框架（标准库 `net/http`）、无 ORM、`log/slog`。

目录树：

```
auto-finance-assistant/
├── cmd/server/{main.go,service.go}  # 入口 + 服务子命令
├── internal/
│   ├── config/        # 三层配置 + 校验 + 测试
│   ├── storage/       # SQLite + 5 migration + 全 repo + 测试
│   ├── parser/        # 6 格式解析（txt/md/html/docx/xlsx/pdf）
│   ├── knowledge/     # 导入器 + 分块器 + 测试
│   ├── rag/           # FTS + 向量索引 + 融合 + 置信度
│   ├── ollama/        # Client + Chat 流式 + Embed + Health
│   ├── chat/          # 问答编排 + FAQ 匹配 + PII 脱敏 + 测试
│   ├── finance/       # 金融计算（定点）+ 测试
│   ├── queue/         # 单并发 LLM 队列 + 测试
│   ├── backup/        # 备份 + retention
│   ├── api/           # 全部 handler + 路由
│   ├── logging/       # slog JSON
│   └── web/           # //go:embed + SPA
├── evaluations/       # 评测 runner + JSONL 数据集
├── scripts/           # build/install/start/stop/backup.ps1
├── web/               # Vue 3 源码
└── config.{example,dev}.yaml
```

## 如何运行

```bash
make tidy && make web-deps && make web-build && make build
# CPU 验证
./bin/auto-finance-assistant.exe -config config.dev.yaml run
# 生产服务
./bin/auto-finance-assistant.exe -config config.yaml install && ./bin/auto-finance-assistant.exe start
make test
```

## 关键约定

- **配置两套**：`config.dev.yaml`（CPU）/ `config.example.yaml`（RTX 4060 生产）。代码一致仅配置差异。
- **migration 按阶段 gating**：`AllActiveVersions()` 激活全部 5 个。
- **FTS5 trigram**：`unicode61` 对中文不分词，改用 `trigram`（3字符滑窗），短词用 LIKE 兜底。
- **FAQ 短路**：高置信（≥0.85）不调模型；启动加载，CRUD 后刷新（读锁）。
- **RAG 三态**：有结果高置信→grounded answer；有结果低置信→拒答；无结果→降级纯模型（问候/闲聊）。
- **PII 脱敏**：落库前 MaskPII（手机/身份证/银行卡）。
- **降级容错**：Ollama 不可达 → 503 明确提示，不崩溃。
- **服务子命令**：无参数或 `run`=前台；`install/start/stop/uninstall/status`=服务管理。

## 与其他项目的关系

- **工作区第二个 Go 项目**：继 `go-rmm` 后复用其验证过的工程模式（signal shutdown、Go 1.22 方法路由、embed.FS、kardianos/service）。
- **不耦合其他项目**：独立 SQLite + Ollama。

## 技术选型理由（关键决策）

| 决策点 | 选择 | 理由 |
|--------|------|------|
| SQLite 驱动 | `modernc.org/sqlite` | 纯 Go 免 CGO，Windows 无 gcc 可编译 |
| FTS5 tokenizer | `trigram` | 中文友好（3字符切分），unicode61 不分词 |
| HTTP 路由 | std `net/http` + Go 1.22 方法路由 | go-rmm 先例，少依赖 |
| 日志 | `log/slog`（JSON） | 结构化字段，标准库原生 |
| 前端 | Vue 3 + Vite → `//go:embed` | 单二进制交付 |
| 服务化 | `kardianos/service` | go-rmm 已验证 |
| 金融金额 | `int64` 分 | 避免浮点误差 |
| Embedding | `nomic-embed-text` | 专用 embedding 模型（chat 模型不支持 embed） |
