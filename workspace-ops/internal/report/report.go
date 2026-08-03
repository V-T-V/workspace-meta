// Package report 把 inspector/storage 的结果格式化为多种输出（text/json/markdown）。
// 设计：Reporter 接口 + Registry，对齐 generic-admin/internal/export 的可插拔范式。
package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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

// DefaultRegistry 返回注册了 text/json/markdown/csv 四种格式的默认注册表。
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("text", TextReporter{})
	r.Register("json", JSONReporter{})
	r.Register("markdown", MarkdownReporter{})
	r.Register("csv", CSVReporter{})
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

// ===== CSV =====

// CSVReporter 输出 CSV 表格，便于外部工具（Excel/数据管道）消费。
//
// 列固定为：slug,stack,tests,agents,git,health_score。
//   - stack: 用 StackPrimary，空栈留空（与 text/markdown 的 "-" 占位不同，
//     CSV 里空值更便于下游解析，无需额外做 "-"→"" 转换）。
//   - agents: "true" / "false"（避免 ✓/✗ 这种非 ASCII 符号在 CSV 里造成编码麻烦）。
//   - git: "branch" 或 "branch*"（* 标 dirty，沿用本项目其他 reporter 的约定）。
//
// 用标准库 encoding/csv 处理引号/转义（如 slug 含逗号、引号、换行），
// 行尾统一 \n（csv.Writer 默认 \r\n，这里显式重置为 \n，与项目其他 reporter 一致）。
type CSVReporter struct{}

// Format 实现 Reporter。
func (CSVReporter) Format(r Report) string {
	var buf bytes.Buffer
	// 用 \n 行尾，避免 csv 默认的 \r\n 在跨工具消费时混入回车。
	w := csv.NewWriter(&buf)
	w.UseCRLF = false
	// Write 内部对字段做标准 CSV 转义（含逗号/引号/换行时自动加双引号）。
	_ = w.Write([]string{"slug", "stack", "tests", "agents", "git", "health_score"})
	for _, p := range r.Projects {
		stack := p.StackPrimary
		agents := "false"
		if p.HasAgentsMD {
			agents = "true"
		}
		tests := or(p.TestCount, "0")
		git := p.GitBranch
		if p.GitDirty {
			git += "*"
		}
		_ = w.Write([]string{
			p.Slug, stack, tests, agents, git,
			itoa(p.HealthScore),
		})
	}
	w.Flush()
	return buf.String()
}

// itoa 把 int 转成十进制字符串（纯标准库，避免为单点用途引入 strconv 到上层）。
// 与本项目已有 sortStrings 一样，用最朴素实现，不处理负数以外的格式问题。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
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

// Search 在报告中按关键字搜索项目（匹配 slug/stack）。
//
// 匹配规则：把 query 转小写后，在项目的 Slug、StackPrimary、StackAll 三个字段中
// 做子串匹配（大小写不敏感）。任一字段命中即纳入结果。
//
// 设计取舍：
//   - 只搜 slug/stack（不搜 facts）：facts 是开放键值集合，语义不可控，
//     贸然全字段搜会引入大量噪音；slug/stack 是项目最稳定的定位维度。
//   - 子串匹配 + 大小写不敏感：覆盖最常见的 "按语言查"（go）、
//     "按名字片段查"（auth）场景，零依赖、无需分词。
//   - 空 query 返回 nil：避免"空查询命中全部"造成意外（调用方若需全量可直接用 r.Projects）。
//   - 保持命中项在 r.Projects 里的原始顺序（确定性，便于断言）。
//
// 返回匹配到的 ProjectView 切片；无匹配返回 nil。
func Search(r Report, query string) []ProjectView {
	q := strings.ToLower(query)
	if q == "" {
		return nil
	}
	var out []ProjectView
	for _, p := range r.Projects {
		if strings.Contains(strings.ToLower(p.Slug), q) ||
			strings.Contains(strings.ToLower(p.StackPrimary), q) ||
			strings.Contains(strings.ToLower(p.StackAll), q) {
			out = append(out, p)
		}
	}
	return out
}

// SortField 是 SortBy 支持的排序维度。
type SortField string

const (
	SortBySlug        SortField = "slug"         // 按项目 slug 字母序
	SortByStack       SortField = "stack"        // 按 StackPrimary 字母序
	SortByTestCount   SortField = "test_count"   // 按测试数数值降序（多测的在前）
	SortByHealthScore SortField = "health_score" // 按健康分数值降序（健康的在前）
)

// SortBy 按 field 对 projects 原地排序，返回排序后的切片（即入参本身，便于链式用）。
//
// 各字段排序语义：
//   - slug / stack：字符串升序（A→Z），空值视为最大（排到末尾，避免空栈挤到前面）。
//   - test_count / health_score：数值降序（大值在前）；test_count 解析失败或为空按 0 处理。
//
// 同字段相等时保持稳定（用 sort.SliceStable），保留项目在输入里的相对顺序，
// 这样多次排序可叠加（如先按 stack 再按 health，得到"每栈内按健康降序"）。
//
// 未知 field 不排序（原样返回），便于调用方传用户输入而不必先校验。
func SortBy(projects []ProjectView, field SortField) []ProjectView {
	switch field {
	case SortBySlug:
		sort.SliceStable(projects, func(i, j int) bool {
			return projects[i].Slug < projects[j].Slug
		})
	case SortByStack:
		sort.SliceStable(projects, func(i, j int) bool {
			// 空栈排到末尾：用 (=="" 的额外项) 让空值更大。
			ei, ej := projects[i].StackPrimary == "", projects[j].StackPrimary == ""
			if ei != ej {
				return !ei // 非空的（false）排前
			}
			return projects[i].StackPrimary < projects[j].StackPrimary
		})
	case SortByTestCount:
		sort.SliceStable(projects, func(i, j int) bool {
			return parseTestCount(projects[i].TestCount) > parseTestCount(projects[j].TestCount)
		})
	case SortByHealthScore:
		sort.SliceStable(projects, func(i, j int) bool {
			return projects[i].HealthScore > projects[j].HealthScore
		})
	}
	return projects
}

// parseTestCount 把 test_count 字符串解析为整数，空/非法返回 0。
// 与 CalculateHealth 把 test_count 当"有无"判断不同，这里要数值用于排序。
func parseTestCount(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	if n < 0 {
		return 0
	}
	return n
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

// CountByStack 统计每个技术栈的项目数。
func CountByStack(projects []ProjectView) map[string]int {
	out := map[string]int{}
	for _, p := range projects {
		stack := p.StackPrimary
		if stack == "" {
			stack = "unknown"
		}
		out[stack]++
	}
	return out
}
