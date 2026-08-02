package report

import (
	"strings"
	"testing"
)

// TestSnapshotMarkdown 验证快照含全部主要区块：标题、摘要段、栈分布表、
// 健康分布表、完整项目表。
func TestSnapshotMarkdown(t *testing.T) {
	r := Build(sampleFacts())
	r.ScanAt = "2026-08-01 12:00:00" // 模拟扫描时间
	out := SnapshotMarkdown(r)

	// 标题 + 摘要段
	for _, want := range []string{
		"# 工作区快照",
		"**扫描时间**: 2026-08-01 12:00:00",
		"**项目总数**: 2",
		"**技术栈种类**: 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SnapshotMarkdown 缺少 %q\n输出:\n%s", want, out)
		}
	}

	// 栈分布表
	for _, want := range []string{
		"## 技术栈分布",
		"| 技术栈 | 项目数 | 占比 | 分布 |",
		"| go | 1 | 50.0%",
		"| node/ts | 1 | 50.0%",
		"主导栈",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SnapshotMarkdown 栈分布缺少 %q\n输出:\n%s", want, out)
		}
	}

	// 健康分布表
	for _, want := range []string{
		"## 健康评分分布",
		"| 健康区间 | 项目数 | 占比 | 分布 |",
		"优秀 (80-100)", // proj-a=90
		"一般 (40-59)",  // proj-b=50
		"平均健康分",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SnapshotMarkdown 健康分布缺少 %q\n输出:\n%s", want, out)
		}
	}

	// 完整项目表
	for _, want := range []string{
		"## 项目明细",
		"| Slug | Stack | Tests | AGENTS.md | Git | Health |",
		"| proj-a | go | 12 | ✓ | main | 90 |",
		"| proj-b | node/ts | 5 | ✗ | dev* | 50 |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SnapshotMarkdown 项目表缺少 %q\n输出:\n%s", want, out)
		}
	}

	// 结束标记
	if !strings.Contains(out, "_快照由 workspace-ops 生成_") {
		t.Errorf("SnapshotMarkdown 应含结束标记\n输出:\n%s", out)
	}
}

// TestSnapshotMarkdownHealthBandCounts 用手工构造的分数覆盖各区间，
// 验证分布计数正确（不依赖 CalculateHealth 的具体得分）。
func TestSnapshotMarkdownHealthBandCounts(t *testing.T) {
	r := Report{
		ScanAt:       "t",
		ProjectCount: 4,
		StackSummary: map[string]int{"go": 4},
		Projects: []ProjectView{
			{Slug: "a", StackPrimary: "go", HealthScore: 95}, // 优秀
			{Slug: "b", StackPrimary: "go", HealthScore: 70}, // 良好
			{Slug: "c", StackPrimary: "go", HealthScore: 50}, // 一般
			{Slug: "d", StackPrimary: "go", HealthScore: 10}, // 较差
		},
	}
	out := SnapshotMarkdown(r)
	// 每段恰好 1 个项目（25.0%）
	for _, band := range []string{"优秀 (80-100)", "良好 (60-79)", "一般 (40-59)", "较差 (0-39)"} {
		line := band
		if !strings.Contains(out, line+" | 1 | 25.0%") {
			t.Errorf("健康段 %s 应计数 1 / 25.0%%\n输出:\n%s", band, out)
		}
	}
}

// TestSnapshotMarkdownStackOrdering 验证栈分布按 count 降序、并列按 stack 升序。
func TestSnapshotMarkdownStackOrdering(t *testing.T) {
	r := Report{
		ScanAt:       "t",
		ProjectCount: 4,
		StackSummary: map[string]int{"go": 2, "node/ts": 1, "rust": 1},
		Projects: []ProjectView{
			{Slug: "a", StackPrimary: "go", HealthScore: 90},
			{Slug: "b", StackPrimary: "go", HealthScore: 90},
			{Slug: "c", StackPrimary: "node/ts", HealthScore: 90},
			{Slug: "d", StackPrimary: "rust", HealthScore: 90},
		},
	}
	out := SnapshotMarkdown(r)
	// go(2) 应排在 node/ts(1) 与 rust(1) 之前。
	idxGo := strings.Index(out, "| go |")
	idxNode := strings.Index(out, "| node/ts |")
	idxRust := strings.Index(out, "| rust |")
	if idxGo < 0 || idxNode < 0 || idxRust < 0 {
		t.Fatalf("找不到栈行\n输出:\n%s", out)
	}
	if !(idxGo < idxNode && idxGo < idxRust) {
		t.Errorf("go(2) 应排在最前，实际 go=%d node=%d rust=%d", idxGo, idxNode, idxRust)
	}
	// node/ts 与 rust 并列(count=1)，按 stack 升序：node/ts < rust。
	if !(idxNode < idxRust) {
		t.Errorf("并列时应按 stack 升序 node/ts < rust，实际 node=%d rust=%d", idxNode, idxRust)
	}
}

// TestSnapshotMarkdownEmpty 验证空报告不 panic 且结构完整。
func TestSnapshotMarkdownEmpty(t *testing.T) {
	r := Report{StackSummary: map[string]int{}}
	out := SnapshotMarkdown(r)
	for _, want := range []string{
		"# 工作区快照",
		"**项目总数**: 0",
		"## 技术栈分布",
		"## 健康评分分布",
		"## 项目明细",
		"| Slug | Stack | Tests | AGENTS.md | Git | Health |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("空快照仍应含 %q\n输出:\n%s", want, out)
		}
	}
}

// TestBarChart 验证条形渲染（每 10% 一个方块，封顶 10 个）。
func TestBarChart(t *testing.T) {
	cases := []struct {
		pct  float64
		full int
	}{
		{0, 0},
		{5, 1}, // 0.5 -> 1
		{50, 5},
		{100, 10},
		{150, 10}, // 封顶
	}
	for _, c := range cases {
		got := barChart(c.pct)
		if strings.Count(got, "█") != c.full {
			t.Errorf("barChart(%.0f) 实心块数=%d, want %d (got=%q)", c.pct, strings.Count(got, "█"), c.full, got)
		}
		// 总长度恒为 10
		if len([]rune(got)) != 10 {
			t.Errorf("barChart 总长应为 10，实际 %d (got=%q)", len([]rune(got)), got)
		}
	}
}
