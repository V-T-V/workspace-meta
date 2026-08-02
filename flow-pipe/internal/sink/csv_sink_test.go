package sink

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestCSVSink_Write(t *testing.T) {
	rows := pipeline.Rows{
		{"id": 1, "name": "alice"},
		{"id": 2, "name": "bob"},
	}
	outPath := filepath.Join(t.TempDir(), "out.csv")
	if err := (CSVSink{}).Write(rows, map[string]any{"path": outPath}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读回文件失败: %v", err)
	}
	content := string(data)
	// 表头（字段排序: id,name）+ 2 行
	wantLines := []string{"id,name", "1,alice", "2,bob"}
	for _, l := range wantLines {
		if !containsLine(content, l) {
			t.Fatalf("输出缺少行 %q，完整内容:\n%s", l, content)
		}
	}
}

func TestCSVSink_MissingPath(t *testing.T) {
	if err := (CSVSink{}).Write(pipeline.Rows{{"a": 1}}, map[string]any{}); err == nil {
		t.Fatal("缺少 path 应报错")
	}
}

func TestCSVSink_EmptyRows(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "empty.csv")
	if err := (CSVSink{}).Write(pipeline.Rows{}, map[string]any{"path": outPath}); err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	info, _ := os.Stat(outPath)
	if info == nil {
		t.Fatal("空输入也应创建文件")
	}
}

func TestCSVSink_SparseRows(t *testing.T) {
	// 不同行字段不同 → header 取并集，缺失写空
	rows := pipeline.Rows{
		{"id": 1, "name": "x"},
		{"id": 2, "extra": "y"},
	}
	outPath := filepath.Join(t.TempDir(), "sparse.csv")
	if err := (CSVSink{}).Write(rows, map[string]any{"path": outPath}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	data, _ := os.ReadFile(outPath)
	// collectHeaders 按字典序排序：extra < id < name
	if !containsLine(string(data), "extra,id,name") {
		t.Fatalf("表头应为并集排序 extra,id,name，内容:\n%s", string(data))
	}
}

func TestCSVSink_Registered(t *testing.T) {
	if _, ok := pipeline.GetSink("csv"); !ok {
		t.Fatal("csv sink 未注册")
	}
}

// containsLine 简单按行匹配。
func containsLine(content, line string) bool {
	for _, l := range splitLines(content) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		if r == '\r' {
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
