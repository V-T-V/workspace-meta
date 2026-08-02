// Package alignment 实现对齐评估指标：precision/recall/accuracy/F1。
//
// 用红队测试用例集（redteam）+ 检测器（detector）跑批量评估，
// 计算：
//   - precision（精确率）：被标记为攻击的，多少真是攻击（误报率 = 1 - precision）
//   - recall（召回率）：真攻击中，多少被检测到（漏报率 = 1 - recall）
//   - accuracy（准确率）：总体正确判断比例
//   - F1：precision 和 recall 的调和平均
package alignment

import (
	"fmt"

	"github.com/QiuShichang/ai-safety-atlas/internal/detector"
	"github.com/QiuShichang/ai-safety-atlas/internal/redteam"
	"github.com/QiuShichang/ai-safety-atlas/internal/types"
)

// Metrics 是评估指标汇总。
type Metrics struct {
	Total      int     // 总用例数
	TruePos    int     // 真阳：攻击被正确标记
	FalsePos   int     // 假阳：良性被误标为攻击（误报）
	TrueNeg    int     // 真阴：良性正确未标记
	FalseNeg   int     // 假阴：攻击漏检
	Precision  float64 // TP / (TP + FP)
	Recall     float64 // TP / (TP + FN)
	Accuracy   float64 // (TP + TN) / Total
	F1         float64 // 2*P*R / (P+R)
	ByCategory map[string]CategoryMetrics
}

// CategoryMetrics 是按攻击类别的细分指标。
type CategoryMetrics struct {
	Total    int
	Detected int
	Recall   float64
}

// Evaluate 用内置红队用例集评估检测器性能。
func Evaluate(det *detector.Detector) Metrics {
	cases := redteam.Default()
	return EvaluateCases(det, cases)
}

// EvaluateCases 用指定用例集评估检测器。
func EvaluateCases(det *detector.Detector, cases []redteam.TestCase) Metrics {
	m := Metrics{ByCategory: map[string]CategoryMetrics{}}
	m.Total = len(cases)
	for _, c := range cases {
		isAttack := c.Category != "benign"
		detected := len(det.Analyze(c.Input)) > 0
		switch {
		case isAttack && detected:
			m.TruePos++
		case !isAttack && detected:
			m.FalsePos++
		case !isAttack && !detected:
			m.TrueNeg++
		case isAttack && !detected:
			m.FalseNeg++
		}
		// 按类别统计（攻击类）
		if isAttack {
			cm := m.ByCategory[c.Category]
			cm.Total++
			if detected {
				cm.Detected++
			}
			m.ByCategory[c.Category] = cm
		}
	}
	// 计算比率
	if m.TruePos+m.FalsePos > 0 {
		m.Precision = float64(m.TruePos) / float64(m.TruePos+m.FalsePos)
	}
	if m.TruePos+m.FalseNeg > 0 {
		m.Recall = float64(m.TruePos) / float64(m.TruePos+m.FalseNeg)
	}
	if m.Total > 0 {
		m.Accuracy = float64(m.TruePos+m.TrueNeg) / float64(m.Total)
	}
	if m.Precision+m.Recall > 0 {
		m.F1 = 2 * m.Precision * m.Recall / (m.Precision + m.Recall)
	}
	// 类别召回率
	for k, v := range m.ByCategory {
		if v.Total > 0 {
			v.Recall = float64(v.Detected) / float64(v.Total)
		}
		m.ByCategory[k] = v
	}
	return m
}

// Format 把指标格式化成可读报告。
func Format(m Metrics) string {
	out := fmt.Sprintf("=== 检测器评估 ===\n")
	out += fmt.Sprintf("总用例: %d\n", m.Total)
	out += fmt.Sprintf("  真阳(TP): %d  假阳(FP): %d  真阴(TN): %d  假阴(FN): %d\n",
		m.TruePos, m.FalsePos, m.TrueNeg, m.FalseNeg)
	out += fmt.Sprintf("  精确率(Precision): %.1f%%（误报率 %.1f%%）\n", m.Precision*100, (1-m.Precision)*100)
	out += fmt.Sprintf("  召回率(Recall):    %.1f%%（漏报率 %.1f%%）\n", m.Recall*100, (1-m.Recall)*100)
	out += fmt.Sprintf("  准确率(Accuracy):  %.1f%%\n", m.Accuracy*100)
	out += fmt.Sprintf("  F1 分数:           %.3f\n", m.F1)
	if len(m.ByCategory) > 0 {
		out += "\n按攻击类别召回率:\n"
		for cat, cm := range m.ByCategory {
			out += fmt.Sprintf("  %-18s %d/%d (%.0f%%)\n", cat, cm.Detected, cm.Total, cm.Recall*100)
		}
	}
	return out
}

// FalsePositiveRate 计算良性输入被误标的比率。
//
// 对每个 benignInputs 跑检测器，若被标记为含攻击（Analyze 命中任何规则）即视为
// 一次误报。返回 误报数 / 总输入数：
//   - 0.0：无误报（理想，所有良性输入都判为安全）
//   - 1.0：全部被误标
//
// 空入参返回 0（避免除零），且不报错——便于调用方在"无良性样本"时
// 直接把该值当作 0 渲染到报告里。
//
// 与 Evaluate 的区别：Evaluate 用内置红队用例集（攻击+良性混合）算整体 precision；
// 本函数聚焦"纯良性集"上的误报率，适合做针对性回归（拿一批已知良性输入，
// 看检测器会不会过度拦截）。
func FalsePositiveRate(det *detector.Detector, benignInputs []string) float64 {
	if len(benignInputs) == 0 {
		return 0
	}
	falsePositives := 0
	for _, input := range benignInputs {
		// 命中任何规则即视为误报（det.Analyze 返回的 Detection 切片非空）。
		if len(det.Analyze(input)) > 0 {
			falsePositives++
		}
	}
	return float64(falsePositives) / float64(len(benignInputs))
}

// SeverityDistribution 统计检测结果按严重度的分布。
func SeverityDistribution(detections []types.Detection) map[types.Severity]int {
	out := map[types.Severity]int{}
	for _, d := range detections {
		out[d.Severity]++
	}
	return out
}
