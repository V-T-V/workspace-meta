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
