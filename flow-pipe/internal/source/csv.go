package source

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// CSVSource 从 CSV 文件读取数据。
// 第一行作为 header（字段名），后续每行按 header 映射成 Row，值统一为 string。
type CSVSource struct{}

// Type 返回连接器类型标识。
func (CSVSource) Type() string { return "csv" }

// Read 根据 config 读 CSV。config:
//
//	path       string  CSV 文件路径（必填）
//	delimiter  string  分隔符（默认 ","），取首字符
//
// 示例: {path: "data.csv", delimiter: ","}
func (CSVSource) Read(config map[string]any) (pipeline.Rows, error) {
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("csv source 缺少 path 配置")
	}
	delim := ","
	if d, ok := config["delimiter"].(string); ok && d != "" {
		delim = d
	}
	skipRows := 0
	if sr, ok := config["skip_rows"]; ok {
		switch v := sr.(type) {
		case int:
			skipRows = v
		case int64:
			skipRows = int(v)
		case float64:
			skipRows = int(v)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 CSV 文件失败: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	if len(delim) > 0 {
		r.Comma = rune(delim[0])
	}
	// 字段数不一致也读（容忍尾随空行等），交给下面逻辑处理
	r.FieldsPerRecord = -1

	all, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析 CSV 失败: %w", err)
	}
	if len(all) == 0 {
		return pipeline.Rows{}, nil
	}
	// skip_rows：跳过前 N 行（如注释行）
	if skipRows > 0 && skipRows < len(all) {
		all = all[skipRows:]
	} else if skipRows >= len(all) {
		return pipeline.Rows{}, nil
	}

	// header：默认第一行作为列名。若 config 提供 header（[]any），则用它作为列名，
	// 且所有数据行都被解析（不跳过第一行）。适合无标题行的 CSV。
	var header []string
	dataStart := 1 // 默认跳过第一行（标题行）
	if headerCfg, ok := config["header"].([]any); ok && len(headerCfg) > 0 {
		for _, h := range headerCfg {
			header = append(header, fmt.Sprintf("%v", h))
		}
		dataStart = 0 // 有自定义 header 时不跳过第一行
	} else {
		header = all[0]
	}

	rows := make(pipeline.Rows, 0, len(all)-dataStart)
	for _, rec := range all[dataStart:] {
		// 跳过全空行（csv.Reader 一般不会产生，但稳妥起见）
		if len(rec) == 0 {
			continue
		}
		row := pipeline.Row{}
		for i, name := range header {
			if i >= len(rec) {
				row[name] = ""
				continue
			}
			row[name] = strings.TrimSpace(rec[i])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func init() {
	pipeline.RegisterSource(&CSVSource{})
}
