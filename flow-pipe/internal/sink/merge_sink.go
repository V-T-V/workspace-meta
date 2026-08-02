package sink

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// MergeSink 把多个上游步骤的输出合并成一个大数组。
//
// 与 stdout/csv 不同：merge 把 runner 已经按 depends_on 顺序拼接好的输入 Rows，
// 整体序列化成一个 JSON 数组写到文件。语义对齐任务描述的"合并多个上游输出成大数组"：
//
//   - id: merge_all
//     type: sink
//     connector: merge
//     depends_on: [read1, read2, read3]
//
// runner 在执行 sink 前会把 read1/read2/read3 三者的输出（各自一批 Row）按
// depends_on 声明顺序拼接成单个 input Rows（见 runner.go 的 input 收集逻辑），
// 再交给本 sink。所以 merge 只需把收到的 Rows 整体写成一个 JSON 数组即可。
type MergeSink struct{}

// Type 返回连接器类型标识。
func (MergeSink) Type() string { return "merge" }

// Write 把 rows 合并成一个大 JSON 数组写到文件。config:
//
//	path    string  输出文件路径（必填）
//	pretty  bool    是否美化输出（缩进），默认 false（紧凑单行 JSON 数组）
//
// 空输入写出一个合法的空数组 "[]"。
func (MergeSink) Write(rows pipeline.Rows, config map[string]any) error {
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("merge sink 缺少 path 配置")
	}
	pretty, _ := config["pretty"].(bool)

	// 直接把整批 rows 作为一个 JSON 数组序列化（[]map → [{...},{...}]）。
	var (
		data []byte
		err  error
	)
	if pretty {
		data, err = json.MarshalIndent(rows, "", "  ")
	} else {
		data, err = json.Marshal(rows)
	}
	if err != nil {
		return fmt.Errorf("merge 序列化失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("merge 写文件失败: %w", err)
	}
	return nil
}

func init() {
	pipeline.RegisterSink(&MergeSink{})
}
