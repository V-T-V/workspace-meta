package sink

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// CSVSink 把行写入 CSV 文件。
type CSVSink struct{}

// Type 返回连接器类型标识。
func (CSVSink) Type() string { return "csv" }

// Write 把 rows 写到 CSV 文件。config:
//
//	path       string  输出文件路径（必填）
//	delimiter  string  分隔符（默认 ","），取首字符
//
// header 取所有行字段名的并集（字典序排序保证确定性），缺失字段写空串。
func (CSVSink) Write(rows pipeline.Rows, config map[string]any) error {
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("csv sink 缺少 path 配置")
	}
	delim := ","
	if d, ok := config["delimiter"].(string); ok && d != "" {
		delim = d
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建 CSV 文件失败: %w", err)
	}

	w := csv.NewWriter(f)
	if len(delim) > 0 {
		w.Comma = rune(delim[0])
	}
	// 先 flush 后 close：defer 保证任何 return 路径都先 flush 再 close，
	// 避免 Write 失败提前返回时 buffer 数据丢失。
	defer func() {
		w.Flush()
		_ = f.Close()
	}()

	if len(rows) == 0 {
		return nil // 空输入 → 写空文件
	}

	headers := collectHeaders(rows)
	if err := w.Write(headers); err != nil {
		return fmt.Errorf("写 CSV header 失败: %w", err)
	}
	for _, r := range rows {
		rec := make([]string, len(headers))
		for i, h := range headers {
			rec[i] = fmt.Sprint(r[h])
		}
		if err := w.Write(rec); err != nil {
			return fmt.Errorf("写 CSV 行失败: %w", err)
		}
	}
	return nil
}

// header 取所有行字段名的并集（复用同包 stdout.go 的 collectHeaders，字典序排序保证确定性）。

func init() {
	pipeline.RegisterSink(&CSVSink{})
}
