// Package evaluations 实现业务评测 runner。
// 对应原计划第二十三节。从 JSONL 读取问题集，调用 API，对比预期。
// 用法：go run ./evaluations -url http://127.0.0.1:8080 -dataset evaluations/questions.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// EvalCase 评测用例。
type EvalCase struct {
	Question        string   `json:"question"`
	ExpectedIntent  string   `json:"expectedIntent"`
	RequiredFacts   []string `json:"requiredFacts"`
	ForbiddenFacts  []string `json:"forbiddenFacts"`
	ShouldRefuse    bool     `json:"shouldRefuse"`
}

// EvalResult 单条评测结果。
type EvalResult struct {
	Case       EvalCase
	GotIntent  string
	GotAnswer  string
	Pass       bool
	FailReason string
	DurationMS int64
}

func main() {
	url := flag.String("url", "http://127.0.0.1:8080", "服务地址")
	dataset := flag.String("dataset", "evaluations/questions.jsonl", "评测集路径")
	flag.Parse()

	cases, err := loadDataset(*dataset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载评测集失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("=== 评测开始：%d 条 ===\n", len(cases))

	results := make([]EvalResult, 0, len(cases))
	pass := 0
	for i, c := range cases {
		fmt.Printf("[%d/%d] %s\n", i+1, len(cases), truncate(c.Question, 40))
		r := evaluate(*url, c)
		results = append(results, r)
		if r.Pass {
			pass++
		} else {
			fmt.Printf("  ✗ %s\n", r.FailReason)
		}
	}

	fmt.Println("\n=== 汇总 ===")
	if len(results) == 0 {
		fmt.Println("无有效评测用例（数据集为空或格式错误）")
		return
	}
	fmt.Printf("通过：%d/%d (%.1f%%)\n", pass, len(results), float64(pass)/float64(len(results))*100)

	// 按意图统计
	intentStats := map[string]struct{ pass, total int }{}
	for _, r := range results {
		k := r.Case.ExpectedIntent
		if k == "" {
			k = "other"
		}
		s := intentStats[k]
		s.total++
		if r.Pass {
			s.pass++
		}
		intentStats[k] = s
	}
	fmt.Println("\n按意图：")
	for k, s := range intentStats {
		fmt.Printf("  %s: %d/%d\n", k, s.pass, s.total)
	}
}

func evaluate(baseURL string, c EvalCase) EvalResult {
	start := time.Now()
	body := fmt.Sprintf(`{"question":%q}`, c.Question)
	resp, err := http.Post(baseURL+"/api/chat", "application/json", strings.NewReader(body))
	if err != nil {
		return EvalResult{Case: c, FailReason: "请求失败: " + err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var data struct {
		Answer  string `json:"answer"`
		Intent  string `json:"intent"`
	}
	_ = json.Unmarshal(raw, &data)
	dur := time.Since(start).Milliseconds()

	r := EvalResult{Case: c, GotIntent: data.Intent, GotAnswer: data.Answer, DurationMS: dur}

	// 判定
	if c.ShouldRefuse && data.Intent != "refuse" {
		r.FailReason = "应拒答但未拒答"
		return r
	}
	// 必需事实
	for _, fact := range c.RequiredFacts {
		if !strings.Contains(data.Answer, fact) {
			r.FailReason = fmt.Sprintf("缺少必需事实：%s", fact)
			return r
		}
	}
	// 禁止事实
	for _, fact := range c.ForbiddenFacts {
		if strings.Contains(data.Answer, fact) {
			r.FailReason = fmt.Sprintf("包含禁止事实：%s", fact)
			return r
		}
	}
	r.Pass = true
	return r
}

func loadDataset(path string) ([]EvalCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cases []EvalCase
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c EvalCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		cases = append(cases, c)
	}
	return cases, scanner.Err()
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
