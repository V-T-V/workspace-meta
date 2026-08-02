package pipeline

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// errWriteFailed 是 failSink 模拟写失败用的哨兵错误。
var errWriteFailed = errors.New("模拟写入失败")

// 本文件测试管道状态恢复：失败的管道能从上次成功的步骤继续，而非从头重跑。
//
// 核心能力：
//   - WithSkipSteps / RunWithOptions：跳过指定步骤（标记 Skipped=true），不执行连接器
//   - 被跳过步骤的输出用空 Rows 代替（partial rerun），下游步骤仍正常执行

// ===== 测试用连接器 =====

// passThroughTransform 把输入原样透传，并记录调用次数（验证是否被跳过）。
type passThroughTransform struct {
	mu       sync.Mutex
	calls    int
	records  []string // 记录每次被调用时收到的行数（调试用）
	lastRows int
}

var passThrough = &passThroughTransform{}

func (p *passThroughTransform) Type() string { return "passthrough" }
func (p *passThroughTransform) Transform(rows Rows, config map[string]any) (Rows, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.lastRows = len(rows)
	p.records = append(p.records, "called")
	return rows, nil
}

// countingSink 记录被写入的次数和总行数。
type countingSink struct {
	mu        sync.Mutex
	writes    int
	totalRows int
}

var counter = &countingSink{}

func (c *countingSink) Type() string { return "countingsink" }
func (c *countingSink) Write(rows Rows, config map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes++
	c.totalRows += len(rows)
	return nil
}

func init() {
	RegisterTransform(passThrough)
	RegisterSink(counter)
}

// resetCounters 把测试用的计数器清零（各测试互不影响）。
func resetCounters() {
	passThrough.mu.Lock()
	passThrough.calls = 0
	passThrough.records = nil
	passThrough.lastRows = 0
	passThrough.mu.Unlock()

	counter.mu.Lock()
	counter.writes = 0
	counter.totalRows = 0
	counter.mu.Unlock()
}

// linearPipeline 构造 read → clean → write 的 3 步线性管道。
func linearPipeline() Pipeline {
	return Pipeline{Name: "recover-test", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "mocksource"},
		{ID: "clean", Kind: KindTransform, Connector: "passthrough", DependsOn: []string{"read"}},
		{ID: "write", Kind: KindSink, Connector: "countingsink", DependsOn: []string{"clean"}},
	}}
}

// findStep 在结果里按 ID 查找单步结果（找不到则 t.Fatal）。
func findStep(t *testing.T, result *RunResult, id string) StepResult {
	t.Helper()
	for _, s := range result.Steps {
		if s.StepID == id {
			return s
		}
	}
	t.Fatalf("未找到步骤 %s", id)
	return StepResult{}
}

// TestRunWithSkipSteps 跳过第 1 步（read）后，read 标记 Skipped=true 且不执行，
// clean/write 仍正常执行。验证下游步骤拿到空输入但仍被调用。
func TestRunWithSkipSteps(t *testing.T) {
	resetCounters()
	p := linearPipeline()

	result := RunWithOptions(p, WithSkipSteps([]string{"read"}))
	if result.Err != nil {
		t.Fatalf("RunWithOptions 不应失败: %v", result.Err)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("应有 3 步结果，实际 %d", len(result.Steps))
	}

	// 第 1 步 read：被跳过
	readStep := findStep(t, result, "read")
	if !readStep.Skipped {
		t.Errorf("read 应 Skipped=true")
	}
	if readStep.RowsOut != 0 {
		t.Errorf("被跳过的 read 不应产出数据，RowsOut=%d", readStep.RowsOut)
	}

	// clean 仍执行（拿到空输入），write 仍执行
	cleanStep := findStep(t, result, "clean")
	if cleanStep.Skipped {
		t.Errorf("clean 不应被跳过")
	}
	if passThrough.calls != 1 {
		t.Errorf("clean 的 transform 应被调用 1 次，实际 %d", passThrough.calls)
	}
	// read 被跳过 → clean 输入为空（partial rerun）
	if cleanStep.RowsIn != 0 {
		t.Errorf("clean 输入应为空（上游被跳过），实际 %d", cleanStep.RowsIn)
	}

	writeStep := findStep(t, result, "write")
	if writeStep.Skipped {
		t.Errorf("write 不应被跳过")
	}
	if counter.writes != 1 {
		t.Errorf("write 的 sink 应被调用 1 次，实际 %d", counter.writes)
	}
}

// TestRunWithSkipMultipleSteps 跳过多步：read 与 clean 都跳过，只剩 write 执行。
func TestRunWithSkipMultipleSteps(t *testing.T) {
	resetCounters()
	p := linearPipeline()

	result := RunWithOptions(p, WithSkipSteps([]string{"read", "clean"}))
	if result.Err != nil {
		t.Fatalf("RunWithOptions 不应失败: %v", result.Err)
	}
	if findStep(t, result, "read").Skipped != true {
		t.Error("read 应被跳过")
	}
	if findStep(t, result, "clean").Skipped != true {
		t.Error("clean 应被跳过")
	}
	if findStep(t, result, "write").Skipped != false {
		t.Error("write 不应被跳过")
	}
	// clean 被跳过 → 其 transform 不应被调用
	if passThrough.calls != 0 {
		t.Errorf("clean 被跳过时 transform 不应执行，实际调用 %d 次", passThrough.calls)
	}
	// write 仍执行（拿到空输入）
	if counter.writes != 1 {
		t.Errorf("write 仍应执行 1 次，实际 %d", counter.writes)
	}
}

// TestRunEquivalentToRunWithOptions 不传任何 opt 时，RunWithOptions 应与 Run 行为一致。
func TestRunEquivalentToRunWithOptions(t *testing.T) {
	resetCounters()
	p := linearPipeline()

	r1 := Run(p)
	resetCounters()
	r2 := RunWithOptions(p)

	if r1.Err != nil || r2.Err != nil {
		t.Fatalf("都不应失败: r1=%v r2=%v", r1.Err, r2.Err)
	}
	if len(r1.Steps) != len(r2.Steps) {
		t.Fatalf("步骤数不一致: %d vs %d", len(r1.Steps), len(r2.Steps))
	}
	// 都不应有任何步骤被跳过
	for i, s := range r1.Steps {
		if s.Skipped || r2.Steps[i].Skipped {
			t.Errorf("无 opt 时不应有跳过的步骤")
		}
	}
}

// TestRecoverFromHistory 模拟真实恢复场景：
//  1. 第 1 次运行：read 成功、clean 成功、write 失败（模拟写入崩溃）。
//     历史记录里 read/clean 是"已成功完成的步骤"。
//  2. 第 2 次运行（恢复）：用 WithSkipSteps 跳过 read/clean，
//     验证它们 Skipped=true，且 write 这次成功。
//
// 这里直接用 WithSkipSteps 模拟"查询历史得到 read/clean"，
// 避免在 pipeline 包内 import storage（防循环依赖）。
func TestRecoverFromHistory(t *testing.T) {
	resetCounters()

	// ---- 第 1 次运行：write 失败 ----
	// 用一个会失败的 sink 模拟崩溃。
	failingSink := &failSink{failN: 1} // 第 1 次写失败
	RegisterSink(failingSink)

	p1 := Pipeline{Name: "hist", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "mocksource"},
		{ID: "clean", Kind: KindTransform, Connector: "passthrough", DependsOn: []string{"read"}},
		{ID: "write", Kind: KindSink, Connector: "failsink", DependsOn: []string{"clean"}},
	}}
	first := Run(p1)
	if first.Err == nil {
		t.Fatal("第 1 次运行 write 应失败")
	}
	// read/clean 应成功
	if findStep(t, first, "read").Err != nil || findStep(t, first, "clean").Err != nil {
		t.Fatal("read/clean 第 1 次应成功")
	}
	// write 失败 → 整体失败
	if findStep(t, first, "write").Err == nil {
		t.Fatal("write 第 1 次应失败")
	}

	// ---- 模拟从历史查出 read/clean 已成功 ----
	skipIDs := []string{}
	for _, s := range first.Steps {
		if !s.Skipped && s.Err == nil {
			skipIDs = append(skipIDs, s.StepID)
		}
	}
	if len(skipIDs) != 2 {
		t.Fatalf("应查出 2 个成功步骤（read,clean），实际 %v", skipIDs)
	}

	// ---- 第 2 次运行：恢复，跳过 read/clean，write 这次成功 ----
	resetCounters()
	// 失败计数器已耗尽（failN=1，第 1 次已用掉），改用成功 sink。
	// 直接换一个不失败的 sink 名（mocksink，来自 pipeline_test.go）。
	p2 := Pipeline{Name: "hist", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "mocksource"},
		{ID: "clean", Kind: KindTransform, Connector: "passthrough", DependsOn: []string{"read"}},
		{ID: "write", Kind: KindSink, Connector: "mocksink", DependsOn: []string{"clean"}},
	}}
	recovered := RunWithOptions(p2, WithSkipSteps(skipIDs))

	if recovered.Err != nil {
		t.Fatalf("恢复运行不应失败: %v", recovered.Err)
	}
	// read/clean 被跳过
	if !findStep(t, recovered, "read").Skipped {
		t.Error("恢复时 read 应 Skipped=true")
	}
	if !findStep(t, recovered, "clean").Skipped {
		t.Error("恢复时 clean 应 Skipped=true")
	}
	// write 这次成功执行（未被跳过、无错）
	w := findStep(t, recovered, "write")
	if w.Skipped {
		t.Error("write 不应被跳过")
	}
	if w.Err != nil {
		t.Errorf("write 恢复后应成功: %v", w.Err)
	}
	// clean 被跳过 → transform 不应被调用
	if passThrough.calls != 0 {
		t.Errorf("clean 被跳过时 transform 不应执行，实际 %d 次", passThrough.calls)
	}
}

// TestSummaryMentionsSkipped 验证 Summary 文本在状态恢复时提及跳过的步骤。
func TestSummaryMentionsSkipped(t *testing.T) {
	resetCounters()
	p := linearPipeline()
	result := RunWithOptions(p, WithSkipSteps([]string{"read"}))
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	summary := result.Summary()
	if !contains(summary, "跳过") {
		t.Errorf("Summary 应提及跳过步骤，实际: %q", summary)
	}
}

// TestSkippedStepHasNearZeroDuration 被跳过的步骤不应消耗显著时间。
func TestSkippedStepHasNearZeroDuration(t *testing.T) {
	resetCounters()
	p := linearPipeline()
	result := RunWithOptions(p, WithSkipSteps([]string{"clean"}))
	cleanStep := findStep(t, result, "clean")
	if cleanStep.Duration > 10*time.Millisecond {
		t.Errorf("被跳过步骤耗时异常: %s", cleanStep.Duration)
	}
}

// failSink 第 failN 次写入以内都失败，之后成功。
type failSink struct{ failN int }

var failCount int

func (f *failSink) Type() string { return "failsink" }
func (f *failSink) Write(rows Rows, config map[string]any) error {
	failCount++
	if failCount <= f.failN {
		return errWriteFailed
	}
	return nil
}

// contains 是简易子串判断（避免引 strings 增加依赖噪音）。
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
