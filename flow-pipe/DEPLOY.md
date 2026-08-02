# flow-pipe · 部署

> 单二进制 + SQLite + YAML 管道文件，部署极简。

## 本地运行

```bash
cd flow-pipe

# 跑一次管道（不启服务）
make run-example         # generate → stdout（最简）
make run-example-csv     # csv → filter → sqlite（ETL 闭环）
go run ./cmd/server -run examples/json-to-csv.yaml

# 启 REST API 服务
make run-server
# → http://127.0.0.1:8767

# 全部测试
make test
```

## 构建

```bash
make build      # server + worker 到 bin/
make release    # 跨平台到 dist/
```

二进制自带 SQLite（modernc 纯 Go，免 CGO）+ 所有连接器，无外部依赖。

## 配置

复制 `config.example.yaml` 为 `config.yaml`：

```yaml
storage:
  database_path: "flow-pipe.db"   # 管道定义 + 运行历史
server:
  port: 8767
worker:
  enabled: false                  # M3 才用
```

## 管道定义（YAML）

```yaml
name: my-pipeline
steps:
  - id: read
    type: source
    connector: csv
    config: { path: "in.csv" }
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

规则：source 无依赖；transform/sink 必须有 depends_on。

## REST API

| 方法 路径 | 说明 |
|----------|------|
| `POST /api/run` | body=管道 YAML，跑一次返回结果 |
| `POST /api/run-file?path=x.yaml` | 从文件加载跑 |
| `GET /api/pipelines` | 列已存管道 |
| `POST /api/pipelines` | `{name, yaml}` 存管道 |
| `GET /api/runs?limit=20` | 运行历史 |
| `GET /api/health` | 健康检查 |

示例：
```bash
curl -X POST http://127.0.0.1:8767/api/run -d @my-pipe.yaml
curl http://127.0.0.1:8767/api/runs
```

## 数据

- `flow-pipe.db`：SQLite（运行时生成）
- 两张表：`pipelines`（存管道定义）/ `runs`（运行历史）
- 删 `*.db*` 即重置

## 不需要的

- 无 systemd/service（前台跑）
- 无 Docker（单二进制）
- 无反向代理（本地 127.0.0.1）
- 无 TLS（本地）
