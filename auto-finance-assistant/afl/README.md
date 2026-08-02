# 汽车金融本地智能客服系统

> **9 个 Milestone 全部完成** · M1-M9 全部交付并 CPU 验证 · 生产部署只需换配置（RTX 4060 + Qwen3 4B）

## 项目定位

在一台 Windows 电脑上运行的**轻量、本地化**汽车金融客服问答系统。

- **后端**：Go 单 EXE 服务（标准库 `net/http` + Go 1.22 方法路由）
- **数据库**：SQLite（`modernc.org/sqlite` 纯 Go，免 CGO，WAL 模式 + FTS5 trigram）
- **模型**：Ollama 本地模型（生产 Qwen3 4B / 开发 qwen2.5:3b）
- **前端**：Vue 3 + Vite + Pinia，构建产物经 `//go:embed` 嵌入 Go 二进制
- **不引入** Docker、WSL2、Redis、PostgreSQL、Qdrant、Python 常驻服务

## 两套配置

| 配置文件 | 环境 | 模型 | 推理 |
|----------|------|------|------|
| `config.example.yaml` | 生产（RTX 4060 8GB） | `qwen3:4b` + `qwen3-embedding:0.6b` | `num_gpu` GPU offload |
| `config.dev.yaml` | 开发机（AMD 4750U CPU） | `qwen2.5:3b` + `nomic-embed-text` | `num_thread:8` CPU |

**代码完全一致**，仅配置差异。CPU 上每次回答约 10-30s，验证链路与正确性而非速度。

## 快速部署（3 步）

**方式 A：一键脚本（Windows PowerShell，推荐）**

```powershell
# 1. 安装 Ollama（如未装）：https://ollama.com/download
ollama pull qwen2.5:3b         # CPU 验证模型（生产用 qwen3:4b）
ollama pull nomic-embed-text   # 向量模型

# 2. 一键构建（检查 Go+Node → 构建前端 → 构建 EXE → 生成 config.yaml）
.\scripts\setup.ps1 -Dev       # -Dev = CPU 验证模式；不加 = 生产模式

# 3. 启动
.\scripts\start.ps1            # 前台运行
.\scripts\install-service.ps1  # 或安装为服务（需管理员）
```

**方式 B：手动（跨平台）**

```bash
# 前置：Go 1.22+、Node 18+、Ollama
ollama pull qwen2.5:3b && ollama pull nomic-embed-text

# 构建（前端 + Go 一步完成）
make build

# 生成配置（首次）
cp config.dev.yaml config.yaml   # CPU 验证；生产用 config.example.yaml

# 运行
./bin/auto-finance-assistant.exe -config config.yaml run
```

浏览器打开 `http://127.0.0.1:8080` 即可使用。

> **配置不存在不崩溃**：`-config` 文件不存在时自动用默认配置启动。启动时探测 Ollama，模型缺失会打印 `ollama pull` 命令。

## 运行依赖

| 组件 | 构建时 | 运行时 |
|------|--------|--------|
| Go 1.22+ | ✅ | ❌（静态编译） |
| Node 18+ | ✅（构建前端） | ❌（嵌入 EXE） |
| gcc/CGO | ❌ | ❌（纯 Go SQLite） |
| Ollama + 模型 | ❌ | ✅ |

**单 EXE 17.5MB，无运行时依赖**（除 Ollama 外）。

## API（全部 M1-M9）

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/health` | 健康检查 | - |
| GET | `/api/system/info` `/model` | 系统信息 | - |
| GET | `/api/metrics` | 运行指标 | - |
| POST/GET | `/api/conversations[/{id}]` | 会话 CRUD | - |
| POST | `/api/chat` `/api/chat/stream` | 问答（FAQ 短路 / RAG / 纯模型） | - |
| CRUD | `/api/faqs[/{id}]` | FAQ 管理 | - |
| POST | `/api/faqs/import` `/api/faqs/test` | 批量导入 / 测试匹配 | - |
| CRUD | `/api/documents[/{id}]` | 文档管理 | - |
| POST | `/api/documents/{id}/publish` `/disable` `/reparse` `/embed` | 发布/停用/重解析/向量化 | - |
| GET | `/api/documents/{id}/chunks` | 查看分块 | - |
| POST | `/api/finance/equal-payment` `/equal-principal` `/down-payment` | 金融试算 | - |
| POST | `/api/feedback` | 用户反馈 | - |
| GET | `/api/feedback` `/audit/logs` `/refused` | 反馈/审计/拒答列表 | ✓ |
| POST | `/api/system/backup` GET `/api/system/backups` | 备份 | ✓ |

## Milestone 路线图（全部完成）

| M | 阶段 | 状态 | 核心交付 |
|---|------|------|---------|
| **1** | 工程基础 + 基础聊天 | ✅ | Go 单 EXE + SQLite + Ollama SSE 流式 + Vue 聊天页 + 队列 + PII 脱敏 |
| **2** | FAQ | ✅ | 多级匹配（精确/关键词/编辑距离）+ 短路（104ms 不调模型）+ CRUD + 批量导入 |
| **3** | 文档导入 + 分块 | ✅ | TXT/MD/HTML/DOCX/XLSX/PDF 解析 + 分块（300-800字/80重叠）+ 发布/停用 + hash 去重 |
| **4** | FTS5 RAG + 来源 + 拒答 | ✅ | FTS5 trigram 检索 + 证据上下文 + grounded answer + 来源展示 + 低置信拒答 |
| **5** | 金融计算 | ✅ | 等额本息/等额本金/首付（int64 分定点）+ 单元测试 + 计算工具路由 + 免责声明 |
| **6** | 向量检索 | ✅ | Ollama Embed（nomic-embed-text 768维）+ 内存余弦索引 + FTS/向量融合 + 文档批量向量化 + 启动加载 |
| **7** | 管理 + 审计 + 反馈 | ✅ | feedback/audit repo + 反馈/审计/拒答列表 API + metrics + 单管理员认证 |
| **8** | Windows 部署 + 服务化 | ✅ | `kardianos/service` install/start/stop/uninstall/status + scripts/*.ps1 + 启动检查清单 |
| **9** | 备份 + 评测 | ✅ | SQLite WAL checkpoint 备份 + retention（留 N 份）+ 评测 runner（JSONL 数据集） |

## 架构

```
Windows 11
│
├─ Ollama（CPU：qwen2.5:3b + nomic-embed-text；生产：qwen3:4b + qwen3-embedding:0.6b）
│
└─ auto-finance-assistant.exe
   ├─ HTTP API (net/http + Go 1.22 方法路由)
   ├─ Vue 静态页面 (//go:embed dist/*)
   ├─ 客服问答编排 (脱敏→FAQ短路→RAG检索→grounded answer→落库)
   ├─ FAQ 匹配器 (精确/关键词/编辑距离，内存索引)
   ├─ RAG 服务 (FTS5 trigram + 向量融合 + 置信度 + 拒答)
   ├─ 文档导入 (6格式解析 + 分块 + 发布)
   ├─ 金融计算 (int64 定点)
   ├─ 单并发 LLM 队列 (信号量 + 等待席 + ctx 取消)
   ├─ SQLite (modernc.org/sqlite, WAL + FTS5)
   ├─ 备份管理 (WAL checkpoint + retention)
   └─ slog JSON 日志 + 服务化 (kardianos/service)
```

## 技术选型理由

| 项 | 选择 | 理由 |
|----|------|------|
| SQLite 驱动 | `modernc.org/sqlite` | 纯 Go 免 CGO，Windows 无 gcc 可编译，支持 FTS5 |
| FTS5 tokenizer | `trigram` | 中文友好（按 3 字符切分），优于 unicode61（中文不分词） |
| HTTP 路由 | std `net/http` | 工作区 go-rmm 先例；Go 1.22 方法路由够用 |
| 日志 | `log/slog`（JSON） | 原计划要结构化字段，slog 标准库原生支持 |
| 前端嵌入 | `//go:embed` | Vue 构建产物打进单 EXE |
| 服务化 | `kardianos/service` | go-rmm 已验证的跨平台方案 |
| 金融金额 | `int64` 分 | 避免浮点累计误差 |

## 关键文件

- `cmd/server/` — 入口（main.go + service.go 服务子命令）
- `internal/config/` — 三层配置 + 校验
- `internal/storage/` — SQLite + 5 migration + 全 repo（会话/消息/设置/文档/片段/FAQ/反馈/审计）
- `internal/parser/` — 6 格式解析器（txt/md/html/docx/xlsx/pdf）
- `internal/knowledge/` — 导入器 + 分块器
- `internal/rag/` — FTS 检索 + 向量索引 + 融合 + 置信度 + 上下文构造
- `internal/chat/` — 问答编排 + FAQ 匹配 + PII 脱敏
- `internal/finance/` — 金融计算（定点）
- `internal/backup/` — 备份 + retention
- `evaluations/` — 评测 runner + 数据集
- `scripts/` — build/install/start/stop/backup.ps1

## 代码规模

65 Go 文件 + 12 Vue 文件（6 页面）· 9 测试文件全绿 · 单 EXE 17.5MB（含嵌入完整前端）

## 前端页面（6 个）

| 页面 | 路由 | 功能 |
|------|------|------|
| 客服问答 | `/chat` | SSE 流式问答 + 来源展示 + 会话切换 |
| 知识库 | `/knowledge` | 上传/列表/发布/停用/查看分块/向量化/删除 |
| FAQ 管理 | `/faq` | CRUD + 启停 + 测试匹配 |
| 会话历史 | `/history` | 会话列表 + 消息详情 + 点赞/点踩反馈 |
| 金融试算 | `/finance` | 等额本息/等额本金/首付计算器 |
| 系统设置 | `/settings` | 健康状态 + 运行指标 + 备份管理 |
