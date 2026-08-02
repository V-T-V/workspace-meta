# workspace-ops · 部署

> 这是自用工具，部署极简：单二进制 + SQLite 文件 + 配置文件。

## 本地运行（开发）

```bash
cd workspace-ops

# 1. 扫描（首次）
make run-scan

# 2. 看报告
make run-report

# 3. 起 Web 看板
make run-serve
# → http://127.0.0.1:8765
```

## 构建

```bash
# 单二进制（当前平台）
make build
# → bin/workspace-ops.exe（Windows）

# 跨平台
make release
# → dist/workspace-ops-{windows,linux,darwin}-{amd64,arm64}
```

二进制自带 SQLite 驱动（modernc 纯 Go，免 CGO），无外部依赖。

## 配置

复制 `config.example.yaml` 为 `config.yaml`，按需修改：

```yaml
scan:
  root: ".."              # 工作区根目录
  ignore_dirs: [...]      # 忽略的目录
storage:
  database_path: "workspace-ops.db"  # SQLite 文件路径
server:
  port: 8765
```

## 数据

- SQLite 文件：`workspace-ops.db`（运行时生成，含 WAL/SHM）
- 三张表：`scans`（扫描历史）/ `projects`（项目）/ `project_facts`（检查结果 KV）
- 删除 `*.db*` 文件即可重置，下次 scan 自动重建

## 前端（M2）

当前用 fallback HTML（无需构建）。完整 Vue 看板：

```bash
make web-build    # npm install + npm run build + 拷贝 dist
make build        # 重新构建二进制（嵌入新 dist）
```

## 不需要的

- 无 systemd/service 配置（自用工具，直接前台跑）
- 无 Docker（单二进制足够）
- 无反向代理（本地 127.0.0.1 监听）
- 无 TLS（本地）
