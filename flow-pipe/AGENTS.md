# flow-pipe · AGENTS.md

## 项目内容（What）

Go 1.25 + SQLite 的**轻量数据管道 / ETL 平台**——YAML 定义管道，DAG 拓扑排序编排，source/transform/sink 三类可插拔连接器，单机首期执行，架构层预留分布式 worker（M3）。

```
   YAML 管道定义
        │
        ▼
   ┌──────────┐    拓扑排序      ┌──────────┐
   │ loader   │ ───────────────▶ │ runner   │
   │ Parse()  │                  │ Run()    │
   └──────────┘                  └────┬─────┘
                                      │ 按序执行步骤
                     ┌────────────────┼────────────────┐
                     ▼                ▼                ▼
               ┌──────────┐    ┌──────────┐    ┌──────────┐
               │ source   │    │ transform│    │ sink     │
               │ Registry │    │ Registry │    │ Registry │
               └──────────┘    └──────────┘    └──────────┘
```

**做**：DAG 编排（Kahn 拓扑排序 + 循环检测）+ 11 连接器（4 source 含 http + 3 transform 含 map + 4 sink 含 http）+ YAML 管道定义 + REST API + SQLite 存管道/运行历史 + 定时调度（-schedule flag 接线）。
**不做**：真分布式 worker（M3）/ 真 Web 看板（M2）/ 生产级连接器（kafka/mysql）。

**已做（M1 + 深化）**：retry 重试机制 + dead_letter 死信兜底 + http source/sink + map transform（lookup/cast/template）。

## 目标（Goal）

- **G1**：YAML 定义管道，单机端到端跑通（csv → filter → sqlite 闭环）。
- **G2**：连接器插件化——新增 source/transform/sink 只需实现接口 + init 注册，零改框架。
- **G3**：DAG 正确编排，支持线性链和分支，检测循环依赖。
- **G4**：REST API 能触发运行、存历史、查历史。
- **G5**：架构层为 M3 分布式留好接口（proto 包协议定义 + cmd/worker 占位），但不提前实装。
- **成功标准**：`go test ./...` 全绿 + 3 个示例管道跑通 + REST API 端到端验证。

## 当前情况（Status）

- **完成度**：**M1 完成**——单机 ETL 闭环可用
- **核心**（`internal/pipeline`，已完成）：
  - `connector.go`：Row/Rows + 三类连接器接口 + Register/Get/Registry
  - `dag.go`：Step/Pipeline + TopoSort（Kahn）+ Validate + 循环检测
  - `runner.go`：Run 按拓扑序执行，RunResult/StepResult
  - `loader.go`：Parse/LoadFromFile 解析 YAML
- **连接器**（已完成）：
  - source：csv / json / generate
  - transform：filter（where 表达式）/ field（add/rename/drop）
  - sink：stdout / csv / sqlite（自动建表）
- **基础设施**（已完成）：config（三层）/ logging（slog）/ storage（SQLite + embed migration）/ scheduler（M1 最小定时）
- **协议预留**（`internal/proto`，M1 定义 M3 实装）：Envelope/TaskPayload/TaskResultPayload/WorkerInfo

## 技术栈与架构

- **语言**：Go 1.25.6
- **依赖**：`modernc.org/sqlite`（纯 Go）+ `gopkg.in/yaml.v3` + `github.com/google/uuid`，**零 Web 框架**
- **设计参考**：
  - `generic-admin/internal/export`（连接器 Interface+Registry 范式）
  - `auto-finance-assistant`（服务入口 + 配置三层 + embed）
  - `go-rmm`（M3 分布式 worker 的 relay/agent 反向 WS 参考）
- **目录**：cmd + internal（标准 Go 布局）

```
flow-pipe/
├── cmd/{server,worker}/main.go
├── internal/
│   ├── pipeline/{connector,dag,runner,loader,pipeline_test}.go
│   ├── source/{source,csv,json_source}.go
│   ├── transform/{filter,field}.go
│   ├── sink/{stdout,csv_sink,sqlite_sink}.go
│   ├── scheduler/scheduler.go
│   ├── proto/proto.go
│   ├── storage/{database,migrations,pipeline_repo,run_repo}.go
│   ├── config/{config,defaults}.go
│   └── logging/logging.go
└── examples/{generate-to-stdout,csv-to-sqlite,json-to-csv}.yaml + data/
```

## 如何运行

```bash
make run-example      # generate → stdout（最简验证）
make run-example-csv  # csv → filter → sqlite（ETL 闭环）
make run-schedule     # 定时循环跑示例（每 60s，Ctrl+C 退出）
make run-server       # REST API（http://127.0.0.1:8767）

go run ./cmd/server -config config.dev.yaml -run examples/generate-to-stdout.yaml
go run ./cmd/server -config config.dev.yaml -run examples/csv-to-sqlite.yaml -recover   # 状态恢复

make test             # 全部测试
make build            # 构建 server + worker
```

## 关键约定

- **模块名**：`github.com/QiuShichang/flow-pipe`
- **连接器插件化**：实现 `SourceConnector`/`TransformConnector`/`SinkConnector` 接口 + `init()` 注册；cmd/main 必须**匿名 import** 连接器包触发注册（`_ ".../internal/source"` 等）
- **零 Web 框架**：标准库 net/http + Go 1.22 方法路由
- **零 ORM**：database/sql + 手写 repo
- **零 CGO**：modernc.org/sqlite 纯 Go
- **不提前实装分布式**：proto 包 M1 只有协议定义，worker cmd 是占位打印说明。M3 才实装反向 WS 传输。
- **DAG 确定性**：同层步骤按 ID 字典序执行
- **配置三层**：defaults → yaml → flag

## 与其他项目的关系

- **与 [`generic-admin`](../generic-admin) 同范式**：连接器的 Interface+Registry 直接对齐 generic-admin 的 Exporter+Registry；storage 的 Open+embed migration 同源。
- **与 [`auto-finance-assistant`](../auto-finance-assistant) 同范式**：服务入口三段式 + 配置三层 + 前端 embed。
- **与 [`go-rmm`](../go-rmm) 的关系**：M3 分布式 worker 将参考 go-rmm 的 relay/agent 反向 WS 连接模式（worker 主动连 server）。
- **与 [`workspace-ops`](../workspace-ops) 同范式**：config/logging/storage 三件套完全同源（workspace-ops 先做，flow-pipe 复用）。
- **工作区定位**：补全工作区在"数据工程 / ETL"象限的空白。
