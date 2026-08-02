# obs-lite

> 轻量可观测性平台 —— metrics（counter/gauge/histogram）+ trace（span 树），零依赖纯 Go，对齐 Prometheus/OpenTelemetry 数据模型。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![零依赖](https://img.shields.io/badge/dependencies-0-success)
![license](https://img.shields.io/badge/license-MIT-blue)

## 为什么有这个项目

工作区有 go-agent-research 的 trace（内嵌，非独立产品），但**没有独立的可观测性平台**。`obs-lite` 补这一环：轻量 metrics + trace 收集，零依赖，可作为库嵌入任何 Go 服务。

## 核心能力

### Metrics（3 种）
| 类型 | 用途 | 示例 |
|------|------|------|
| **Counter** | 单调递增 | `http_requests_total` |
| **Gauge** | 可增可减瞬时值 | `active_connections` |
| **Histogram** | 分布 | `request_duration_seconds` |

带标签维度（如 `{method:"GET",status:"200"}`），支持 Histogram 桶配置。

### Trace（span 树）
- **Span**：一次操作（HTTP 请求/DB 查询）
- **Trace**：调用链（共享 TraceID）
- **ParentID**：父子关系（树形）
- **事件/属性/状态**：每个 span 可记录属性、事件、错误状态

## 快速开始

```bash
cd obs-lite

go run ./cmd/obs -d metrics   # metrics demo
go run ./cmd/obs -d trace     # trace demo（span 树）
go run ./cmd/obs -d all       # 全部
make test                     # 全部测试
```

## 作为库使用

```go
package main

import (
	"github.com/QiuShichang/obs-lite/internal/metrics"
	"github.com/QiuShichang/obs-lite/internal/trace"
)

func main() {
	// Metrics
	reg := metrics.NewRegistry()
	counter := reg.Counter("requests")
	counter.Inc(map[string]string{"method": "GET"})

	// Trace
	tr := trace.NewTracer()
	ctx, root := tr.Start("handler", nil)
	defer root.End()
	_, db := tr.Start("db.query", ctx)
	defer db.End()
}
```

## 核心设计

- **对齐 Prometheus/OpenTelemetry 数据模型**（但零依赖，无 HTTP 端点/无 protobuf）
- **线程安全**：metric 用 sync.Mutex 保护
- **Registry**：注册并收集所有 metric
- **Span 自动注册**：End() 时自动加入 Tracer

## 目录结构

```
obs-lite/
├── cmd/obs/main.go          # demo 入口
├── internal/
│   ├── types/               # 共享：MetricPoint/Span/HistogramData
│   ├── metrics/             # Counter/Gauge/Histogram + Registry
│   ├── trace/               # Tracer/Span/Context（span 树）
│   └── exporter/            # 文本/Prometheus 格式输出
```

## 相关项目

- [`go-agent-research`](../go-agent-research) —— 内嵌的 trace 基础设施（本库是独立产品版）
- [`workspace-ops`](../workspace-ops) —— 同范式的 Go 零依赖工具
