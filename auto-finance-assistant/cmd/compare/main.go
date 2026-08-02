// compare — 多模型横向对比评测工具（v2: 编码修复 + 思考链处理 + 稳健性）
// 用法：compare.exe -ollama http://127.0.0.1:11434
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ModelConfig struct {
	Name       string  `json:"name"`
	Label      string  `json:"label"`
	SizeGB     float64 `json:"sizeGB"`
	Family     string  `json:"family"`
}

// 10 题对比集：覆盖速度/准确度/边界保护/金融计算
var compareQuestions = []struct {
	Category string
	Question string
	Expect   []string
}{
	{"Speed", "新车贷款最低首付比例是多少", []string{"20|%"}},
	{"Speed", "申请贷款需要什么材料", []string{"身份证"}},
	{"Speed", "贷款利率是多少", []string{"3.5|7.9|%"}},
	{"Accuracy", "首付最少付多少", []string{"20|%"}},
	{"Accuracy", "企业客户需要什么额外材料", []string{"营业执照|财务"}},
	{"Accuracy", "贷款期限最长多少", []string{"60|5年|五年"}},
	{"Accuracy", "逾期了怎么办", []string{"0.05|罚息|逾期"}},
	{"Accuracy", "审批需要多长时间", []string{"3|5|工作日|天"}},
	{"Accuracy", "等额本息和等额本金有什么区别", []string{"本金|利息"}},
	{"Finance", "贷款20万年利率4.5%分36期等额本息月供多少", []string{"元|月供"}},
}

type ModelResult struct {
	Model       string     `json:"model"`
	Label       string     `json:"label"`
	SizeGB      float64    `json:"sizeGB"`
	Family      string     `json:"family"`
	PassCount   int        `json:"passCount"`
	TotalCount  int        `json:"totalCount"`
	Accuracy    float64    `json:"accuracy"`
	AvgWallMs   int64      `json:"avgWallMs"`
	AvgTPS      float64    `json:"avgTps"`
	TotalTokens int        `json:"totalTokens"`
	Results     []QAResult `json:"results"`
}

type QAResult struct {
	Category string  `json:"category"`
	Question string  `json:"question"`
	Pass     bool    `json:"pass"`
	WallMs   int64   `json:"wallMs"`
	Tokens   int     `json:"tokens"`
	TPS      float64 `json:"tps"`
	Answer   string  `json:"answer"`
}

func main() {
	ollamaURL := flag.String("ollama", "http://127.0.0.1:11434", "Ollama URL")
	outputDir := flag.String("out", "reports", "Output directory")
	modelFilter := flag.String("models", "", "Comma-separated model names (empty=all)")
	skipFamilies := flag.String("skip", "embed", "Skip model families (comma-separated)")
	flag.Parse()

	fmt.Println("============================================")
	fmt.Println("  Multi-Model Comparison Benchmark")
	fmt.Println("============================================")
	fmt.Printf("Ollama: %s\n\n", *ollamaURL)

	models, err := listModels(*ollamaURL)
	if err != nil {
		fmt.Println("[FATAL] Cannot list models:", err)
		os.Exit(1)
	}

	// 过滤
	skipMap := map[string]bool{}
	for _, s := range strings.Split(*skipFamilies, ",") {
		skipMap[strings.TrimSpace(s)] = true
	}
	var candidates []ModelConfig
	for _, m := range models {
		if skipMap[m.Family] {
			continue
		}
		if *modelFilter != "" {
			matched := false
			for _, f := range strings.Split(*modelFilter, ",") {
				if strings.Contains(m.Name, strings.TrimSpace(f)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		candidates = append(candidates, m)
	}

	fmt.Printf("Models to test: %d\n", len(candidates))
	for _, m := range candidates {
		fmt.Printf("  - %s (%s, %.1fGB, %s)\n", m.Name, m.Label, m.SizeGB, m.Family)
	}
	fmt.Println()

	// 逐模型测试
	var results []ModelResult
	for _, mc := range candidates {
		fmt.Printf("=== %s (%s) ===\n", mc.Name, mc.Label)
		r := testModel(*ollamaURL, mc)
		results = append(results, r)
		fmt.Printf("  => %d/%d (%.0f%%)  avg %dms  %.1f tok/s\n\n",
			r.PassCount, r.TotalCount, r.Accuracy, r.AvgWallMs, r.AvgTPS)
	}

	// 生成报告
	os.MkdirAll(*outputDir, 0755)
	ts := time.Now().Format("20060102-150405")
	mdPath := filepath.Join(*outputDir, "model-compare-"+ts+".md")
	jsonPath := filepath.Join(*outputDir, "model-compare-"+ts+".json")
	latestMD := filepath.Join(*outputDir, "model-compare-latest.md")

	report := buildReport(results)
	os.WriteFile(mdPath, []byte(report), 0644)
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(jsonPath, jsonData, 0644)
	os.WriteFile(latestMD, []byte(report), 0644)

	// 汇总
	fmt.Println("==============================================")
	fmt.Println("  Summary")
	fmt.Println("==============================================")
	fmt.Printf("%-22s %-8s %7s %8s %7s %7s\n", "Model", "Acc", "Wall(ms)", "TPS", "Tokens", "Size")
	fmt.Println(strings.Repeat("-", 65))
	for _, r := range results {
		fmt.Printf("%-22s %.0f%%      %7d %8.1f %7d %5.1fGB\n",
			r.Model, r.Accuracy, r.AvgWallMs, r.AvgTPS, r.TotalTokens, r.SizeGB)
	}
	fmt.Printf("\nReport: %s\n", mdPath)
}

func listModels(baseURL string) ([]ModelConfig, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize  string `json:"parameter_size"`
				Family         string `json:"family"`
				Quantization   string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	var out []ModelConfig
	for _, m := range data.Models {
		out = append(out, ModelConfig{
			Name: m.Name, Label: m.Details.ParameterSize,
			SizeGB: float64(m.Size) / 1e9, Family: m.Details.Family,
		})
	}
	return out, nil
}

func testModel(ollamaURL string, mc ModelConfig) ModelResult {
	mr := ModelResult{
		Model: mc.Name, Label: mc.Label, SizeGB: mc.SizeGB,
		Family: mc.Family, TotalCount: len(compareQuestions),
	}

	var totalWall int64
	var totalTPS float64
	var tpsCount int

	for i, q := range compareQuestions {
		fmt.Printf("  [%d/%d] %s... ", i+1, len(compareQuestions), q.Question[:min(15, len([]rune(q.Question)))])

		answer, tokens, wallMs, err := callOllama(ollamaURL, mc.Name, q.Question)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			mr.Results = append(mr.Results, QAResult{Category: q.Category, Question: q.Question})
			continue
		}

		tps := 0.0
		if tokens > 0 && wallMs > 0 {
			tps = float64(tokens) / (float64(wallMs) / 1000.0)
		}
		pass := checkAnswer(answer, q.Expect)
		if pass {
			mr.PassCount++
		}

		status := "PASS"
		if !pass {
			status = "FAIL"
		}
		fmt.Printf("%s %dms %.1ftok/s\n", status, wallMs, tps)

		mr.Results = append(mr.Results, QAResult{
			Category: q.Category, Question: q.Question, Pass: pass,
			WallMs: wallMs, Tokens: tokens, TPS: tps, Answer: answer,
		})
		totalWall += wallMs
		mr.TotalTokens += tokens
		if tps > 0 {
			totalTPS += tps
			tpsCount++
		}
	}

	mr.Accuracy = float64(mr.PassCount) / float64(mr.TotalCount) * 100
	mr.AvgWallMs = totalWall / int64(len(compareQuestions))
	if tpsCount > 0 {
		mr.AvgTPS = totalTPS / float64(tpsCount)
	}
	return mr
}

// callOllama 直接调 Ollama API（绕过服务，测纯模型能力）。
// 对 qwen3 系列，解析 thinking 字段，只取 content 部分。
func callOllama(baseURL, model, question string) (answer string, tokens int, wallMs int64, err error) {
	start := time.Now()

	// 用 json.Marshal 确保中文 UTF-8 正确编码
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": question}},
		"stream":   false,
		"options": map[string]interface{}{
			"num_predict": 300,
			"temperature": 0.7,
		},
	})

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Post(baseURL+"/api/chat", "application/json; charset=utf-8", bytes.NewReader(reqBody))
	if err != nil {
		return "", 0, time.Since(start).Milliseconds(), err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	wallMs = time.Since(start).Milliseconds()

	var r struct {
		Message struct {
			Content  string `json:"content"`
			Thinking string `json:"thinking"` // qwen3 思考链（不计入回答）
		} `json:"message"`
		EvalCount       int `json:"eval_count"`
		PromptEvalCount int `json:"prompt_eval_count"`
		Error           string `json:"error"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", 0, wallMs, fmt.Errorf("parse error: %w", err)
	}
	if r.Error != "" {
		return "", 0, wallMs, fmt.Errorf("ollama error: %s", r.Error)
	}

	answer = r.Message.Content
	// 如果 content 为空但有 thinking，说明思考链消耗了预算
	// 此时用 thinking 的最后部分作为回答（总比空好）
	if answer == "" && r.Message.Thinking != "" {
		// 取思考链最后的结论部分
		lines := strings.Split(r.Message.Thinking, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if len(line) > 20 {
				answer = line
				break
			}
		}
	}

	tokens = r.EvalCount
	return answer, tokens, wallMs, nil
}

func checkAnswer(answer string, expects []string) bool {
	if answer == "" {
		return false
	}
	lower := strings.ToLower(answer)
	for _, exp := range expects {
		if strings.Contains(exp, "|") {
			for _, alt := range strings.Split(exp, "|") {
				if strings.Contains(lower, strings.ToLower(strings.TrimSpace(alt))) {
					return true
				}
			}
		} else {
			if strings.Contains(lower, strings.ToLower(exp)) {
				return true
			}
		}
	}
	return false
}

func buildReport(results []ModelResult) string {
	var b strings.Builder
	now := time.Now().Format("2006-01-02 15:04:05")
	hostname, _ := os.Hostname()

	b.WriteString(fmt.Sprintf("# 多模型横向对比评测报告\n\n"))
	b.WriteString(fmt.Sprintf("| 项目 | 值 |\n|------|----|\n"))
	b.WriteString(fmt.Sprintf("| 时间 | %s |\n", now))
	b.WriteString(fmt.Sprintf("| 主机 | %s |\n", hostname))
	b.WriteString(fmt.Sprintf("| 测试题数 | %d |\n\n", len(compareQuestions)))

	// 排序：准确率降序，同准确率按 TPS 降序
	sorted := make([]ModelResult, len(results))
	copy(sorted, results)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Accuracy > sorted[i].Accuracy ||
				(sorted[j].Accuracy == sorted[i].Accuracy && sorted[j].AvgTPS > sorted[i].AvgTPS) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	b.WriteString("## 排名总表\n\n")
	b.WriteString("| 排名 | 模型 | 参数 | 大小(GB) | 准确率 | 平均延迟(ms) | 平均 TPS | 累计 Tokens |\n")
	b.WriteString("|------|------|------|---------|--------|------------|---------|------------|\n")
	for i, r := range sorted {
		medal := ""
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		}
		b.WriteString(fmt.Sprintf("| %d %s | %s | %s | %.1f | **%.0f%%** | %d | %.1f | %d |\n",
			i+1, medal, r.Model, r.Label, r.SizeGB, r.Accuracy, r.AvgWallMs, r.AvgTPS, r.TotalTokens))
	}

	// 选型矩阵
	if len(sorted) > 0 {
		best := sorted[0]
		fastest := sorted[0]
		smallest := sorted[0]
		for _, r := range sorted[1:] {
			if r.AvgTPS > fastest.AvgTPS {
				fastest = r
			}
			if r.SizeGB < smallest.SizeGB && r.Accuracy >= 50 {
				smallest = r
			}
		}

		b.WriteString("\n## 选型推荐\n\n")
		b.WriteString("| 场景 | 推荐模型 | 准确率 | 速度 | 大小 | 理由 |\n")
		b.WriteString("|------|---------|--------|------|------|------|\n")

		// CPU 最佳平衡
		cpuBest := sorted[0]
		for _, r := range sorted {
			if r.Accuracy >= 60 && r.AvgTPS > cpuBest.AvgTPS {
				cpuBest = r
			}
		}
		b.WriteString(fmt.Sprintf("| CPU 部署（平衡） | %s | %.0f%% | %.1f tok/s | %.1fGB | 准确率达标且速度最快 |\n",
			cpuBest.Model, cpuBest.Accuracy, cpuBest.AvgTPS, cpuBest.SizeGB))

		b.WriteString(fmt.Sprintf("| GPU 部署（质量优先） | %s | %.0f%% | %.1f tok/s | %.1fGB | 最高准确率 |\n",
			best.Model, best.Accuracy, best.AvgTPS, best.SizeGB))

		b.WriteString(fmt.Sprintf("| 极速响应 | %s | %.0f%% | %.1f tok/s | %.1fGB | 速度最快 |\n",
			fastest.Model, fastest.Accuracy, fastest.AvgTPS, fastest.SizeGB))

		if smallest.Model != cpuBest.Model && smallest.Model != best.Model {
			b.WriteString(fmt.Sprintf("| 低资源设备 | %s | %.0f%% | %.1f tok/s | %.1fGB | 模型最小 |\n",
				smallest.Model, smallest.Accuracy, smallest.AvgTPS, smallest.SizeGB))
		}
	}

	// 详细结果
	b.WriteString("\n## 详细结果\n\n")
	for _, r := range sorted {
		b.WriteString(fmt.Sprintf("\n### %s (%s, %.1fGB) — %.0f%%\n\n", r.Model, r.Label, r.SizeGB, r.Accuracy))
		b.WriteString("| # | 类别 | 问题 | 结果 | 延迟 | TPS | Tokens | 回答预览 |\n")
		b.WriteString("|---|------|------|------|------|-----|--------|----------|\n")
		for i, qa := range r.Results {
			status := "PASS"
			if !qa.Pass {
				status = "FAIL"
			}
			preview := qa.Answer
			if len([]rune(preview)) > 50 {
				preview = string([]rune(preview)[:50]) + "..."
			}
			preview = strings.ReplaceAll(preview, "\n", " ")
			b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %dms | %.1f | %d | %s |\n",
				i+1, qa.Category, truncate(qa.Question, 18), status,
				qa.WallMs, qa.TPS, qa.Tokens, preview))
		}
	}

	// 分析结论
	b.WriteString("\n## 分析结论\n\n")
	if len(sorted) > 0 {
		b.WriteString(fmt.Sprintf("1. **最佳综合**：%s（%.0f%%准确率，%.1f tok/s）\n", sorted[0].Model, sorted[0].Accuracy, sorted[0].AvgTPS))
		if len(sorted) > 1 {
			b.WriteString(fmt.Sprintf("2. **性价比**：%s 到 %s 之间准确率差距 %.0f%%，但速度差距 %.1f倍\n",
				sorted[0].Model, sorted[len(sorted)-1].Model,
				sorted[0].Accuracy-sorted[len(sorted)-1].Accuracy,
				sorted[0].AvgTPS/sorted[len(sorted)-1].AvgTPS))
		}
		// qwen3 分析
		for _, r := range results {
			if strings.Contains(r.Model, "qwen3") {
				b.WriteString(fmt.Sprintf("3. **Qwen3 系列注意**：%s 因思考链消耗 token，建议通过服务端部署时设置 max_output_tokens=0（不限）\n", r.Model))
				break
			}
		}
	}

	b.WriteString(fmt.Sprintf("\n---\n*报告由 compare 工具自动生成 · %s*\n", now))
	return b.String()
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
