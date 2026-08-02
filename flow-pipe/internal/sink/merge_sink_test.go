package sink

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestMergeSink_WriteSingleBatch(t *testing.T) {
	// 直接 Write：把一批 rows 序列化成一个 JSON 数组
	rows := pipeline.Rows{
		{"id": 1, "name": "alice"},
		{"id": 2, "name": "bob"},
	}
	outPath := filepath.Join(t.TempDir(), "merged.json")
	if err := (MergeSink{}).Write(rows, map[string]any{"path": outPath}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读回文件失败: %v", err)
	}
	// 应能反序列化回长度 2 的数组，且内容一致
	var got []map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("输出不是合法 JSON 数组: %v\n内容: %s", err, data)
	}
	if len(got) != 2 {
		t.Fatalf("应有 2 个元素，实际 %d", len(got))
	}
	if got[0]["id"] != float64(1) || got[1]["name"] != "bob" {
		t.Fatalf("数组内容不符: %v", got)
	}
}

func TestMergeSink_EmptyRows(t *testing.T) {
	// 空输入写出合法的空数组 "[]"
	outPath := filepath.Join(t.TempDir(), "empty.json")
	if err := (MergeSink{}).Write(pipeline.Rows{}, map[string]any{"path": outPath}); err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	data, _ := os.ReadFile(outPath)
	if string(data) != "[]" {
		t.Fatalf("空输入应写出 \"[]\"，实际 %q", string(data))
	}
}

func TestMergeSink_Pretty(t *testing.T) {
	// pretty=true → 带换行缩进；pretty=false（默认）→ 紧凑单行
	rows := pipeline.Rows{{"id": 1}}
	outPath := filepath.Join(t.TempDir(), "pretty.json")
	if err := (MergeSink{}).Write(rows, map[string]any{"path": outPath, "pretty": true}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	data, _ := os.ReadFile(outPath)
	if !containsByte(data, '\n') {
		t.Fatalf("pretty 模式应含换行，实际 %q", string(data))
	}

	outPath2 := filepath.Join(t.TempDir(), "compact.json")
	if err := (MergeSink{}).Write(rows, map[string]any{"path": outPath2}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	data2, _ := os.ReadFile(outPath2)
	if containsByte(data2, '\n') {
		t.Fatalf("默认紧凑模式不应含换行，实际 %q", string(data2))
	}
}

func TestMergeSink_MissingPath(t *testing.T) {
	if err := (MergeSink{}).Write(pipeline.Rows{{"a": 1}}, map[string]any{}); err == nil {
		t.Fatal("缺少 path 应报错")
	}
}

func TestMergeSink_Registered(t *testing.T) {
	if _, ok := pipeline.GetSink("merge"); !ok {
		t.Fatal("merge sink 未注册")
	}
}

// TestMergeSink_MergesMultipleUpstreams 是 merge 的核心端到端验证：
// 三个 source（read1/read2/read3）各产 1 行，merge sink depends_on 三者，
// 运行后输出文件应是一个长度为 3 的 JSON 数组（多上游输出被合并成大数组）。
func TestMergeSink_MergesMultipleUpstreams(t *testing.T) {
	// 注册一次性 mock source：每步的 config["label"] 决定产出行的值，
	// 借此区分三个上游各自贡献的行。RegisterSource 是覆盖语义，无副作用。
	const srcType = "merge_test_src"
	pipeline.RegisterSource(mergeTestSource{})

	outPath := filepath.Join(t.TempDir(), "all.json")
	p := pipeline.Pipeline{
		Name: "merge-e2e",
		Steps: []pipeline.Step{
			{ID: "read1", Kind: pipeline.KindSource, Connector: srcType, Config: map[string]any{"label": "a"}},
			{ID: "read2", Kind: pipeline.KindSource, Connector: srcType, Config: map[string]any{"label": "b"}},
			{ID: "read3", Kind: pipeline.KindSource, Connector: srcType, Config: map[string]any{"label": "c"}},
			{
				ID:        "merge_all",
				Kind:      pipeline.KindSink,
				Connector: "merge",
				DependsOn: []string{"read1", "read2", "read3"},
				Config:    map[string]any{"path": outPath},
			},
		},
	}

	res := pipeline.Run(p)
	if res.Err != nil {
		t.Fatalf("管道运行失败: %v", res.Err)
	}
	// merge_all 步骤应读入 3 行、写出 3 行
	var mergeStep *pipeline.StepResult
	for i := range res.Steps {
		if res.Steps[i].StepID == "merge_all" {
			mergeStep = &res.Steps[i]
		}
	}
	if mergeStep == nil {
		t.Fatal("找不到 merge_all 步骤结果")
	}
	if mergeStep.RowsIn != 3 || mergeStep.RowsOut != 3 {
		t.Fatalf("merge_all 应读入/写出 3 行，实际 RowsIn=%d RowsOut=%d", mergeStep.RowsIn, mergeStep.RowsOut)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读回合并文件失败: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("输出不是合法 JSON 数组: %v\n内容: %s", err, data)
	}
	if len(got) != 3 {
		t.Fatalf("合并后应是大数组（长度 3），实际 %d", len(got))
	}
	// depends_on 顺序：a,b,c 应按声明顺序出现
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if got[i]["v"] != w {
			t.Errorf("合并数组第 %d 个元素 v=%v，期望 %q（depends_on 顺序）", i, got[i]["v"], w)
		}
	}
}

// mergeTestSource 产出 1 行 {v: <label>}，供端到端测试区分各上游。
type mergeTestSource struct{}

func (mergeTestSource) Type() string { return "merge_test_src" }
func (mergeTestSource) Read(config map[string]any) (pipeline.Rows, error) {
	label, _ := config["label"].(string)
	return pipeline.Rows{{"v": label}}, nil
}

// containsByte 报告 data 是否含字节 b。
func containsByte(data []byte, b byte) bool {
	for _, c := range data {
		if c == b {
			return true
		}
	}
	return false
}
