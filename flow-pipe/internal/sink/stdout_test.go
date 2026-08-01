package sink

import (
	"bytes"
	"strings"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestStdoutSink_JSON(t *testing.T) {
	rows := pipeline.Rows{
		{"id": 1, "name": "alice"},
		{"id": 2, "name": "bob"},
	}
	var buf bytes.Buffer
	if err := writeStdout(&buf, rows, "json"); err != nil {
		t.Fatalf("write 失败: %v", err)
	}
	out := buf.String()
	// 每行一条 JSON，2 行应包含两个独立 JSON 对象
	if !strings.Contains(out, `"id": 1`) || !strings.Contains(out, `"id": 2`) {
		t.Fatalf("json 输出缺少内容: %q", out)
	}
}

func TestStdoutSink_Table(t *testing.T) {
	rows := pipeline.Rows{{"id": 1, "name": "x"}}
	var buf bytes.Buffer
	if err := writeStdout(&buf, rows, "table"); err != nil {
		t.Fatalf("write 失败: %v", err)
	}
	out := buf.String()
	// 表头含字段名（字典序: id, name）
	if !strings.Contains(out, "id") || !strings.Contains(out, "name") {
		t.Fatalf("table 输出缺少表头: %q", out)
	}
}

func TestStdoutSink_CSV(t *testing.T) {
	rows := pipeline.Rows{{"id": 1, "name": "x"}, {"id": 2, "name": "y"}}
	var buf bytes.Buffer
	if err := writeStdout(&buf, rows, "csv"); err != nil {
		t.Fatalf("write 失败: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// 1 表头 + 2 数据行 = 3 行
	if len(lines) != 3 {
		t.Fatalf("csv 应输出 3 行(表头+2 数据)，得到 %d: %q", len(lines), buf.String())
	}
	if lines[0] != "id,name" {
		t.Fatalf("表头应为 id,name，得到 %q", lines[0])
	}
}

func TestStdoutSink_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := writeStdout(&buf, pipeline.Rows{}, "json"); err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if !strings.Contains(buf.String(), "[]") {
		t.Fatalf("空 json 应输出 []: %q", buf.String())
	}
}

func TestStdoutSink_BadFormat(t *testing.T) {
	if err := writeStdout(&bytes.Buffer{}, pipeline.Rows{{"a": 1}}, "xml"); err == nil {
		t.Fatal("不支持的格式应报错")
	}
}

func TestStdoutSink_Registered(t *testing.T) {
	if _, ok := pipeline.GetSink("stdout"); !ok {
		t.Fatal("stdout sink 未注册")
	}
}
