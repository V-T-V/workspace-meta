// alert.go 实现一个轻量的告警规则引擎：把 Registry 导出的 MetricPoint
// 喂给一组 AlertRule，触发阈值条件的规则会产出 Alert。
//
// 设计要点（对齐 Prometheus alerting rules 的语义，但零依赖、纯内存）：
//   - 一条 AlertRule = (MetricName, Operator, Threshold) 三元组。
//     Operator 取 ">" / "<" / ">=" / "<="，未知算符视为永不触发（安全降级）。
//   - Evaluate 遍历所有 point × 所有 rule：只要某个 point 满足规则就产一条 Alert，
//     Alert.Value 记录触发时的实际值（便于排障）。多条规则/多个 point 命中会各自独立产出。
//   - Severity / Message 直接来自规则定义，引擎不做"升级/抑制"等复合语义
//     （那是上层告警平台的事，这里只做最朴素的"规则 → 命中即报"）。
package metrics

import (
	"fmt"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// AlertRule 定义一个告警规则。
type AlertRule struct {
	Name       string  // 规则名（如 "high_error_rate"），用于追溯
	MetricName string  // 监控的 metric 名（与 MetricPoint.Name 比对）
	Threshold  float64 // 阈值
	Operator   string  // ">" / "<" / ">=" / "<="
	Severity   string  // "warning" / "critical"
	Message    string  // 告警描述（可含 %v 等占位符，引擎不格式化，留给调用方）
}

// AlertEngine 评估告警规则。
type AlertEngine struct {
	rules []AlertRule
}

// NewAlertEngine 创建告警引擎。rules 为空时 Evaluate 永远返回 nil。
func NewAlertEngine(rules []AlertRule) *AlertEngine {
	return &AlertEngine{rules: rules}
}

// Rules 返回引擎当前持有的规则切片（只读视图，调用方不应修改）。
func (ae *AlertEngine) Rules() []AlertRule { return ae.rules }

// Evaluate 评估所有规则，返回触发的告警列表。
//
// 对每个 point 逐条比对规则：MetricName 必须相等，且 value 必须满足
// 规则的 Operator+Threshold，二者皆成立才产出一条 Alert。
// 未知 Operator 跳过该规则（不触发，安全降级）。
func (ae *AlertEngine) Evaluate(points []types.MetricPoint) []Alert {
	if ae == nil || len(ae.rules) == 0 || len(points) == 0 {
		return nil
	}
	var out []Alert
	for _, p := range points {
		for _, r := range ae.rules {
			if p.Name != r.MetricName {
				continue
			}
			if !matchOperator(r.Operator, p.Value, r.Threshold) {
				continue
			}
			out = append(out, Alert{
				Rule:     r,
				Value:    p.Value,
				Severity: r.Severity,
				Message:  r.Message,
			})
		}
	}
	return out
}

// matchOperator 判断 value op threshold 是否成立。
// 未知 op 一律返回 false（永不触发，避免把拼写错误的算符误判成默认比较）。
func matchOperator(op string, value, threshold float64) bool {
	switch op {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	}
	return false
}

// Alert 是一条触发的告警。
type Alert struct {
	Rule     AlertRule // 触发它的规则
	Value    float64   // 触发时的 metric 实际值
	Severity string    // 冗余存放 Severity（便于消费方直接读，不必再走 Rule.Severity）
	Message  string    // 冗余存放 Message
}

// String 返回人类可读的单行摘要，便于日志/CLI 输出。
// 形如：[critical] high_error_rate: requests_total=123.00 > 100 (error rate too high)
func (a Alert) String() string {
	return fmt.Sprintf("[%s] %s: %s=%.2f %s %.2f (%s)",
		a.Severity, a.Rule.Name, a.Rule.MetricName, a.Value, a.Rule.Operator, a.Rule.Threshold, a.Message)
}
