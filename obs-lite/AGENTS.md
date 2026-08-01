# obs-lite · AGENTS.md

## 项目内容（What）

Go 1.25 纯标准库的**轻量可观测性平台**——metrics（counter/gauge/histogram）+ trace（span 树），对齐 Prometheus/OpenTelemetry 数据模型，可嵌入任何 Go 服务。

```
   Counter ─┐
   Gauge  ──┼─▶ Registry ─▶ 文本/Prometheus 输出
   Histogram┘

   Span（操作）─┬─▶ Trace（调用链）
                └─▶ 树形父子关系（TraceID + ParentID）
```

**做**：3 种 metric（带标签维度 + histogram 桶）+ trace span 树（属性/事件/状态）+ 文本/Prometheus 导出。
**不做**：HTTP 端点（prometheus scrape）、protobuf 导出、采样策略（M2）、分布式上下文传播（跨进程 W3C traceparent）。

## 目标（Goal）

- **G1**：metrics 三种类型（counter/gauge/histogram）完整实现，线程安全，带标签维度。
- **G2**：trace span 树（ParentID 父子关系）+ 属性/事件/状态。
- **G3**：零依赖，可嵌入任何 Go 服务作为库。
- **成功标准**：`go test ./...` 全绿 + demo 跑通 + 线程安全。

## 当前情况（Status）

- **完成度**：**M1 完成**
- **metrics**：Counter/Gauge/Histogram + Registry（7 测试 + 4 测试 = 全绿）
- **trace**：Tracer/Span/Context + 自动注册 + Collect（6 测试全绿）
- **exporter**：文本 + Prometheus 格式
- **cmd**：metrics/trace demo

## 技术栈与架构

- **语言**：Go 1.25.6，零外部依赖
- **设计参考**：Prometheus 数据模型、OpenTelemetry trace 模型

## 如何运行

```bash
go run ./cmd/obs -d metrics   # metrics demo
go run ./cmd/obs -d trace     # trace demo
make test                     # 全部测试
```

## 与其他项目的关系

- **与 [`go-agent-research`](../go-agent-research)**：go-agent-research 内嵌 trace（`internal/trace`），本库是独立的、通用的、产品级可观测性平台。
- **工作区定位**：补"可观测性平台"象限的空白。
