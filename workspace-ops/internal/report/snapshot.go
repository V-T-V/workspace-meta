// Package report 的扩展快照导出。
//
// SnapshotMarkdown 相对 MarkdownReporter.Format 更"自包含、丰富"：
// MarkdownReporter 只输出标题 + 一行栈汇总 + 项目表；
// SnapshotMarkdown 额外加头部摘要段、栈分布饼图描述（纯文本渲染）、
// 健康分布统计（按分数段分桶）和带健康列的完整项目表，
// 适合作为一次完整扫描的归档快照（可直接落盘为单个 .md 文件）。

package report

import (
	"fmt"
	"sort"
	"strings"
)

// SnapshotMarkdown 生成完整的工作区快照 Markdown 文档。
// 含：扫描时间、项目总数、栈分布表、健康评分分布、完整项目表。
func SnapshotMarkdown(r Report) string {
	var b strings.Builder

	// ===== 1. 标题 + 头部摘要段 =====
	b.WriteString("# 工作区快照\n\n")
	if r.ScanAt != "" {
		fmt.Fprintf(&b, "**扫描时间**: %s  \n", r.ScanAt)
	} else {
		b.WriteString("**扫描时间**: _(未记录)_  \n")
	}
	fmt.Fprintf(&b, "**项目总数**: %d  \n", r.ProjectCount)
	fmt.Fprintf(&b, "**技术栈种类**: %d\n\n", len(r.StackSummary))

	// ===== 2. 栈分布表 + 饼图描述 =====
	stackRows := sortedStackSummary(r.StackSummary)
	total := r.ProjectCount
	if total == 0 {
		total = 1 // 避免除零；空报告时占比无意义但不至于 panic
	}

	b.WriteString("## 技术栈分布\n\n")
	b.WriteString("| 技术栈 | 项目数 | 占比 | 分布 |\n")
	b.WriteString("|--------|--------|------|------|\n")
	for _, s := range stackRows {
		pct := 100.0 * float64(s.count) / float64(total)
		fmt.Fprintf(&b, "| %s | %d | %.1f%% | %s |\n",
			or(s.stack, "unknown"), s.count, pct, barChart(pct))
	}
	b.WriteString("\n")
	// 纯文本饼图描述：用文字列出最大栈占比，便于无图表渲染环境理解。
	if len(stackRows) > 0 {
		top := stackRows[0]
		fmt.Fprintf(&b, "> 主导栈：**%s**（%d 个，约 %.0f%%）\n\n",
			or(top.stack, "unknown"), top.count, 100.0*float64(top.count)/float64(total))
	}

	// ===== 3. 健康评分分布统计 =====
	health := healthDistribution(r.Projects)
	b.WriteString("## 健康评分分布\n\n")
	b.WriteString("| 健康区间 | 项目数 | 占比 | 分布 |\n")
	b.WriteString("|----------|--------|------|------|\n")
	for _, band := range healthBands {
		c := health[band.key]
		pct := 100.0 * float64(c) / float64(total)
		fmt.Fprintf(&b, "| %s (%s) | %d | %.1f%% | %s |\n",
			band.label, band.key, c, pct, barChart(pct))
	}
	// 平均健康分
	if len(r.Projects) > 0 {
		sum := 0
		for _, p := range r.Projects {
			sum += p.HealthScore
		}
		fmt.Fprintf(&b, "\n> 平均健康分：**%.1f / 100**\n\n", float64(sum)/float64(len(r.Projects)))
	} else {
		b.WriteString("\n> _(无项目)_\n\n")
	}

	// ===== 4. 完整项目表 =====
	b.WriteString("## 项目明细\n\n")
	b.WriteString("| Slug | Stack | Tests | AGENTS.md | Git | Health |\n")
	b.WriteString("|------|-------|-------|-----------|-----|--------|\n")
	for _, p := range r.Projects {
		git := p.GitBranch
		if p.GitDirty {
			git += "*"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d |\n",
			p.Slug, or(p.StackPrimary, "-"), or(p.TestCount, "0"),
			boolMark(p.HasAgentsMD), or(git, "-"), p.HealthScore)
	}
	// 末尾不留多余空行以外，给一个文档结束标记便于拼接。
	b.WriteString("\n---\n_快照由 workspace-ops 生成_\n")
	return b.String()
}

// stackRow 是 StackSummary 排序后的中间结构。
type stackRow struct {
	stack string
	count int
}

// sortedStackSummary 把 StackSummary map 按 count 降序、stack 升序转成稳定切片。
func sortedStackSummary(m map[string]int) []stackRow {
	out := make([]stackRow, 0, len(m))
	for k, v := range m {
		out = append(out, stackRow{stack: k, count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].count != out[j].count {
			return out[i].count > out[j].count
		}
		return out[i].stack < out[j].stack
	})
	return out
}

// healthBand 描述一个健康分数段（用于分布统计）。
type healthBand struct {
	key   string // 区间标记，如 "80-100"
	label string // 中文标签，如 "优秀"
	min   int    // 该段下界（含）
	max   int    // 该段上界（含）
}

// healthBands 定义健康分数段（分数 0-100）。
// 顺序即输出顺序，从高到低。
var healthBands = []healthBand{
	{key: "80-100", label: "优秀", min: 80, max: 100},
	{key: "60-79", label: "良好", min: 60, max: 79},
	{key: "40-59", label: "一般", min: 40, max: 59},
	{key: "0-39", label: "较差", min: 0, max: 39},
}

// healthDistribution 统计各分数段的项目数，返回 key -> count。
func healthDistribution(projects []ProjectView) map[string]int {
	out := map[string]int{}
	for _, p := range projects {
		for _, band := range healthBands {
			if p.HealthScore >= band.min && p.HealthScore <= band.max {
				out[band.key]++
				break
			}
		}
	}
	return out
}

// barChart 把百分比渲染成纯文本条形（每 10% 一个方块，最多 10 个）。
// 选型说明：零依赖、任何 Markdown 渲染器都能看，不依赖 mermaid/图片。
func barChart(pct float64) string {
	n := int(pct/10 + 0.5)
	if n < 0 {
		n = 0
	}
	if n > 10 {
		n = 10
	}
	return strings.Repeat("█", n) + strings.Repeat("░", 10-n)
}
