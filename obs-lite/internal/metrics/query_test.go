package metrics

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// helper：构造一批 metric 点。
func mkPoints() []types.MetricPoint {
	return []types.MetricPoint{
		{Name: "requests", Value: 100, Labels: map[string]string{"method": "GET", "host": "a"}},
		{Name: "requests", Value: 50, Labels: map[string]string{"method": "POST", "host": "a"}},
		{Name: "requests", Value: 30, Labels: map[string]string{"method": "GET", "host": "b"}},
		{Name: "errors", Value: 5, Labels: map[string]string{"method": "GET", "host": "a"}},
		{Name: "errors", Value: 2, Labels: map[string]string{"method": "POST", "host": "b"}},
		{Name: "latency", Value: 0.5, Labels: map[string]string{"host": "a"}},
	}
}

// ===== FilterByLabel =====

func TestFilterByLabel(t *testing.T) {
	pts := mkPoints()
	// 过滤 method=GET：3 个点（2 个 requests + 1 个 errors）
	got := FilterByLabel(pts, "method", "GET")
	if len(got) != 3 {
		t.Errorf("method=GET 应有 3 个点，实际 %d", len(got))
	}
	for _, p := range got {
		if p.Labels["method"] != "GET" {
			t.Errorf("过滤结果含非 GET 点：%v", p.Labels)
		}
	}
}

func TestFilterByLabelHost(t *testing.T) {
	pts := mkPoints()
	// 过滤 host=b：2 个点（requests GET + errors POST）
	got := FilterByLabel(pts, "host", "b")
	if len(got) != 2 {
		t.Errorf("host=b 应有 2 个点，实际 %d", len(got))
	}
}

func TestFilterByLabelNoMatch(t *testing.T) {
	pts := mkPoints()
	got := FilterByLabel(pts, "method", "DELETE")
	if len(got) != 0 {
		t.Errorf("无匹配应返回空切片，实际 %d 个", len(got))
	}
}

func TestFilterByLabelEmptyKey(t *testing.T) {
	// 空 key 返回 nil（不 panic）。
	got := FilterByLabel(mkPoints(), "", "anything")
	if got != nil {
		t.Errorf("空 key 应返回 nil，实际 %v", got)
	}
}

func TestFilterByLabelNilLabels(t *testing.T) {
	// 点的 Labels 为 nil 不应 panic，且不匹配。
	pts := []types.MetricPoint{
		{Name: "x", Value: 1, Labels: nil},
		{Name: "y", Value: 2, Labels: map[string]string{"k": "v"}},
	}
	got := FilterByLabel(pts, "k", "v")
	if len(got) != 1 {
		t.Errorf("应只匹配 1 个点（nil labels 跳过），实际 %d", len(got))
	}
}

func TestFilterByLabelDoesNotMutateInput(t *testing.T) {
	pts := mkPoints()
	_ = FilterByLabel(pts, "method", "GET")
	// 原切片长度不变。
	if len(pts) != len(mkPoints()) {
		t.Error("FilterByLabel 不应修改入参切片")
	}
}

// ===== SumByName =====

func TestSumByName(t *testing.T) {
	pts := mkPoints()
	// requests: 100 + 50 + 30 = 180
	if got := SumByName(pts, "requests"); got != 180 {
		t.Errorf("requests 总和 = %v, want 180", got)
	}
	// errors: 5 + 2 = 7
	if got := SumByName(pts, "errors"); got != 7 {
		t.Errorf("errors 总和 = %v, want 7", got)
	}
}

func TestSumByNameNoMatch(t *testing.T) {
	if got := SumByName(mkPoints(), "nonexistent"); got != 0 {
		t.Errorf("无匹配 name 应返回 0，实际 %v", got)
	}
}

func TestSumByNameEmpty(t *testing.T) {
	if got := SumByName(nil, "x"); got != 0 {
		t.Errorf("空入参应返回 0，实际 %v", got)
	}
}

// ===== GroupSum =====

func TestGroupSum(t *testing.T) {
	pts := mkPoints()
	got := GroupSum(pts)
	want := map[string]float64{
		"requests": 180,
		"errors":   7,
		"latency":  0.5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupSum = %v, want %v", got, want)
	}
}

func TestGroupSumEmpty(t *testing.T) {
	// 空入参返回空非 nil map（可安全遍历）。
	got := GroupSum(nil)
	if got == nil {
		t.Fatal("空入参应返回非 nil map")
	}
	if len(got) != 0 {
		t.Errorf("空入参应返回空 map，实际 %v", got)
	}
}

func TestGroupSumFloatPrecision(t *testing.T) {
	// 浮点累加：0.1 * 10 应接近 1（容忍浮点误差）。
	pts := make([]types.MetricPoint, 10)
	for i := range pts {
		pts[i] = types.MetricPoint{Name: "frac", Value: 0.1}
	}
	got := GroupSum(pts)["frac"]
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("0.1*10 累加 = %v，期望接近 1.0", got)
	}
}

// ===== TopN =====

func TestTopN(t *testing.T) {
	pts := mkPoints()
	// 按 Sum 降序：requests(180) > errors(7) > latency(0.5)
	got := TopN(pts, 2)
	if len(got) != 2 {
		t.Fatalf("TopN(2) 应返回 2 个，实际 %d", len(got))
	}
	if got[0].Name != "requests" || got[0].Sum != 180 {
		t.Errorf("TopN[0] 应为 requests/180，实际 %v", got[0])
	}
	if got[1].Name != "errors" || got[1].Sum != 7 {
		t.Errorf("TopN[1] 应为 errors/7，实际 %v", got[1])
	}
}

func TestTopNCountField(t *testing.T) {
	// Count 应反映该 name 的点数（不标签组合数）。
	pts := mkPoints()
	got := TopN(pts, 1)[0]
	// requests 有 3 个点。
	if got.Name != "requests" || got.Count != 3 {
		t.Errorf("requests 应 Count=3，实际 %v", got)
	}
}

func TestTopNAll(t *testing.T) {
	// n > name 数量时返回全部（去重后 3 个 name）。
	got := TopN(mkPoints(), 100)
	if len(got) != 3 {
		t.Errorf("n 超过 name 数时应返回全部 3 个，实际 %d", len(got))
	}
}

func TestTopNOrderingDesc(t *testing.T) {
	// 整体应按 Sum 降序。
	got := TopN(mkPoints(), 10)
	for i := 1; i < len(got); i++ {
		if got[i].Sum > got[i-1].Sum {
			t.Errorf("TopN 应 Sum 降序：位置 %d (%v) > 位置 %d (%v)", i, got[i], i-1, got[i-1])
		}
	}
}

func TestTopNTieBreakByName(t *testing.T) {
	// Sum 相同时按 Name 升序（确定性）。
	pts := []types.MetricPoint{
		{Name: "zeta", Value: 10},
		{Name: "alpha", Value: 10},
		{Name: "mid", Value: 10},
	}
	got := TopN(pts, 3)
	// 三个 Sum 都 = 10，应按 name 升序：alpha, mid, zeta
	want := []string{"alpha", "mid", "zeta"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("并列时位置 %d 应为 %s，实际 %s（全量：%v）", i, w, got[i].Name, got)
		}
	}
}

func TestTopNDeterministic(t *testing.T) {
	// 同样输入两次，结果应完全一致（依赖稳定排序）。
	pts := mkPoints()
	a := TopN(pts, 3)
	b := TopN(pts, 3)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("TopN 应确定性：%v vs %v", a, b)
	}
}

func TestTopNNegative(t *testing.T) {
	// n <= 0 返回 nil。
	if got := TopN(mkPoints(), 0); got != nil {
		t.Errorf("TopN(0) 应返回 nil，实际 %v", got)
	}
	if got := TopN(mkPoints(), -1); got != nil {
		t.Errorf("TopN(-1) 应返回 nil，实际 %v", got)
	}
}

func TestTopNEmpty(t *testing.T) {
	// 空入参 + 正 n 返回空切片（非 nil 切片，但长度 0）。
	got := TopN(nil, 5)
	if len(got) != 0 {
		t.Errorf("空入参应返回长度 0，实际 %d", len(got))
	}
}

func TestRate(t *testing.T) {
	now := time.Now()
	points := []types.MetricPoint{
		{Name: "requests", Kind: types.MetricCounter, Value: 100, Timestamp: now},
		{Name: "requests", Kind: types.MetricCounter, Value: 160, Timestamp: now.Add(10 * time.Second)},
	}
	rate := Rate(points, "requests")
	if rate != 6 { // (160-100)/10 = 6/s
		t.Errorf("Rate 应为 6/s，实际 %.1f", rate)
	}
}

func TestRateSingle(t *testing.T) {
	rate := Rate([]types.MetricPoint{{Name: "x", Value: 1, Timestamp: time.Now()}}, "x")
	if rate != 0 {
		t.Error("单个点 Rate 应为 0")
	}
}

// ===== Percentile =====
//
// 构造一个已知分布并验证线性插值百分位估算。
// 用 Histogram.Observe 真实累积，再 Data() 取出 HistogramData，确保走完整采集链路。
//
// 桶 [1, 2, 3]，观测值 {0.5, 1.5, 2.5, 2.5}（共 4 个）：
//
//	累计计数：[1, 2, 4, 4]（最后一个是 +Inf 桶，Count == 总数 4）
//	- p50  target=2  → 落桶 1(ub=2)  → 2 + 0   = 2.0
//	- p90  target=3.6→ 落桶 2(ub=3)  → 2 + 0.8 = 2.8
//	- p99  target=3.96→落桶 2(ub=3)  → 2 + 0.98= 2.98
func TestPercentile(t *testing.T) {
	hist := NewHistogram("latency", []float64{1, 2, 3})
	for _, v := range []float64{0.5, 1.5, 2.5, 2.5} {
		hist.Observe(v, nil)
	}
	data := hist.Data()
	if len(data) != 1 {
		t.Fatalf("应有 1 组 histogram 数据（无标签），实际 %d", len(data))
	}

	cases := []struct {
		name string
		p    float64
		want float64
	}{
		{"p50", 50, 2.0},
		{"p90", 90, 2.8},
		{"p99", 99, 2.98},
	}
	for _, c := range cases {
		got := Percentile(data[0], c.p)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPercentileBoundaries(t *testing.T) {
	hist := NewHistogram("latency", []float64{1, 2, 3})
	for _, v := range []float64{0.5, 1.5, 2.5, 2.5} {
		hist.Observe(v, nil)
	}
	data := hist.Data()[0]

	// p0 返回 0（最低桶下界）。
	if got := Percentile(data, 0); got != 0 {
		t.Errorf("p0 应返回 0，实际 %v", got)
	}
	// p100 target=4 落到 ub=3 的桶，fraction=1 → 返回桶上界 3。
	if got := Percentile(data, 100); got != 3 {
		t.Errorf("p100 应返回 3（最大有限桶上界），实际 %v", got)
	}
	// p > 100 应被截断为 100，结果同上。
	if got := Percentile(data, 150); got != 3 {
		t.Errorf("p150 应截断为 p100 → 3，实际 %v", got)
	}
	// 负百分位视为 0。
	if got := Percentile(data, -5); got != 0 {
		t.Errorf("负百分位应返回 0，实际 %v", got)
	}
}

func TestPercentileAllInFirstBucket(t *testing.T) {
	// 所有观测都落在第一个桶内：p50 应落在桶 0，在 [0, ub0] 间插值。
	hist := NewHistogram("latency", []float64{1, 2, 3})
	for i := 0; i < 10; i++ {
		hist.Observe(0.1, nil) // 全部 <= 1
	}
	data := hist.Data()[0]
	// 桶 0(ub=1) count=10。target=5,prevCount=0,bucketCount=10,fraction=0.5 → 0+0.5*1=0.5
	if got := Percentile(data, 50); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("全落在首桶时 p50 应在 [0,1] 中点 = 0.5，实际 %v", got)
	}
}

func TestPercentileEmpty(t *testing.T) {
	// 空 histogram（无观测）返回 0，不 panic。
	empty := types.HistogramData{
		Name:    "x",
		Buckets: []types.HistogramBucket{{UpperBound: 1, Count: 0}, {UpperBound: math.Inf(1), Count: 0}},
		Count:   0,
	}
	if got := Percentile(empty, 50); got != 0 {
		t.Errorf("空 histogram 应返回 0，实际 %v", got)
	}
}

// ===== ExtractLabels =====

func TestExtractLabelsMethod(t *testing.T) {
	pts := mkPoints()
	// method 标签有 GET/POST 两种不同值，去重后升序。
	got := ExtractLabels(pts, "method")
	want := []string{"GET", "POST"}
	if len(got) != len(want) {
		t.Fatalf("method 应有 %d 个不同值，实际 %d（%v）", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("位置 %d 应为 %s，实际 %s（全量 %v）", i, w, got[i], got)
		}
	}
}

func TestExtractLabelsHost(t *testing.T) {
	pts := mkPoints()
	// host 标签有 a/b 两种值。
	got := ExtractLabels(pts, "host")
	if len(got) != 2 {
		t.Fatalf("host 应有 2 个不同值，实际 %d（%v）", len(got), got)
	}
	// 升序：a 在 b 前。
	if got[0] != "a" || got[1] != "b" {
		t.Errorf("host 升序应为 [a b]，实际 %v", got)
	}
}

func TestExtractLabelsSortedAsc(t *testing.T) {
	// 构造一组乱序标签值，验证返回是字母序升序且确定。
	pts := []types.MetricPoint{
		{Labels: map[string]string{"k": "zeta"}},
		{Labels: map[string]string{"k": "alpha"}},
		{Labels: map[string]string{"k": "mid"}},
		{Labels: map[string]string{"k": "alpha"}}, // 重复
	}
	got := ExtractLabels(pts, "k")
	want := []string{"alpha", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("应有 %d 个去重值，实际 %d（%v）", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("升序位置 %d 应为 %s，实际 %s（全量 %v）", i, w, got[i], got)
		}
	}
}

func TestExtractLabelsNoMatch(t *testing.T) {
	// 没有任何点含该 key → 返回 nil。
	if got := ExtractLabels(mkPoints(), "nonexistent"); got != nil {
		t.Errorf("无匹配 key 应返回 nil，实际 %v", got)
	}
}

func TestExtractLabelsEmptyKey(t *testing.T) {
	// 空 key 返回 nil（不命中所有点）。
	if got := ExtractLabels(mkPoints(), ""); got != nil {
		t.Errorf("空 key 应返回 nil，实际 %v", got)
	}
}

func TestExtractLabelsEmptyInput(t *testing.T) {
	// 空入参返回 nil（无点可提取）。
	if got := ExtractLabels(nil, "method"); got != nil {
		t.Errorf("空入参应返回 nil，实际 %v", got)
	}
}

func TestExtractLabelsNilLabelsSkipped(t *testing.T) {
	// Labels 为 nil 的点应被跳过，不 panic。
	pts := []types.MetricPoint{
		{Labels: nil},
		{Labels: map[string]string{"k": "v1"}},
		{Labels: nil},
		{Labels: map[string]string{"k": "v2"}},
	}
	got := ExtractLabels(pts, "k")
	if len(got) != 2 {
		t.Errorf("应跳过 nil labels 后剩 2 个值，实际 %d（%v）", len(got), got)
	}
}

func TestExtractLabelsDoesNotMutateInput(t *testing.T) {
	pts := mkPoints()
	origLen := len(pts)
	_ = ExtractLabels(pts, "method")
	if len(pts) != origLen {
		t.Error("ExtractLabels 不应修改入参切片长度")
	}
	// 标签 map 不应被破坏：method=GET 仍可取到。
	if pts[0].Labels["method"] != "GET" {
		t.Error("ExtractLabels 不应破坏入参的 Labels map")
	}
}

func TestDedup(t *testing.T) {
	now := time.Now()
	points := []types.MetricPoint{
		{Name: "x", Value: 1, Timestamp: now},
		{Name: "y", Value: 2, Timestamp: now},
		{Name: "x", Value: 3, Timestamp: now.Add(time.Second)},
	}
	deduped := Dedup(points)
	if len(deduped) != 2 {
		t.Errorf("去重后应有 2 个，实际 %d", len(deduped))
	}
	// x 的最后一个值应是 3
	for _, p := range deduped {
		if p.Name == "x" && p.Value != 3 {
			t.Error("x 应保留最后值 3")
		}
	}
}
