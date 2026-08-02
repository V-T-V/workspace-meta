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

// HTMLReport 把批量结果导出为自包含 HTML 页面（含 CSS 样式）。
//
// 输出结构：
//   - <!DOCTYPE html> + <html>，<head> 内嵌 <style>（不外链，零依赖、可直接保存为单文件打开）
//   - 顶部汇总段：总计 / 安全 / 命中
//   - <table> 结果表格，每条输入一行，含输入、风险分、风险级别、命中规则
//   - 风险级别用 CSS class 着色：SAFE=绿、LOW=蓝、MEDIUM=黄、HIGH=橙、CRITICAL=红
//
// 所有用户输入文本都经 htmlEscape 转义，避免 XSS / 破坏标签结构。
func HTMLReport(results []BatchResult) string {
	total := len(results)
	safe, flagged := 0, 0
	for _, r := range results {
		if len(r.Detections) == 0 {
			safe++
		} else {
			flagged++
		}
	}

	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n")
	sb.WriteString("<html lang=\"zh-CN\">\n<head>\n")
	sb.WriteString("<meta charset=\"utf-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	sb.WriteString("<title>批量检测报告</title>\n")
	sb.WriteString("<style>\n")
	sb.WriteString("body{font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;margin:2rem auto;max-width:60rem;color:#222;line-height:1.5}\n")
	sb.WriteString("h1{font-size:1.6rem;margin:0 0 .5rem}\n")
	sb.WriteString(".summary{display:flex;gap:1.5rem;flex-wrap:wrap;margin:1rem 0 1.5rem;font-size:1.05rem}\n")
	sb.WriteString(".summary .stat{background:#f5f7fa;border-radius:.4rem;padding:.5rem 1rem}\n")
	sb.WriteString(".summary .stat b{font-size:1.2rem}\n")
	sb.WriteString("table{border-collapse:collapse;width:100%;font-size:.95rem}\n")
	sb.WriteString("th,td{border:1px solid #dfe3e8;padding:.5rem .7rem;text-align:left;vertical-align:top}\n")
	sb.WriteString("th{background:#f5f7fa}\n")
	sb.WriteString("tr:nth-child(even) td{background:#fafbfc}\n")
	sb.WriteString(".level{display:inline-block;padding:.1rem .5rem;border-radius:1rem;font-size:.8rem;font-weight:600;color:#fff;white-space:nowrap}\n")
	sb.WriteString(".level-SAFE{background:#2e7d32}\n")
	sb.WriteString(".level-LOW{background:#1565c0}\n")
	sb.WriteString(".level-MEDIUM{background:#f9a825}\n")
	sb.WriteString(".level-HIGH{background:#ef6c00}\n")
	sb.WriteString(".level-CRITICAL{background:#c62828}\n")
	sb.WriteString("td.num{text-align:center;white-space:nowrap}\n")
	sb.WriteString("</style>\n")
	sb.WriteString("</head>\n<body>\n")

	// 标题 + 汇总段
	sb.WriteString("<h1>批量检测报告</h1>\n")
	sb.WriteString("<div class=\"summary\">\n")
	sb.WriteString(fmt.Sprintf("<div class=\"stat\">总计 <b>%d</b></div>\n", total))
	sb.WriteString(fmt.Sprintf("<div class=\"stat\">安全 <b>%d</b></div>\n", safe))
	sb.WriteString(fmt.Sprintf("<div class=\"stat\">命中 <b>%d</b></div>\n", flagged))
	sb.WriteString("</div>\n")

	// 结果表格
	sb.WriteString("<table>\n")
	sb.WriteString("<thead><tr><th>#</th><th>输入</th><th>风险分</th><th>风险级别</th><th>命中规则</th></tr></thead>\n")
	sb.WriteString("<tbody>\n")
	for i, r := range results {
		rules := make([]string, 0, len(r.Detections))
		for _, d := range r.Detections {
			rules = append(rules, d.Rule)
		}
		rulesCol := htmlEscape(strings.Join(rules, "; "))
		if rulesCol == "" {
			rulesCol = "&mdash;"
		}
		sb.WriteString("<tr>")
		sb.WriteString(fmt.Sprintf("<td class=\"num\">%d</td>", i+1))
		sb.WriteString(fmt.Sprintf("<td>%s</td>", htmlEscape(r.Input)))
		sb.WriteString(fmt.Sprintf("<td class=\"num\">%d</td>", r.RiskScore))
		sb.WriteString(fmt.Sprintf("<td class=\"num\"><span class=\"level level-%s\">%s</span></td>",
			htmlEscape(r.RiskLevel), htmlEscape(r.RiskLevel)))
		sb.WriteString(fmt.Sprintf("<td>%s</td>", rulesCol))
		sb.WriteString("</tr>\n")
	}
	sb.WriteString("</tbody>\n</table>\n")
	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}

// htmlEscape 转义 HTML 文本中的特殊字符，避免注入与破坏标签结构。
// 覆盖 & < > " '，等价于 html.EscapeString 的核心子集（零依赖、自实现）。
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
