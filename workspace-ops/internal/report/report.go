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
	Slug         string            `json:"slug"`
	StackPrimary string            `json:"stack_primary,omitempty"`
	StackAll     string            `json:"stack_all,omitempty"`
	HasAgentsMD  bool              `json:"has_agents_md"`
	GitBranch    string            `json:"git_branch,omitempty"`
	GitDirty     bool              `json:"git_dirty"`
	TestCount    string            `json:"test_count,omitempty"`
	TestStatus   string            `json:"test_status,omitempty"`   // 最近一次实跑测试状态：pass/fail/skipped/timeout/error
	TestDuration string            `json:"test_duration,omitempty"` // 测试耗时（毫秒，人类可读）
	Facts        map[string]string `json:"facts,omitempty"`
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
func Build(facts []inspector.Facts) Report {
	r := Report{
		Projects:     make([]ProjectView, 0, len(facts)),
		StackSummary: map[string]int{},
	}
	for _, f := range facts {
		v := FromFacts(f)
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
	fmt.Fprintln(&b, pad("SLUG", 28), pad("STACK", 12), pad("TESTS", 6), pad("AGENTS", 7), pad("GIT", 20))
	fmt.Fprintln(&b, strings.Repeat("-", 75))
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
	fmt.Fprintln(&b, "| Slug | Stack | Tests | AGENTS.md | Git |")
	fmt.Fprintln(&b, "|------|-------|-------|-----------|-----|")
	for _, p := range r.Projects {
		git := p.GitBranch
		if p.GitDirty {
			git += "*"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			p.Slug, or(p.StackPrimary, "-"), or(p.TestCount, "0"),
			boolMark(p.HasAgentsMD), or(git, "-"))
	}
	return b.String()
}

// ===== 辅助 =====

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
