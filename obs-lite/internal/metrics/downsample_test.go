package metrics

import (
	"testing"
	"time"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// 本文件测试 Downsample：按时间窗口把高频 MetricPoint 降采样。
//
// 聚合规则：
//   - counter：窗口内取最后一个点（时间戳最大）的值
//   - gauge：窗口内取平均值
//   - 同 (name, labels) 的点构成一个序列，分别降采样
//   - 输出 Timestamp = 窗口起点（对齐到 window 整数倍）

// TestDownsampleGaugeAverage 构造 10 个 gauge 点（1s 间隔），5s 窗口 → 降采样到 2 个点。
// 每窗口 5 个点取平均：[0..4] 均值 2，[5..9] 均值 7。
func TestDownsampleGaugeAverage(t *testing.T) {
	base := time.Unix(0, 0)
	pts := make([]types.MetricPoint, 10)
	for i := 0; i < 10; i++ {
		pts[i] = types.MetricPoint{
			Name:      "cpu",
			Kind:      types.MetricGauge,
			Value:     float64(i), // 0,1,2,...,9
			Timestamp: base.Add(time.Duration(i) * time.Second),
		}
	}
	out := Downsample(pts, 5*time.Second)
	if len(out) != 2 {
		t.Fatalf("5s 窗口下 10 个点应降采样到 2 个，实际 %d", len(out))
	}
	// 窗口 1：[0,1,2,3,4] 均值 = 2
	if out[0].Value != 2 {
		t.Errorf("第 1 窗口均值应为 2, 实际 %f", out[0].Value)
	}
	// 窗口 2：[5,6,7,8,9] 均值 = 7
	if out[1].Value != 7 {
		t.Errorf("第 2 窗口均值应为 7, 实际 %f", out[1].Value)
	}
	// 输出 Timestamp 对齐窗口起点
	if !out[0].Timestamp.Equal(base) {
		t.Errorf("第 1 窗口 Timestamp 应为 %v, 实际 %v", base, out[0].Timestamp)
	}
	if !out[1].Timestamp.Equal(base.Add(5 * time.Second)) {
		t.Errorf("第 2 窗口 Timestamp 应为 %v, 实际 %v", base.Add(5*time.Second), out[1].Timestamp)
	}
	// Kind/Name 保持
	if out[0].Kind != types.MetricGauge || out[0].Name != "cpu" {
		t.Errorf("降采样不应改变 Kind/Name, got %+v", out[0])
	}
}

// TestDownsampleCounterLastValue counter 取窗口内最末点。
// 10 个点 1s 间隔，counter 单调递增 0,1,...,9；5s 窗口 → 末值分别为 4 和 9。
func TestDownsampleCounterLastValue(t *testing.T) {
	base := time.Unix(0, 0)
	pts := make([]types.MetricPoint, 10)
	for i := 0; i < 10; i++ {
		pts[i] = types.MetricPoint{
			Name:      "requests_total",
			Kind:      types.MetricCounter,
			Value:     float64(i),
			Timestamp: base.Add(time.Duration(i) * time.Second),
		}
	}
	out := Downsample(pts, 5*time.Second)
	if len(out) != 2 {
		t.Fatalf("应降采样到 2 个点，实际 %d", len(out))
	}
	if out[0].Value != 4 {
		t.Errorf("第 1 窗口 counter 末值应为 4, 实际 %f", out[0].Value)
	}
	if out[1].Value != 9 {
		t.Errorf("第 2 窗口 counter 末值应为 9, 实际 %f", out[1].Value)
	}
}

// TestDownsampleMultipleSeries 不同序列分别降采样：cpu 与 mem 两条序列各 4 个点，
// 2s 窗口 → 每序列 2 个点，共 4 个输出点。
func TestDownsampleMultipleSeries(t *testing.T) {
	base := time.Unix(0, 0)
	mk := func(name string, vals []float64) []types.MetricPoint {
		out := make([]types.MetricPoint, len(vals))
		for i, v := range vals {
			out[i] = types.MetricPoint{
				Name: name, Kind: types.MetricGauge, Value: v,
				Timestamp: base.Add(time.Duration(i) * time.Second),
				Labels:    map[string]string{"host": "a"},
			}
		}
		return out
	}
	pts := append(mk("cpu", []float64{0, 2, 4, 6}), mk("mem", []float64{10, 20, 30, 40})...)
	out := Downsample(pts, 2*time.Second)
	if len(out) != 4 {
		t.Fatalf("两条序列各 2 窗口应得 4 个点，实际 %d", len(out))
	}
	// cpu 窗口：[0,2]→1, [4,6]→5；mem 窗口：[10,20]→15, [30,40]→35
	// 收集所有值便于断言（顺序：按时间再按 name）
	want := map[string][]float64{"cpu": {1, 5}, "mem": {15, 35}}
	got := map[string][]float64{}
	for _, p := range out {
		got[p.Name] = append(got[p.Name], p.Value)
	}
	for name, vs := range want {
		gs := got[name]
		if len(gs) != len(vs) {
			t.Errorf("%s 应有 %d 个点，实际 %d", name, len(vs), len(gs))
			continue
		}
		for i := range vs {
			if gs[i] != vs[i] {
				t.Errorf("%s[%d] 应为 %f, 实际 %f", name, i, vs[i], gs[i])
			}
		}
	}
}

// TestDownsampleEmpty 输入为空 → 返回 nil。
func TestDownsampleEmpty(t *testing.T) {
	if out := Downsample(nil, 5*time.Second); out != nil {
		t.Errorf("空输入应返回 nil, got %v", out)
	}
}

// TestDownsampleWindowZero window <= 0 → 不降采样，原样返回（拷贝）。
func TestDownsampleWindowZero(t *testing.T) {
	pts := []types.MetricPoint{
		{Name: "g", Kind: types.MetricGauge, Value: 1, Timestamp: time.Unix(0, 0)},
		{Name: "g", Kind: types.MetricGauge, Value: 2, Timestamp: time.Unix(1, 0)},
	}
	out := Downsample(pts, 0)
	if len(out) != 2 {
		t.Fatalf("window=0 不应降采样, 实际 %d 个点", len(out))
	}
	// 验证返回的是拷贝（改 out 不影响原切片）
	out[0].Value = 999
	if pts[0].Value != 1 {
		t.Error("Downsample 应返回拷贝，不应改动原切片")
	}
}

// TestDownsampleUnsortedInput 输入未排序也能正确降采样。
func TestDownsampleUnsortedInput(t *testing.T) {
	base := time.Unix(0, 0)
	// 故意打乱顺序
	pts := []types.MetricPoint{
		{Name: "g", Kind: types.MetricGauge, Value: 4, Timestamp: base.Add(4 * time.Second)},
		{Name: "g", Kind: types.MetricGauge, Value: 0, Timestamp: base},
		{Name: "g", Kind: types.MetricGauge, Value: 2, Timestamp: base.Add(2 * time.Second)},
		{Name: "g", Kind: types.MetricGauge, Value: 1, Timestamp: base.Add(1 * time.Second)},
		{Name: "g", Kind: types.MetricGauge, Value: 3, Timestamp: base.Add(3 * time.Second)},
	}
	out := Downsample(pts, 5*time.Second)
	if len(out) != 1 {
		t.Fatalf("5s 窗口下应降采样到 1 个点，实际 %d", len(out))
	}
	// 均值 (0+1+2+3+4)/5 = 2
	if out[0].Value != 2 {
		t.Errorf("无序输入降采样均值应为 2, 实际 %f", out[0].Value)
	}
}
