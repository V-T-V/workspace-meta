package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// writeTemp 写临时文件并返回路径。
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("写临时文件失败: %v", err)
	}
	return p
}

func TestCSVSource_Read(t *testing.T) {
	p := writeTemp(t, "data.csv", "id,name,amount\n1,alice,100\n2,bob,200\n")
	src := CSVSource{}
	if src.Type() != "csv" {
		t.Fatalf("Type 应为 csv，得到 %q", src.Type())
	}
	rows, err := src.Read(map[string]any{"path": p, "delimiter": ","})
	if err != nil {
		t.Fatalf("Read 失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("应为 2 行，得到 %d", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["name"] != "alice" || rows[0]["amount"] != "100" {
		t.Fatalf("首行解析错误: %#v", rows[0])
	}
	if rows[1]["name"] != "bob" {
		t.Fatalf("次行 name 错误: %#v", rows[1])
	}
}

func TestCSVSource_DefaultDelimiter(t *testing.T) {
	p := writeTemp(t, "d.csv", "a,b\nx,y\n")
	rows, err := CSVSource{}.Read(map[string]any{"path": p})
	if err != nil {
		t.Fatalf("Read 失败: %v", err)
	}
	if len(rows) != 1 || rows[0]["a"] != "x" || rows[0]["b"] != "y" {
		t.Fatalf("默认分隔符解析错误: %#v", rows)
	}
}

func TestCSVSource_MissingPath(t *testing.T) {
	if _, err := (CSVSource{}).Read(map[string]any{}); err == nil {
		t.Fatal("缺少 path 应报错")
	}
}

func TestCSVSource_EmptyFile(t *testing.T) {
	p := writeTemp(t, "empty.csv", "")
	rows, err := CSVSource{}.Read(map[string]any{"path": p})
	if err != nil {
		t.Fatalf("空文件不应报错: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("空文件应返回 0 行，得到 %d", len(rows))
	}
}

func TestCSVSource_Registered(t *testing.T) {
	c, ok := pipeline.GetSource("csv")
	if !ok {
		t.Fatal("csv source 未注册到 pipeline")
	}
	if c.Type() != "csv" {
		t.Fatalf("注册的 Type 错误: %q", c.Type())
	}
}
