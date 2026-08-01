package detector

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QiuShichang/ai-safety-atlas/internal/types"
)

// BatchResult 是单个输入的批量检测结果。
type BatchResult struct {
	Input      string            `json:"input"`
	Detections []types.Detection `json:"detections"`
	RiskScore  int               `json:"risk_score"`
	RiskLevel  string            `json:"risk_level"`
}

// BatchAnalyze 批量分析多个输入，返回结果切片。
func BatchAnalyze(det *Detector, inputs []string) []BatchResult {
	out := make([]BatchResult, 0, len(inputs))
	for _, in := range inputs {
		dets := det.Analyze(in)
		score := types.RiskScore(dets)
		out = append(out, BatchResult{
			Input:      in,
			Detections: dets,
			RiskScore:  score,
			RiskLevel:  types.RiskLevel(score),
		})
	}
	return out
}

// jsonReport 是 JSONReport 输出的顶层结构。
type jsonReport struct {
	Total   int           `json:"total"`
	Safe    int           `json:"safe"`
	Flagged int           `json:"flagged"`
	Results []BatchResult `json:"results"`
}

// JSONReport 把批量结果导出为 JSON 字符串（带统计摘要）。
func JSONReport(results []BatchResult) string {
	rep := jsonReport{
		Total:   len(results),
		Results: results,
	}
	for _, r := range results {
		if len(r.Detections) == 0 {
			rep.Safe++
		} else {
			rep.Flagged++
		}
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(b)
}

// TextReport 把批量结果导出为可读文本报告。
func TextReport(results []BatchResult) string {
	var sb strings.Builder
	total := len(results)
	safe, flagged := 0, 0
	for _, r := range results {
		if len(r.Detections) == 0 {
			safe++
		} else {
			flagged++
		}
	}

	sb.WriteString("=== 批量检测报告 ===\n")
	sb.WriteString(fmt.Sprintf("总计: %d  安全: %d  命中: %d\n\n", total, safe, flagged))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] 输入: %q\n", i+1, r.Input))
		if len(r.Detections) == 0 {
			sb.WriteString("  ✅ 安全\n\n")
			continue
		}
		sb.WriteString(fmt.Sprintf("  ⚠️ %s (%d/100), %d 个检测:\n", r.RiskLevel, r.RiskScore, len(r.Detections)))
		for _, d := range r.Detections {
			sb.WriteString(fmt.Sprintf("     [%s] %s — %s\n", d.Severity, d.Type, d.Rule))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
