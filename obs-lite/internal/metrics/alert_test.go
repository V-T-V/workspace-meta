package metrics

import (
	"testing"
	"time"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// TestAlertEngineCounterOverThreshold 构造 counter 超阈值 → 触发告警。
// 覆盖主路径：Registry → Counter → AllPoints → AlertEngine.Evaluate。
func TestAlertEngineCounterOverThreshold(t *testing.T) {
	reg := NewRegistry()
	reqCounter := reg.Counter("requests_total")
	// 制造 120 次请求（标签 method=GET），超过阈值 100。
	for i := 0; i < 120; i++ {
		reqCounter.Inc(map[string]string{"method": "GET"})
	}

	engine := NewAlertEngine([]AlertRule{
		{
			Name: "high_request_rate", MetricName: "requests_total",
			Threshold: 100, Operator: ">",
			Severity: "warning", Message: "request rate too high",
		},
	})

	alerts := engine.Evaluate(reg.AllPoints())
	if len(alerts) != 1 {
		t.Fatalf("应触发 1 条告警，实际 %d 条: %+v", len(alerts), alerts)
	}
	a := alerts[0]
	if a.Rule.Name != "high_request_rate" {
		t.Errorf("告警规则名应为 high_request_rate，实际 %q", a.Rule.Name)
	}
	if a.Value != 120 {
		t.Errorf("触发值应为 120，实际 %f", a.Value)
	}
	if a.Severity != "warning" {
		t.Errorf("严重度应为 warning，实际 %q", a.Severity)
	}
	if a.Message != "request rate too high" {
		t.Errorf("Message 应透传，实际 %q", a.Message)
	}
}

// TestAlertEngineAllOperators 覆盖 4 种算符 + 未知算符的语义。
func TestAlertEngineAllOperators(t *testing.T) {
	point := func(name string, v float64) []types.MetricPoint {
		return []types.MetricPoint{{Name: name, Kind: types.MetricGauge, Value: v, Timestamp: time.Now()}}
	}
	cases := []struct {
		op        string
		value     float64
		threshold float64
		want      bool
	}{
		{">", 10, 5, true},
		{">", 5, 5, false},
		{"<", 3, 5, true},
		{"<", 5, 5, false},
		{">=", 5, 5, true},
		{">=", 4, 5, false},
		{"<=", 5, 5, true},
		{"<=", 6, 5, false},
		{"==", 5, 5, false}, // 未知算符：永不触发
		{"", 5, 0, false},   // 空算符：永不触发
	}
	for i, c := range cases {
		ae := NewAlertEngine([]AlertRule{{
			Name: "r", MetricName: "m", Threshold: c.threshold, Operator: c.op,
		}})
		got := ae.Evaluate(point("m", c.value))
		if c.want && len(got) != 1 {
			t.Errorf("case %d (%s %.0f %.0f) 应触发，实际 %d 条", i, c.op, c.value, c.threshold, len(got))
		}
		if !c.want && len(got) != 0 {
			t.Errorf("case %d (%s %.0f %.0f) 不应触发，实际 %d 条", i, c.op, c.value, c.threshold, len(got))
		}
	}
}

// TestAlertEngineNoMatch 验证不命中条件：metric 名不匹配、值未过阈值都不触发。
func TestAlertEngineNoMatch(t *testing.T) {
	points := []types.MetricPoint{
		{Name: "latency_ms", Value: 50, Timestamp: time.Now()},
	}
	ae := NewAlertEngine([]AlertRule{
		{Name: "wrong_metric", MetricName: "other_metric", Threshold: 10, Operator: ">"},
		{Name: "below_threshold", MetricName: "latency_ms", Threshold: 100, Operator: ">"},
	})
	if alerts := ae.Evaluate(points); len(alerts) != 0 {
		t.Errorf("不应触发告警，实际 %d 条: %+v", len(alerts), alerts)
	}
}

// TestAlertEngineMultipleRulesMultiplePoints 验证多规则 × 多 point 的笛卡尔命中。
// 同一 metric 的不同标签会导出多个 point，每个超阈值的 point 都应单独触发告警。
func TestAlertEngineMultipleRulesMultiplePoints(t *testing.T) {
	reg := NewRegistry()
	errCounter := reg.Counter("errors_total")
	// GET 100、POST 50（只有 GET 超 80 阈值）。
	for i := 0; i < 100; i++ {
		errCounter.Inc(map[string]string{"method": "GET"})
	}
	for i := 0; i < 50; i++ {
		errCounter.Inc(map[string]string{"method": "POST"})
	}

	ae := NewAlertEngine([]AlertRule{
		{Name: "err_warn", MetricName: "errors_total", Threshold: 80, Operator: ">", Severity: "warning"},
		{Name: "err_crit", MetricName: "errors_total", Threshold: 90, Operator: ">", Severity: "critical"},
	})
	alerts := ae.Evaluate(reg.AllPoints())
	// GET(100) 同时超 80 和 90 → 2 条；POST(50) 都不超 → 0 条。共 2 条。
	if len(alerts) != 2 {
		t.Fatalf("应触发 2 条告警（GET 同时命中 warn/crit），实际 %d", len(alerts))
	}
	sevCount := map[string]int{}
	for _, a := range alerts {
		sevCount[a.Severity]++
	}
	if sevCount["warning"] != 1 || sevCount["critical"] != 1 {
		t.Errorf("应各有 1 条 warning/critical，实际 %v", sevCount)
	}
}

// TestAlertEngineEmpty 保证空规则、空 point 不 panic 且返回 nil。
func TestAlertEngineEmpty(t *testing.T) {
	if a := NewAlertEngine(nil).Evaluate(regPoints()); a != nil {
		t.Errorf("空规则应返回 nil，实际 %+v", a)
	}
	if a := NewAlertEngine([]AlertRule{{MetricName: "x", Operator: ">"}}).Evaluate(nil); a != nil {
		t.Errorf("空 point 应返回 nil，实际 %+v", a)
	}
}

// TestAlertString 验证 Alert.String 的可读格式。
func TestAlertString(t *testing.T) {
	a := Alert{
		Rule:  AlertRule{Name: "rule-a", MetricName: "cpu", Threshold: 90, Operator: ">"},
		Value: 95.5, Severity: "critical", Message: "cpu overload",
	}
	s := a.String()
	// 至少包含 severity、规则名、metric 名、实际值、算符、阈值。
	for _, want := range []string{"critical", "rule-a", "cpu", "95.50", ">", "90.00", "cpu overload"} {
		if !contains(s, want) {
			t.Errorf("Alert.String() 应含 %q\n实际: %s", want, s)
		}
	}
}

// TestAlertEngineNilSafe 验证 nil 引擎的 Evaluate 不 panic（防御式）。
func TestAlertEngineNilSafe(t *testing.T) {
	var ae *AlertEngine
	if a := ae.Evaluate(regPoints()); a != nil {
		t.Errorf("nil 引擎应返回 nil，实际 %+v", a)
	}
}

// regPoints 返回一个固定的单点切片，用于空输入测试。
func regPoints() []types.MetricPoint {
	return []types.MetricPoint{{Name: "x", Value: 1, Timestamp: time.Now()}}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
