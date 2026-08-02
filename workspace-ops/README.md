# workspace-ops

> 工作区级项目管理工具 —— 扫描 M_X_M 全部项目，统一检查技术栈/依赖/git 状态/测试数/构建产物，存 SQLite，提供 CLI 报告 + Web 看板。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![SQLite](https://img.shields.io/badge/SQLite-modernc-blue)
![license](https://img.shields.io/badge/license-MIT-blue)

## 为什么有这个项目

工作区里有 60+ 个独立项目，跨 TS/Go/Rust/Python/Godot/Flutter 多种技术栈。每次想知道"哪些项目测试覆盖好""哪些 dirty 没提交""技术栈分布"都得手动挨个看。`workspace-ops` 把这个流程自动化：一条命令扫描全工作区，结果入库可查、可报告、可看板。

## 快速开始

```bash
cd workspace-ops

# 扫描工作区（默认 config.dev.yaml，root=.. 即 D:\M_X_M）
make run-scan
# 或：go run ./cmd/ops scan -config config.dev.yaml

# 输出报告（text/json/markdown）
make run-report
go run ./cmd/ops report -format markdown
go run ./cmd/ops report -format json

# 启动 Web 看板（http://127.0.0.1:8765）
make run-serve

# 实跑各项目测试，采集成败 + 耗时（按 slug 过滤）
go run ./cmd/ops test -slug consensus-atlas

# 全部测试
make test
```

扫描真实工作区约 2-3 秒（60 项目 × 6 检查项）。

## 四个子命令

| 子命令 | 作用 | 示例 |
|--------|------|------|
| `scan` | 扫描工作区，结果入库 SQLite | `ops scan -config config.dev.yaml` |
| `report` | 从库读数据，输出 text/json/markdown 报告 | `ops report -format markdown` |
| `serve` | 启动 Web 看板（REST API + 静态页） | `ops serve -port 8766` |
| `test` | 实跑各项目测试，采集成败+耗时+命令 | `ops test -slug consensus-atlas` |

## 检查项（M1）

| 检查项 | 方法 | 示例 fact |
|--------|------|-----------|
| **技术栈识别** | 看标志文件（go.mod/package.json/Cargo.toml/pyproject.toml/project.godot/pubspec.yaml/index.html） | `stack_primary=go`, `stack_all=go,vue` |
| **依赖信息** | 读 go.mod（go 版本+require 数）/ package.json（name+依赖数） | `go_version=1.25.6`, `npm_dep_count=12` |
| **AGENTS.md** | 文件存在性 | `has_agents_md=true` |
| **git 状态** | git CLI（branch + porcelain dirty） | `git_branch=main`, `git_dirty=true` |
| **测试数** | 启发式统计测试文件（`*_test.go`/`*.test.ts`/`test_*.py`），跳过 node_modules/dist/build | `test_count=3041` |
| **构建产物** | 检查 dist/bin/build/target/out 目录 | `build_artifacts=dist,bin` |

## REST API（serve 子命令）

| 方法 路径 | 作用 |
|----------|------|
| `GET /api/projects` | 全部项目（含 facts + 栈汇总） |
| `GET /api/scans` | 扫描历史 |
| `POST /api/rescan` | 触发新扫描 |
| `GET /api/health` | 健康检查 |

首次启动若库为空，会自动扫一次。

## Web 看板

- **M1（当前）**：fallback 页面（内嵌 HTML + 原生 fetch），无需构建前端即可用。展示项目表格 + 栈分布。
- **M2**：完整 Vue 3 看板（`make web-build` 后嵌入）。骨架已就位（`internal/web/web.go` + `//go:embed dist/*`），待前端实现。

## 配置（三层）

```
默认值（defaults.go）→ YAML（config.dev.yaml）→ 命令行 flag
```

关键配置项：
```yaml
scan:
  root: ".."              # 工作区根（相对配置文件）
  ignore_dirs: [node_modules, godot-src, ...]
  checks: {stack: true, dependencies: true, ...}
storage:
  database_path: "workspace-ops.db"
server:
  port: 8765
```

## 核心设计

```
                  ┌─────────────┐
  cmd/ops ───────▶│ scan 子命令  │─── workspace.Discover（项目发现）
                  │             │─── inspector.Inspect（6 项检查）
                  │             │─── storage.SaveFacts（入库）
                  └─────────────┘
                        │
                        ▼
                  ┌─────────────┐
                  │  SQLite     │ projects / project_facts / scans
                  └─────┬───────┘
                        │
              ┌─────────┼─────────┐
              ▼                   ▼
        ┌──────────┐        ┌──────────┐
        │ report   │        │ serve    │
        │ text/json│        │ REST API │
        │ markdown │        │ + Web UI │
        └──────────┘        └──────────┘
```

### 关键约定（对齐工作区 Go 项目公约）

- **模块名**：`github.com/QiuShichang/workspace-ops`
- **零 Web 框架**：标准库 `net/http` + Go 1.22 方法路由
- **零 ORM**：`database/sql` + 手写 repo
- **零 CGO**：`modernc.org/sqlite` 纯 Go
- **日志**：`log/slog`（JSON/text）
- **配置**：defaults → yaml → flag 三层
- **报告多格式**：`report.Reporter` 接口 + `Registry`（仿 `generic-admin/export` 范式），新增格式零改框架
- **前端嵌入**：`//go:embed dist/*`（构建后）+ fallback.html（未构建）

## 目录结构

```
workspace-ops/
├── go.mod / Makefile / README.md / AGENTS.md / DEPLOY.md
├── config.example.yaml / config.dev.yaml
├── cmd/ops/main.go              # scan/report/serve/test 四子命令入口
├── internal/
│   ├── workspace/               # 项目发现（Discover + 标志文件识别）
│   ├── inspector/               # 检查器（stack/deps/agents/git/tests/artifacts）
│   ├── storage/                 # SQLite + embed migration + repos
│   │   └── migrations/001_init.sql
│   ├── report/                  # 多格式报告（text/json/markdown + Registry）
│   ├── api/                     # REST（Go 1.22 方法路由）
│   ├── config/                  # 三层配置
│   ├── logging/                 # slog 封装
│   └── web/                     # 前端嵌入 + fallback
└── web/                         # Vue 3 + Vite（M2 待实现）
```

## M2 路线

- `.gitignore` 规范检查
- 跨项目依赖版本对齐报告
- 完整 Vue 3 看板（健康卡片 / 栈分布图 / 测试统计 / 构建状态）
- `test` 子命令并行化（当前串行跑各项目）

## 相关项目

- [`generic-admin`](../generic-admin) —— report 多格式 + storage 范式的范本
- [`go-rmm`](../go-rmm) —— CLI 子命令分发的范本
- [`auto-finance-assistant`](../auto-finance-assistant) —— 配置三层加载 + 前端 embed 的范本
