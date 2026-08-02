# flow-pipe

> 轻量数据管道 / ETL 平台 —— YAML 定义管道，DAG 编排，source/transform/sink 可插拔连接器，单机首期，架构预留分布式 worker。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![SQLite](https://img.shields.io/badge/SQLite-modernc-blue)
![license](https://img.shields.io/badge/license-MIT-blue)

## 为什么有这个项目

工作区在"数据层"是空白——所有项目都是"代码 + 内容"，没有一个做数据管道。`flow-pipe` 补这一环：用最小可运行的 ETL 引擎，让"从 CSV/JSON 读 → 变换 → 写入 SQLite/文件"这类常见数据流通过一个 YAML 文件就能定义和执行。架构上预留了分布式 worker（M3），但不提前实装。

## 快速开始

```bash
cd flow-pipe

# 跑最简示例（generate → stdout，无需外部文件）
make run-example
# 或：go run ./cmd/server -run examples/generate-to-stdout.yaml

# 跑 csv → filter → sqlite 示例（典型 ETL 闭环）
make run-example-csv

# 启动 server（REST API，端口 8767）
make run-server

# 定时循环跑示例管道（每 60s 一次，Ctrl+C 退出）
make run-schedule
# 或：go run ./cmd/server -schedule examples/generate-to-stdout.yaml -interval 60

# 状态恢复重跑（跳过最近一次已成功的步骤）
go run ./cmd/server -run examples/csv-to-sqlite.yaml -recover

# 全部测试
make test
```

## YAML 管道定义

```yaml
name: demo-csv-to-sqlite
steps:
  - id: read
    type: source
    connector: csv
    config: { path: "data/in.csv" }
  - id: clean
    type: transform
    connector: filter
    config: { where: "amount > 0" }
    depends_on: [read]
  - id: write
    type: sink
    connector: sqlite
    config: { path: "out.db", table: "records" }
    depends_on: [clean]
```

- **source**：无 depends_on（数据起点）
- **transform/sink**：必须有 depends_on（定义 DAG 边）
- 支持 DAG 分支/合并（不只线性链）

## 连接器（M1）

| 类别 | 连接器 | 说明 |
|------|--------|------|
| **source** | `csv` | 读 CSV（首行 header） |
| | `json` | 读 JSON 数组 |
| | `generate` | 造测试数据（无需文件） |
| | `http` | 从 HTTP GET 读 JSON |
| **transform** | `filter` | where 表达式过滤（`field op value`） |
| | `field` | 字段增/删/改名 |
| | `map` | 值映射/类型转换/模板拼接（lookup/cast/template） |
| **sink** | `stdout` | 打印到标准输出（json/table） |
| | `csv` | 写 CSV |
| | `sqlite` | 写 SQLite（自动建表） |
| | `http` | POST 到 HTTP endpoint |

新增连接器：实现 `SourceConnector`/`TransformConnector`/`SinkConnector` 接口 + `init()` 里 Register，零改框架。

## 核心设计

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

- **可插拔连接器**：三类连接器各有 Registry（仿 `generic-admin/export` 的 Interface+Registry）
- **DAG 编排**：拓扑排序（Kahn 算法）+ 循环依赖检测
- **数据流**：每步输出暂存，下游合并所有依赖的输出作为输入
- **确定性**：同层步骤按 ID 字典序执行

## REST API（server 子命令）

| 方法 路径 | 作用 |
|----------|------|
| `POST /api/run` | body 是管道 YAML，跑一次，返回结果 |
| `POST /api/run-file?path=xxx` | 从文件加载并跑 |
| `GET/POST /api/pipelines` | 列出/保存管道定义 |
| `GET /api/runs?limit=20` | 运行历史 |
| `GET /api/health` | 健康检查 |

## 里程碑

- **M1（当前）**：单机闭环——DAG 编排 + 11 连接器（4 source 含 http + 3 transform 含 map + 4 sink 含 http）+ YAML 定义 + REST API + SQLite 存历史 + retry/dead_letter + 状态恢复（-recover）
- **M2 候选**：真 Web 看板（DAG 可视化）/ 更多连接器（kafka/mysql）/ cron 表达式调度 / 更丰富的 where 表达式
- **M3 分布式**：实装 `cmd/worker` + proto 协议（反向 WS，参考 go-rmm relay/agent）+ 任务分片 + worker 心跳

## 关键约定（对齐工作区 Go 项目公约）

- **模块名**：`github.com/QiuShichang/flow-pipe`
- **零 Web 框架**：标准库 `net/http` + Go 1.22 方法路由
- **零 ORM**：`database/sql` + 手写 repo
- **零 CGO**：`modernc.org/sqlite` 纯 Go
- **连接器插件化**：`pipeline.SourceConnector` 等接口 + `RegisterSource` 等 Registry
- **不提前实装分布式**：proto 包 M1 只有协议定义，worker cmd 是占位

## 目录结构

```
flow-pipe/
├── go.mod / Makefile / README.md / AGENTS.md / DEPLOY.md
├── config.{example,dev}.yaml
├── cmd/
│   ├── server/main.go        # server 入口（REST + 单机执行）
│   └── worker/main.go        # M3 占位
├── internal/
│   ├── pipeline/             # 核心：connector 接口 + DAG + runner + loader
│   ├── source/               # csv/json/generate 连接器
│   ├── transform/            # filter/field 连接器
│   ├── sink/                 # stdout/csv/sqlite 连接器
│   ├── scheduler/            # 定时调度（-schedule flag 接线）
│   ├── proto/                # worker 协议（M1 定义，M3 实装）
│   ├── storage/              # SQLite + 管道/运行历史
│   ├── config/               # 三层配置
│   └── logging/              # slog 封装
├── examples/                 # 3 个示例管道 + 测试数据
└── web/                      # Vue 3（M2 待建，当前空）
```

> REST API handler 直接写在 `cmd/server/main.go`（单文件，避免过早拆 internal/api 包）。

## 相关项目

- [`generic-admin`](../generic-admin) —— 连接器插件化（Interface+Registry）的范本
- [`auto-finance-assistant`](../auto-finance-assistant) —— 服务入口 + 配置三层 + embed 范式
- [`go-rmm`](../go-rmm) —— M3 分布式 worker 的反向 WS 协议参考（relay/agent）
