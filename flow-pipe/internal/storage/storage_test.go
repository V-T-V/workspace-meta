package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// testDB 是当前测试在用的 *sql.DB（由 newTestDB 设置，close 时置 nil）。
var testDB *sql.DB

// newTestDB 开一个临时 SQLite 库（跑完 migration）。返回 close 函数。
func newTestDB(t *testing.T) (closeFn func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	// Open 内部已建好表 + 跑完 migration。把 db 存到测试包级变量供后续用。
	var err error
	testDB, err = Open(dbPath)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	return func() {
		_ = testDB.Close()
		testDB = nil
	}
}

func TestOpenMigratesTables(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	// 校验 migrationFS 有 001。
	names, err := fsNames()
	if err != nil {
		t.Fatalf("fsNames 失败: %v", err)
	}
	if len(names) == 0 || names[0] != "001_init.sql" {
		t.Errorf("期望首个 migration 为 001_init.sql, 得到 %v", names)
	}
}

func TestPipelineRepoSaveGetAll(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	// Save（新插入）
	id, err := SavePipeline(testDB, "demo", "name: demo\nsteps: []")
	if err != nil {
		t.Fatalf("SavePipeline 失败: %v", err)
	}
	if id == 0 {
		t.Errorf("返回 id 不应为 0")
	}

	// Get
	p, err := GetPipeline(testDB, "demo")
	if err != nil {
		t.Fatalf("GetPipeline 失败: %v", err)
	}
	if p == nil {
		t.Fatalf("GetPipeline 返回 nil")
	}
	if p.Name != "demo" || p.DefinitionYAML != "name: demo\nsteps: []" {
		t.Errorf("读回的管道内容不对: %+v", p)
	}

	// Save 同名（覆盖）
	if _, err := SavePipeline(testDB, "demo", "name: demo\nsteps: [updated]"); err != nil {
		t.Fatalf("覆盖保存失败: %v", err)
	}
	p2, _ := GetPipeline(testDB, "demo")
	if p2.DefinitionYAML != "name: demo\nsteps: [updated]" {
		t.Errorf("覆盖后内容不对: %q", p2.DefinitionYAML)
	}

	// All
	if _, err := SavePipeline(testDB, "alpha", "name: alpha"); err != nil {
		t.Fatal(err)
	}
	all, err := AllPipelines(testDB)
	if err != nil {
		t.Fatalf("AllPipelines 失败: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("AllPipelines 返回 %d 条, 想要 2", len(all))
	}
	// 按 name 升序：alpha 在前
	if all[0].Name != "alpha" {
		t.Errorf("AllPipelines 排序不对, 首个=%q", all[0].Name)
	}

	// Get 不存在
	missing, err := GetPipeline(testDB, "nope")
	if err != nil {
		t.Fatalf("GetPipeline 不存在时不应返回错误: %v", err)
	}
	if missing != nil {
		t.Errorf("不存在的管道应返回 nil")
	}
}

func TestSaveRunAndAllRuns(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	// 先存一个管道，让 run 能关联 pipeline_id。
	pid, _ := SavePipeline(testDB, "demo", "name: demo")

	rr := &pipeline.RunResult{
		PipelineName: "demo",
		StartedAt:    time.Now().Add(-2 * time.Second).UTC(),
		FinishedAt:   time.Now().UTC(),
		Steps: []pipeline.StepResult{
			{StepID: "read", Kind: pipeline.KindSource, RowsOut: 3, Duration: 5 * time.Millisecond},
			{StepID: "write", Kind: pipeline.KindSink, RowsIn: 3, RowsOut: 3, Duration: 2 * time.Millisecond},
		},
	}

	runID, err := SaveRun(testDB, rr)
	if err != nil {
		t.Fatalf("SaveRun 失败: %v", err)
	}
	if runID == 0 {
		t.Errorf("runID 不应为 0")
	}

	runs, err := AllRuns(testDB, 10)
	if err != nil {
		t.Fatalf("AllRuns 失败: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("AllRuns 返回 %d 条, 想要 1", len(runs))
	}
	got := runs[0]
	if got.PipelineName != "demo" {
		t.Errorf("pipeline_name = %q", got.PipelineName)
	}
	if got.Status != "ok" {
		t.Errorf("status = %q, 想要 ok", got.Status)
	}
	if !got.PipelineID.Valid || got.PipelineID.Int64 != pid {
		t.Errorf("pipeline_id 应关联到 %d, 得到 %+v", pid, got.PipelineID)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("反序列化 steps 数 = %d, 想要 2", len(got.Steps))
	}
	if got.Steps[0].StepID != "read" || got.Steps[1].RowsIn != 3 {
		t.Errorf("steps 内容不对: %+v", got.Steps)
	}

	// 失败的 run（带 error）
	errRR := &pipeline.RunResult{
		PipelineName: "demo",
		StartedAt:    time.Now().UTC(),
		FinishedAt:   time.Now().UTC(),
		Err:          &runError{msg: "boom"},
	}
	if _, err := SaveRun(testDB, errRR); err != nil {
		t.Fatalf("SaveRun 失败 run 失败: %v", err)
	}
	runs2, _ := AllRuns(testDB, 10)
	if len(runs2) != 2 {
		t.Fatalf("应有 2 条 run, 得到 %d", len(runs2))
	}
	// 倒序：最新的（失败的）在前
	if runs2[0].Status != "error" || runs2[0].Err != "boom" {
		t.Errorf("失败 run 读取不对: status=%q err=%q", runs2[0].Status, runs2[0].Err)
	}

	// limit=0 走默认值
	if _, err := AllRuns(testDB, 0); err != nil {
		t.Errorf("limit=0 应走默认值: %v", err)
	}
}

// runError 是测试用的 error 类型（避免在 test 里引 fmt）。
type runError struct{ msg string }

func (e *runError) Error() string { return e.msg }

// TestLatestSuccessfulSteps 验证状态恢复用的"最近成功步骤"查询。
// 场景：read→clean→write，第 1 次 write 失败，read/clean 成功。
// 查询应返回最近那条 run 里所有成功的步骤（read, clean）。
func TestLatestSuccessfulSteps(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	// 第 1 次运行：read/clean 成功，write 失败。
	rr1 := &pipeline.RunResult{
		PipelineName: "demo",
		StartedAt:    time.Now().Add(-2 * time.Second).UTC(),
		FinishedAt:   time.Now().Add(-1 * time.Second).UTC(),
		Steps: []pipeline.StepResult{
			{StepID: "read", Kind: pipeline.KindSource, RowsOut: 3, Duration: 1 * time.Millisecond},
			{StepID: "clean", Kind: pipeline.KindTransform, RowsIn: 3, RowsOut: 3, Duration: 1 * time.Millisecond},
			{StepID: "write", Kind: pipeline.KindSink, RowsIn: 3, Err: &runError{"write boom"}, Duration: 1 * time.Millisecond},
		},
		Err: &runError{"write boom"},
	}
	if _, err := SaveRun(testDB, rr1); err != nil {
		t.Fatalf("SaveRun 失败: %v", err)
	}

	got, err := LatestSuccessfulSteps(testDB, "demo")
	if err != nil {
		t.Fatalf("LatestSuccessfulSteps 失败: %v", err)
	}
	// 应包含 read 与 clean（按保存顺序），不含 write。
	if len(got) != 2 {
		t.Fatalf("想要 2 个成功步骤，实际 %v", got)
	}
	if got[0] != "read" || got[1] != "clean" {
		t.Errorf("成功步骤顺序不对，实际 %v", got)
	}
}

// TestLatestSuccessfulStepsPicksLatestRun 多次运行时，取最近一次"有任何成功步骤"的 run。
// 最近一次某步骤失败时，应回退到更早的成功 run（状态恢复的核心：从上次成功的地方继续）。
func TestLatestSuccessfulStepsPicksLatestRun(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	// 第 1 次（旧）：read 成功。
	old := &pipeline.RunResult{
		PipelineName: "p",
		StartedAt:    time.Now().Add(-2 * time.Hour).UTC(),
		FinishedAt:   time.Now().Add(-2 * time.Hour).UTC(),
		Steps: []pipeline.StepResult{
			{StepID: "read", Kind: pipeline.KindSource, RowsOut: 5},
		},
	}
	if _, err := SaveRun(testDB, old); err != nil {
		t.Fatal(err)
	}
	// 第 2 次（最近）：read 失败，但 clean 成功（说明这次跑到更远了）。
	recent := &pipeline.RunResult{
		PipelineName: "p",
		StartedAt:    time.Now().UTC(),
		FinishedAt:   time.Now().UTC(),
		Steps: []pipeline.StepResult{
			{StepID: "read", Kind: pipeline.KindSource, Err: &runError{"boom"}},
			{StepID: "clean", Kind: pipeline.KindTransform, RowsIn: 0, RowsOut: 0},
		},
		Err: &runError{"boom"},
	}
	if _, err := SaveRun(testDB, recent); err != nil {
		t.Fatal(err)
	}

	got, err := LatestSuccessfulSteps(testDB, "p")
	if err != nil {
		t.Fatalf("LatestSuccessfulSteps 失败: %v", err)
	}
	// 最近一次有成功步骤（clean）→ 取它的成功步骤集 [clean]，而非回退到旧 run 的 [read]。
	if len(got) != 1 || got[0] != "clean" {
		t.Errorf("应取最近一次有成功步骤的 run → [clean]，实际 %v", got)
	}
}

// TestLatestSuccessfulStepsAllFail 所有 run 都无成功步骤时返回空（无可恢复状态）。
func TestLatestSuccessfulStepsAllFail(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	rr := &pipeline.RunResult{
		PipelineName: "p",
		StartedAt:    time.Now().UTC(),
		FinishedAt:   time.Now().UTC(),
		Steps: []pipeline.StepResult{
			{StepID: "read", Kind: pipeline.KindSource, Err: &runError{"boom"}},
		},
		Err: &runError{"boom"},
	}
	if _, err := SaveRun(testDB, rr); err != nil {
		t.Fatal(err)
	}
	got, err := LatestSuccessfulSteps(testDB, "p")
	if err != nil {
		t.Fatalf("LatestSuccessfulSteps 失败: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("全部失败时应返回空，实际 %v", got)
	}
}

// TestLatestSuccessfulStepsEmpty 空管道名 / 不存在的管道应返回 nil 无错。
func TestLatestSuccessfulStepsEmpty(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	if got, err := LatestSuccessfulSteps(testDB, ""); err != nil || got != nil {
		t.Errorf("空 pipelineName 应返回 (nil,nil)，得到 (%v,%v)", got, err)
	}
	if got, err := LatestSuccessfulSteps(testDB, "ghost"); err != nil || len(got) != 0 {
		t.Errorf("不存在的管道应返回空切片无错，得到 (%v,%v)", got, err)
	}
}

// TestSkippedRoundTrip 验证 Skipped 字段经 SaveRun→AllRuns 能往返保留。
func TestSkippedRoundTrip(t *testing.T) {
	closeFn := newTestDB(t)
	defer closeFn()

	rr := &pipeline.RunResult{
		PipelineName: "sk",
		StartedAt:    time.Now().UTC(),
		FinishedAt:   time.Now().UTC(),
		Steps: []pipeline.StepResult{
			{StepID: "read", Kind: pipeline.KindSource, Skipped: true},
			{StepID: "write", Kind: pipeline.KindSink, RowsIn: 0, RowsOut: 0},
		},
	}
	if _, err := SaveRun(testDB, rr); err != nil {
		t.Fatalf("SaveRun 失败: %v", err)
	}
	runs, err := AllRuns(testDB, 5)
	if err != nil {
		t.Fatalf("AllRuns 失败: %v", err)
	}
	if len(runs) != 1 || len(runs[0].Steps) != 2 {
		t.Fatalf("读取结果不对: %+v", runs)
	}
	if !runs[0].Steps[0].Skipped {
		t.Errorf("read 的 Skipped 应往返保留为 true")
	}
	if runs[0].Steps[1].Skipped {
		t.Errorf("write 的 Skipped 应为 false")
	}
}
