// Package report 把 inspector/storage 的结果格式化为多种输出（text/json/markdown）。
// 设计：Reporter 接口 + Registry，对齐 generic-admin/internal/export 的可插拔范式。
package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QiuShichang/workspace-ops/internal/inspector"
)

// ProjectView 是单个项目在报告里的视图（带 facts 的扁平结构）。
type ProjectView struct {
	Slug              string            `json:"slug"`
	StackPrimary      string            `json:"stack_primary,omitempty"`
	StackAll          string            `json:"stack_all,omitempty"`
	HasAgentsMD       bool              `json:"has_agents_md"`
	GitBranch         string            `json:"git_branch,omitempty"`
	GitDirty          bool              `json:"git_dirty"`
	TestCount         string            `json:"test_count,omitempty"`
	TestStatus        string            `json:"test_status,omitempty"`   // 最近一次实跑测试状态：pass/fail/skipped/timeout/error
	TestDuration      string            `json:"test_duration,omitempty"` // 测试耗时（毫秒，人类可读）
	HealthScore       int               `json:"health_score"`            // 综合健康评分 0-100
	HasBuildArtifacts bool              `json:"has_build_artifacts,omitempty"`
	Facts             map[string]string `json:"facts,omitempty"`
}

// FromFacts 把 inspector.Facts 转成 ProjectView。
func FromFacts(f inspector.Facts) ProjectView {
	return ProjectView{
		Slug:         f.Slug,
		StackPrimary: f.Get("stack_primary"),
		StackAll:     f.Get("stack_all"),
		HasAgentsMD:  f.Get("has_agents_md") == "true",
		GitBranch:    f.Get("git_branch"),
		GitDirty:     f.Get("git_dirty") == "true",
		TestCount:    f.Get("test_count"),
		Facts:        f.KV,
	}
}

// Report 是一次完整报告的数据。
type Report struct {
	ScanAt       string         `json:"scan_at"`
	ProjectCount int            `json:"project_count"`
	StackSummary map[string]int `json:"stack_summary"` // stack -> count
	Projects     []ProjectView  `json:"projects"`
}

// Build 从 Facts 列表构建 Report。
// 每个项目的 HealthScore 在这里由 CalculateHealth 计算并填充
// （FromFacts 只做 facts 扁平化，不算分；HealthScore 留给本函数统一填）。
func Build(facts []inspector.Facts) Report {
	r := Report{
		Projects:     make([]ProjectView, 0, len(facts)),
		StackSummary: map[string]int{},
	}
	for _, f := range facts {
		v := FromFacts(f)
		// 推断 has_build_artifacts：facts 里若显式给出就采纳，供 CalculateHealth 加分。
		if f.Get("has_build_artifacts") == "true" {
			v.HasBuildArtifacts = true
		}
		v.HealthScore = CalculateHealth(v)
		r.Projects = append(r.Projects, v)
		r.ProjectCount++
		stack := v.StackPrimary
		if stack == "" {
			stack = "unknown"
		}
		r.StackSummary[stack]++
	}
	return r
}

// Reporter 格式化接口。
type Reporter interface {
	Format(r Report) string
}

// Registry 注册表。
type Registry struct {
	items map[string]Reporter
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{items: map[string]Reporter{}}
}

// Register 注册一个 reporter。
func (r *Registry) Register(format string, rep Reporter) {
	r.items[format] = rep
}

// Get 取某格式的 reporter。
func (r *Registry) Get(format string) (Reporter, error) {
	rep, ok := r.items[format]
	if !ok {
		return nil, fmt.Errorf("未知格式 %q（支持: %s）", format, strings.Join(r.Formats(), ", "))
	}
	return rep, nil
}

// Formats 返回所有已注册的格式名。
func (r *Registry) Formats() []string {
	out := make([]string, 0, len(r.items))
	for k := range r.items {
		out = append(out, k)
	}
	return out
}

// DefaultRegistry 返回注册了 text/json/markdown 三种格式的默认注册表。
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("text", TextReporter{})
	r.Register("json", JSONReporter{})
	r.Register("markdown", MarkdownReporter{})
	return r
}

// ===== Text =====

// TextReporter 输出终端友好的对齐表格。
type TextReporter struct{}

// Format 实现 Reporter。
func (TextReporter) Format(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "工作区扫描报告（%d 个项目）\n", r.ProjectCount)
	fmt.Fprintf(&b, "技术栈分布: %s\n\n", formatStackSummary(r.StackSummary))
	// 表头
	fmt.Fprintln(&b, pad("SLUG", 28), pad("STACK", 12), pad("TESTS", 6), pad("AGENTS", 7), pad("GIT", 20), pad("HEALTH", 6))
	fmt.Fprintln(&b, strings.Repeat("-", 85))
	for _, p := range r.Projects {
		git := p.GitBranch
		if p.GitDirty {
			git += "*"
		}
		if git == "" {
			git = "-"
		}
		fmt.Fprintln(&b,
			pad(p.Slug, 28),
			pad(or(p.StackPrimary, "-"), 12),
			pad(or(p.TestCount, "0"), 6),
			pad(boolMark(p.HasAgentsMD), 7),
			pad(git, 20),
			pad(fmt.Sprintf("%d", p.HealthScore), 6),
		)
	}
	return b.String()
}

// ===== JSON =====

// JSONReporter 输出 JSON。
type JSONReporter struct{}

// Format 实现 Reporter。
func (JSONReporter) Format(r Report) string {
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf("JSON 编码失败: %v", err)
	}
	return string(out)
}

// ===== Markdown =====

// MarkdownReporter 输出 Markdown 表格。
type MarkdownReporter struct{}

// Format 实现 Reporter。
func (MarkdownReporter) Format(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 工作区扫描报告（%d 个项目）\n\n", r.ProjectCount)
	fmt.Fprintf(&b, "**技术栈分布**: %s\n\n", formatStackSummary(r.StackSummary))
	fmt.Fprintln(&b, "| Slug | Stack | Tests | AGENTS.md | Git | Health |")
	fmt.Fprintln(&b, "|------|-------|-------|-----------|-----|--------|")
	for _, p := range r.Projects {
		git := p.GitBranch
		if p.GitDirty {
			git += "*"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d |\n",
			p.Slug, or(p.StackPrimary, "-"), or(p.TestCount, "0"),
			boolMark(p.HasAgentsMD), or(git, "-"), p.HealthScore)
	}
	return b.String()
}

// ===== 辅助 =====

// GroupByStack 按技术栈分组项目，返回 map[stack][]ProjectView。
//
// 分组键用 StackPrimary；空栈归入 "unknown"（与 Build 里 StackSummary 的口径一致，
// 保证二者可对照）。同一栈内的项目保持它们在输入里的相对顺序。
//
// 适合做"每种技术栈有几个项目、各是什么"的二次聚合分析。
func GroupByStack(projects []ProjectView) map[string][]ProjectView {
	groups := make(map[string][]ProjectView)
	for _, p := range projects {
		stack := p.StackPrimary
		if stack == "" {
			stack = "unknown"
		}
		groups[stack] = append(groups[stack], p)
	}
	return groups
}

// FormatGroupReport 输出按栈分组的报告：每个栈一段，含项目数与健康平均分。
//
// 输出形如：
//
//	工作区分组报告（共 N 个项目，M 个栈）
//
//	## go（3 个项目，平均健康 80）
//	  - proj-a  [health=90]
//	  - proj-c  [health=70]
//	  - proj-d  [health=80]
//
//	## node/ts（1 个项目，平均健康 50）
//	  - proj-b  [health=50]
//
// 栈按字母序输出（确定性，便于断言）；栈内项目按 Report.Projects 的原始顺序。
// 健康平均分对整数做四舍五入；空 Report 返回占位说明。
func FormatGroupReport(r Report) string {
	groups := GroupByStack(r.Projects)
	// 收集并排序栈名，保证输出确定性。
	stacks := make([]string, 0, len(groups))
	for s := range groups {
		stacks = append(stacks, s)
	}
	sortStrings(stacks)

	var b strings.Builder
	if r.ProjectCount == 0 || len(r.Projects) == 0 {
		fmt.Fprintf(&b, "工作区分组报告（无项目）\n")
		return b.String()
	}
	fmt.Fprintf(&b, "工作区分组报告（共 %d 个项目，%d 个栈）\n\n", r.ProjectCount, len(stacks))
	for _, s := range stacks {
		projs := groups[s]
		avg := averageHealth(projs)
		fmt.Fprintf(&b, "## %s（%d 个项目，平均健康 %d）\n", s, len(projs), avg)
		for _, p := range projs {
			fmt.Fprintf(&b, "  - %s  [health=%d]\n", p.Slug, p.HealthScore)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// averageHealth 计算一组项目的健康平均分（四舍五入到整数）。
// 空切片返回 0，避免除零。
func averageHealth(projs []ProjectView) int {
	if len(projs) == 0 {
		return 0
	}
	sum := 0
	for _, p := range projs {
		sum += p.HealthScore
	}
	return (sum + len(projs)/2) / len(projs) // 四舍五入
}

// sortStrings 对字符串切片原地升序排序（纯标准库，避免为单点用途引入 sort 包到上层）。
// 用插入排序：项目栈数量很小（几十以内），插入排序简洁且无额外分配。
func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func pad(s string, width int) string {
	// 中文宽度近似处理：一个中文字符算 2 列
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 127 {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func boolMark(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func formatStackSummary(m map[string]int) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%d", k, v))
	}
	if len(parts) == 0 {
		return "(无)"
	}
	return strings.Join(parts, " ")
}

// CalculateHealth 计算项目健康评分（0-100）。
// - has AGENTS.md: +20
// - has tests (test_count > 0): +30
// - git not dirty: +20
// - has build artifacts: +10
// - stack identified (not unknown): +20
func CalculateHealth(p ProjectView) int {
	score := 0
	if p.HasAgentsMD {
		score += 20
	}
	if p.TestCount != "" && p.TestCount != "0" {
		score += 30
	}
	if !p.GitDirty {
		score += 20
	}
	if p.StackPrimary != "" && p.StackPrimary != "unknown" {
		score += 20
	}
	if p.HasBuildArtifacts {
		score += 10
	}
	return score
}

// HasBuildArtifacts 从 ProjectView 推断（需要从 facts 填充）。
// 当前 ProjectView 没有直接字段，用 stack 推断辅助字段。
// 报告层用 Facts 里的 has_build_artifacts 设置。
