// Package sink 实现 flow-pipe 的 sink 连接器（数据汇）。
// 每个连接器实现 pipeline.SinkConnector，通过 init() 注册到 pipeline.RegisterSink。
//
// 设计：连接器各自一个文件，新增只需实现接口 + init 注册，零改框架。
package sink

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// StdoutSink 把行打印到标准输出。
type StdoutSink struct{}

// Type 返回连接器类型标识。
func (StdoutSink) Type() string { return "stdout" }

// Write 把 rows 打印到 stdout。config:
//
//	format  string  输出格式："json"（默认）/ "table" / "csv"
//
// - json: 每行一条 JSON（MarshalIndent）
// - table: 表头 + 对齐行（简单对齐，键排序保证确定性）
// - csv: 标准 CSV 写入 stdout
func (StdoutSink) Write(rows pipeline.Rows, config map[string]any) error {
	format, _ := config["format"].(string)
	if format == "" {
		format = "json"
	}
	return writeStdout(os.Stdout, rows, format)
}

// writeStdout 抽出便于测试（writer 可注入）。
func writeStdout(w io.Writer, rows pipeline.Rows, format string) error {
	switch strings.ToLower(format) {
	case "", "json":
		return writeJSON(w, rows)
	case "table":
		return writeTable(w, rows)
	case "csv":
		return writeCSV(w, rows)
	default:
		return fmt.Errorf("不支持的 stdout 格式 %q（支持 json/table/csv）", format)
	}
}

func writeJSON(w io.Writer, rows pipeline.Rows) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "[]")
		return err
	}
	for _, r := range rows {
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("json 序列化失败: %w", err)
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// writeTable 简单表格输出：收集所有字段名（排序）做表头。
func writeTable(w io.Writer, rows pipeline.Rows) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "(empty)")
		return err
	}
	headers := collectHeaders(rows)
	sep := " | "
	fmt.Fprintln(w, strings.Join(headers, sep))
	for _, r := range rows {
		vals := make([]string, len(headers))
		for i, h := range headers {
			vals[i] = fmt.Sprint(r[h])
		}
		fmt.Fprintln(w, strings.Join(vals, sep))
	}
	return nil
}

func writeCSV(w io.Writer, rows pipeline.Rows) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if len(rows) == 0 {
		return nil
	}
	headers := collectHeaders(rows)
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, r := range rows {
		rec := make([]string, len(headers))
		for i, h := range headers {
			rec[i] = fmt.Sprint(r[h])
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	return nil
}

// collectHeaders 收集所有行的字段名并按字典序排序（确定性）。
func collectHeaders(rows pipeline.Rows) []string {
	set := map[string]bool{}
	for _, r := range rows {
		for k := range r {
			set[k] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func init() {
	pipeline.RegisterSink(&StdoutSink{})
}
