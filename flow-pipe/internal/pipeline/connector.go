// Package pipeline 实现 flow-pipe 的核心：DAG 管道编排 + 可插拔连接器。
//
// 连接器分三类：source（数据源）/ transform（变换）/ sink（数据汇）。
// 每类连接器实现对应接口，通过 Register 注册到全局 Registry。
// YAML 里 type: csv / filter / sqlite 等按名查找，新增连接器零改框架。
//
// 设计对齐 generic-admin/internal/export 的 Interface + Registry 范式。
package pipeline

// Row 是管道里流动的一条记录。键值对形式（类似 JSON object）。
// 各连接器读写 Row：source 产出 Row，transform 改 Row，sink 消费 Row。
type Row map[string]any

// Rows 是一批 Row（连接器间传递的基本单位）。
type Rows []Row

// ===== 连接器接口 =====

// SourceConnector 数据源连接器：从外部系统读数据，产出一批 Row。
// 示例：csv（读 CSV 文件）/ json / generate（造测试数据）/ http / sqlite。
type SourceConnector interface {
	// Type 返回连接器类型标识（对应 YAML 里的 connector: csv）。
	Type() string
	// Read 根据 config 读数据，返回 Row 切片。
	Read(config map[string]any) (Rows, error)
}

// TransformConnector 变换连接器：对输入 Row 做过滤/映射/聚合，输出新 Row。
// 示例：filter（按条件过滤）/ map（字段映射）/ field（增删字段）/ aggregate。
type TransformConnector interface {
	Type() string
	// Transform 处理输入 rows，输出处理后的 rows。
	Transform(rows Rows, config map[string]any) (Rows, error)
}

// SinkConnector 数据汇连接器：把 Row 写到外部系统。
// 示例：stdout / csv / sqlite / http。
type SinkConnector interface {
	Type() string
	// Write 把 rows 写到目标。
	Write(rows Rows, config map[string]any) error
}

// ===== 注册表 =====
//
// 三类连接器各有独立注册表（避免同名冲突，如 "csv" 既是 source 又是 sink）。

var (
	sources    = map[string]SourceConnector{}
	transforms = map[string]TransformConnector{}
	sinks      = map[string]SinkConnector{}
)

// RegisterSource 注册一个 source 连接器。
func RegisterSource(c SourceConnector) { sources[c.Type()] = c }

// RegisterTransform 注册一个 transform 连接器。
func RegisterTransform(c TransformConnector) { transforms[c.Type()] = c }

// RegisterSink 注册一个 sink 连接器。
func RegisterSink(c SinkConnector) { sinks[c.Type()] = c }

// GetSource 按类型名取 source 连接器。
func GetSource(name string) (SourceConnector, bool) { c, ok := sources[name]; return c, ok }

// GetTransform 按类型名取 transform 连接器。
func GetTransform(name string) (TransformConnector, bool) { c, ok := transforms[name]; return c, ok }

// GetSink 按类型名取 sink 连接器。
func GetSink(name string) (SinkConnector, bool) { c, ok := sinks[name]; return c, ok }

// ListSources 返回所有已注册的 source 类型名。
func ListSources() []string { return keys(sources) }

// ListTransforms 返回所有已注册的 transform 类型名。
func ListTransforms() []string { return keys(transforms) }

// ListSinks 返回所有已注册的 sink 类型名。
func ListSinks() []string { return keys(sinks) }

func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
