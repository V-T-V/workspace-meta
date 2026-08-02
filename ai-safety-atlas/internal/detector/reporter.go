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

// CSVReport 把批量结果导出为 CSV 格式（每条输入一行，含风险分/级别/命中规则数）。
// 列：input, risk_score, risk_level, detection_count, rules, severities
func CSVReport(results []BatchResult) string {
	var sb strings.Builder
	sb.WriteString("input,risk_score,risk_level,detection_count,rules,severities\n")
	for _, r := range results {
		rules := make([]string, 0, len(r.Detections))
		sevs := make([]string, 0, len(r.Detections))
		for _, d := range r.Detections {
			rules = append(rules, d.Rule)
			sevs = append(sevs, d.Severity.String())
		}
		// CSV 转义：输入里的双引号翻倍
		escInput := strings.ReplaceAll(r.Input, "\"", "\"\"")
		sb.WriteString(fmt.Sprintf("\"%s\",%d,%s,%d,\"%s\",\"%s\"\n",
			escInput, r.RiskScore, r.RiskLevel, len(r.Detections),
			strings.Join(rules, ";"), strings.Join(sevs, ";")))
	}
	return sb.String()
}

// MarkdownReport 把批量结果导出为 Markdown 表格格式。
//
// 输出结构：
//
//	# 批量检测报告
//
//	**总计**: N  **安全**: S  **命中**: F
//
//	| input | score | level | rules |
//	|-------|-------|-------|-------|
//	| ...   | 42    | MEDIUM| ruleA; ruleB |
//
// rules 列把每条检测的规则名按 "; " 连接；安全输入该列为空。
// 单元格内的 "|" 转义为 "\|"，避免破坏表格。
func MarkdownReport(results []BatchResult) string {
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

	// 标题 + 汇总段
	sb.WriteString("# 批量检测报告\n\n")
	sb.WriteString(fmt.Sprintf("**总计**: %d  **安全**: %d  **命中**: %d\n\n", total, safe, flagged))

	// 表格
	sb.WriteString("| input | score | level | rules |\n")
	sb.WriteString("|-------|-------|-------|-------|\n")
	for _, r := range results {
		rules := make([]string, 0, len(r.Detections))
		for _, d := range r.Detections {
			rules = append(rules, d.Rule)
		}
		input := mdEscapeCell(r.Input)
		rulesCol := mdEscapeCell(strings.Join(rules, "; "))
		sb.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n",
			input, r.RiskScore, r.RiskLevel, rulesCol))
	}
	return sb.String()
}

// mdEscapeCell 转义 Markdown 表格单元格里的特殊字符：
// "|" 会破坏表格列结构，换行会破坏行结构，反斜杠需先转义避免吞掉后续字符。
func mdEscapeCell(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
