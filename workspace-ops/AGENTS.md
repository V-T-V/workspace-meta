# workspace-ops · AGENTS.md

## 项目内容（What）

Go 1.25 + SQLite + Vue 的**工作区级项目管理工具**——扫描 M_X_M 全部 60+ 项目，统一检查技术栈/依赖/git 状态/测试数/构建产物，结果存 SQLite，提供 CLI 报告（text/json/markdown）+ Web 看板（REST API + 静态页）。

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

**做**：6 项静态检查（stack/deps/agents_md/git/tests/build_artifacts）+ SQLite 持久化 + 3 种报告格式 + REST API + fallback Web 页 + test 子命令（按栈实跑测试采集成败/耗时）。
**不做**：真 Vue 看板（M2 已搭骨架）、生产级服务化（kardianos/service）、CI 集成、test 并行化（当前串行）。

## 目标（Goal）

- **G1**：一条命令扫描全工作区（~2 秒），识别全部项目 + 6 项检查全部正确。
- **G2**：结果入库可查询、可历史对比（scans 表记录每次扫描）。
- **G3**：CLI 报告 3 种格式（text 终端表格 / json 机器可读 / markdown 文档可贴）。
- **G4**：serve 子命令提供 REST API + Web 看板（未构建前端时用 fallback HTML）。
- **成功标准**：`go test ./...` 全绿 + scan/report/serve/test 四子命令端到端跑通 + 真实工作区 60+ 项目全部正确识别。

## 当前情况（Status）

- **完成度**：**M1 完成**——四子命令端到端跑通，真实工作区扫描验证通过（约 60 项目）
- **实测结果**（2026-08-01 扫描 D:\M_X_M）：
  - 发现 59 个项目（含 consensus-atlas + workspace-ops 自身）
  - 栈分布：node/ts=28, go=7, rust=4, godot=3, flutter=2, python=2, html=2, unknown=11
  - 测试数采集正确（algorithms-atlas=3041, web-game-research=611 等）
  - git 分支 + dirty 状态正确
- **代码规模**：10 个 Go 包，约 1500 行；`go test ./...` 全绿（workspace/inspector/report 三包有测试）
- **底座**：
  - `internal/config`：三层配置（defaults → yaml → flag），ResolveRoot/ResolveDBPath
  - `internal/workspace`：Discover 项目发现（11 种标志文件识别，跳过 ignoreDirs + 隐藏目录）
  - `internal/inspector`：6 项检查 + Facts KV 结构 + 启发式测试计数（跳过 node_modules/dist/build）
  - `internal/storage`：SQLite（modernc 纯 Go）+ embed migration + scan_repo/project_repo
  - `internal/report`：Reporter 接口 + Registry（text/json/markdown 三种）
  - `internal/api`：REST（Go 1.22 方法路由）+ Resolver 封装扫描流程
  - `internal/testrunner`：test 子命令用，按栈选命令实跑测试采集成败+耗时
  - `internal/web`：`//go:embed dist/*` + fallback.html + SPA handler
  - `internal/logging`：slog JSON/text 封装

## 技术栈与架构

- **语言**：Go 1.25.6
- **依赖**：`modernc.org/sqlite`（纯 Go 免 CGO）+ `gopkg.in/yaml.v3` + `github.com/google/uuid`，**零 Web 框架**（标准库 net/http + Go 1.22 方法路由）
- **设计参考**：
  - `generic-admin`（report 多格式 + storage 范式）
  - `go-rmm/cmd/go-rmm`（CLI 子命令分发）
  - `auto-finance-assistant`（配置三层 + 前端 embed）
- **目录**：cmd + internal（标准 Go 布局）

```
workspace-ops/
├── cmd/ops/main.go              # scan/report/serve/test 四子命令
├── internal/
│   ├── workspace/{workspace.go,workspace_test.go}
│   ├── inspector/{inspector.go,git.go,inspector_test.go}
│   ├── storage/{database.go,migrations.go,scan_repo.go,project_repo.go,migrations/001_init.sql}
│   ├── report/{report.go,report_test.go}
│   ├── api/server.go
│   ├── testrunner/{testrunner.go,testrunner_test.go}
│   ├── config/{config.go,defaults.go}
│   ├── logging/logging.go
│   └── web/{web.go,fallback.html,dist/.gitkeep}
└── web/                         # Vue 3（M2）
```

## 如何运行

```bash
make run-scan      # 扫描工作区，入库
make run-report    # text 报告
make run-serve     # Web 看板（http://127.0.0.1:8765）

go run ./cmd/ops scan -config config.dev.yaml
go run ./cmd/ops report -format markdown
go run ./cmd/ops serve -port 8766
go run ./cmd/ops test -slug consensus-atlas   # 实跑某项目测试采集成败

make test          # 全部测试
make vet           # 静态检查
make build         # 构建到 bin/
make release       # 5 平台跨编译到 dist/
```

首次启动 serve 若库为空，会自动扫一次。

## 关键约定

- **模块名**：`github.com/QiuShichang/workspace-ops`（沿用工作区 `github.com/QiuShichang/` 前缀公约）
- **零 Web 框架**：标准库 `net/http` + Go 1.22 方法路由（`mux.HandleFunc("GET /api/projects", ...)`）
- **零 ORM**：`database/sql` + 手写 repo（`*_repo.go`）
- **零 CGO**：`modernc.org/sqlite` 纯 Go（Windows 无 gcc 可编译）
- **报告多格式**：`report.Reporter` 接口 + `Registry`，新增格式只需实现接口注册，零改框架（仿 `generic-admin/export`）
- **前端嵌入**：`//go:embed dist/*`；未构建时 `HasDist()=false` 自动降级到 `fallback.html`
- **测试文件计数**：启发式跳过 `node_modules`/`.git`/`dist`/`build`/`target`/`vendor`
- **配置三层**：defaults.go → yaml → flag（后者覆盖前者）
- **日志**：`log/slog`（JSON 默认，serve 用 text 便于调试）

## 与其他项目的关系

- **与 [`generic-admin`](../generic-admin) 同范式**：report 的 `Reporter`+`Registry` 直接对齐 generic-admin 的 `Exporter`+`Registry`；storage 的 `Open`+embed migration 同源。
- **与 [`go-rmm`](../go-rmm) 同范式**：cmd/ops 的子命令分发（scan/report/serve）参考 go-rmm/cmd/go-rmm 的 flag + 子命令风格。
- **与 [`auto-finance-assistant`](../auto-finance-assistant) 同范式**：config 三层加载 + `//go:embed dist/*` 前端嵌入。
- **工作区定位**：这是工作区的"元工具"——服务于整个工作区（扫描所有 60+ 项目），本身也是工作区一员。补全工作区在"后端 CLI 工具 + 数据层"象限的空白。
