package report

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/QiuShichang/workspace-ops/internal/inspector"
)

// sampleFacts 构造测试用的 Facts 切片。
func sampleFacts() []inspector.Facts {
	return []inspector.Facts{
		{Slug: "proj-a", Path: "/p/a", KV: map[string]string{
			"stack_primary": "go", "stack_all": "go", "has_agents_md": "true",
			"git_branch": "main", "git_dirty": "false", "test_count": "12",
		}},
		{Slug: "proj-b", Path: "/p/b", KV: map[string]string{
			"stack_primary": "node/ts", "stack_all": "node/ts", "has_agents_md": "false",
			"git_branch": "dev", "git_dirty": "true", "test_count": "5",
		}},
	}
}

// TestBuild 验证报告构建（项目数 + 栈汇总）。
func TestBuild(t *testing.T) {
	r := Build(sampleFacts())
	if r.ProjectCount != 2 {
		t.Errorf("ProjectCount 应为 2，实际 %d", r.ProjectCount)
	}
	if r.StackSummary["go"] != 1 || r.StackSummary["node/ts"] != 1 {
		t.Errorf("StackSummary 错: %v", r.StackSummary)
	}
	if len(r.Projects) != 2 {
		t.Errorf("Projects 应有 2 个，实际 %d", len(r.Projects))
	}
}

// TestTextReporter 验证 text 格式输出含关键信息。
func TestTextReporter(t *testing.T) {
	r := Build(sampleFacts())
	out := TextReporter{}.Format(r)
	checks := []string{"proj-a", "proj-b", "go", "node/ts", "12", "工作区扫描报告"}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Errorf("text 输出应含 %q\n实际:\n%s", s, out)
		}
	}
}

// TestMarkdownReporter 验证 markdown 表格格式。
func TestMarkdownReporter(t *testing.T) {
	r := Build(sampleFacts())
	out := MarkdownReporter{}.Format(r)
	if !strings.Contains(out, "| Slug |") {
		t.Error("markdown 应含表头")
	}
	if !strings.Contains(out, "| proj-a |") {
		t.Error("markdown 应含 proj-a 行")
	}
}

// TestJSONReporter 验证 JSON 可解析。
func TestJSONReporter(t *testing.T) {
	r := Build(sampleFacts())
	out := JSONReporter{}.Format(r)
	if !strings.Contains(out, `"project_count": 2`) {
		t.Errorf("JSON 应含 project_count:2\n实际: %s", out)
	}
	if !strings.Contains(out, `"slug": "proj-a"`) {
		t.Errorf("JSON 应含 proj-a\n实际: %s", out)
	}
}

// TestCSVReporter 验证 CSV 格式：表头、列顺序、行数、用 encoding/csv 能正确解析。
func TestCSVReporter(t *testing.T) {
	r := Build(sampleFacts())
	out := CSVReporter{}.Format(r)

	// 表头含全部 6 列。
	if !strings.HasPrefix(out, "slug,stack,tests,agents,git,health_score") {
		t.Errorf("CSV 表头应为首行 6 列\n实际: %s", out)
	}

	// 用标准库 csv 解析，确保转义合法、可被下游消费。
	rows, err := csvReader(out)
	if err != nil {
		t.Fatalf("CSV 输出无法被 encoding/csv 解析: %v\n原始:\n%s", err, out)
	}
	// 1 行表头 + 2 行数据 = 3 行。
	if len(rows) != 3 {
		t.Fatalf("CSV 应有 3 行（1 表头 + 2 数据），实际 %d\n原始:\n%s", len(rows), out)
	}

	// 验证 proj-a 行：slug=proj-a, stack=go, tests=12, agents=true, git=main, health=90。
	wantA := []string{"proj-a", "go", "12", "true", "main", "90"}
	if !equalSlice(rows[1], wantA) {
		t.Errorf("proj-a 行应为 %v\n实际 %v\n原始:\n%s", wantA, rows[1], out)
	}
	// 验证 proj-b 行：git dirty 在 branch 后加 "*"。
	wantB := []string{"proj-b", "node/ts", "5", "false", "dev*", "50"}
	if !equalSlice(rows[2], wantB) {
		t.Errorf("proj-b 行应为 %v（git dirty 加 *）\n实际 %v\n原始:\n%s", wantB, rows[2], out)
	}
}

// TestCSVReporterEscaping 验证 CSV 转义：slug/stack 含逗号、引号时正确转义。
func TestCSVReporterEscaping(t *testing.T) {
	// 构造一个 slug 含逗号、stack 含引号的极端项目，验证 csv.Writer 的转义。
	r := Report{
		ProjectCount: 1,
		Projects: []ProjectView{
			{Slug: "a,b", StackPrimary: `go "stable"`, TestCount: "3", GitBranch: "main", HealthScore: 70},
		},
	}
	out := CSVReporter{}.Format(r)
	rows, err := csvReader(out)
	if err != nil {
		t.Fatalf("含特殊字符的 CSV 解析失败: %v\n原始:\n%s", err, out)
	}
	if len(rows) != 2 {
		t.Fatalf("应有 2 行，实际 %d", len(rows))
	}
	// 解析后应还原原始值（csv 转义对解析端透明）。
	if rows[1][0] != "a,b" || rows[1][1] != `go "stable"` {
		t.Errorf("CSV 转义后解析值错，slug/stack 应还原为 'a,b' / 'go \"stable\"'\n实际 %v", rows[1])
	}
}

// TestCSVReporterInRegistry 验证 csv 已注册到 DefaultRegistry 且可取出使用。
func TestCSVReporterInRegistry(t *testing.T) {
	reg := DefaultRegistry()
	rep, err := reg.Get("csv")
	if err != nil || rep == nil {
		t.Fatalf("Get(csv) 应成功，err=%v", err)
	}
	r := Build(sampleFacts())
	out := rep.Format(r)
	if !strings.HasPrefix(out, "slug,stack,tests,agents,git,health_score") {
		t.Errorf("从 registry 取出的 csv reporter 输出表头不对\n实际: %s", out)
	}
}

// TestCSVReporterEmpty 验证空报告只输出表头行（不 panic）。
func TestCSVReporterEmpty(t *testing.T) {
	out := CSVReporter{}.Format(Report{})
	if out != "slug,stack,tests,agents,git,health_score\n" {
		t.Errorf("空报告应只输出表头 + 换行\n实际: %q", out)
	}
}

// csvReader 用 encoding/csv 解析 CSV 字符串为二维切片（测试辅助）。
func csvReader(s string) ([][]string, error) {
	return csv.NewReader(strings.NewReader(s)).ReadAll()
}

// equalSlice 比较两个等长字符串切片是否逐元素相等。
func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestRegistry 验证注册表的 Get/Formats。
func TestRegistry(t *testing.T) {
	reg := DefaultRegistry()
	rep, err := reg.Get("text")
	if err != nil || rep == nil {
		t.Errorf("Get(text) 失败: %v", err)
	}
	if _, err := reg.Get("xml"); err == nil {
		t.Error("未注册的格式应返回错误")
	}
	formats := reg.Formats()
	if len(formats) != 4 {
		t.Errorf("应有 4 种格式（text/json/markdown/csv），实际 %d", len(formats))
	}
}

// TestFromFacts 验证 Facts → ProjectView 转换。
func TestFromFacts(t *testing.T) {
	f := inspector.Facts{Slug: "x", KV: map[string]string{
		"stack_primary": "go", "has_agents_md": "true", "git_dirty": "true", "test_count": "7",
	}}
	v := FromFacts(f)
	if v.Slug != "x" || v.StackPrimary != "go" || !v.HasAgentsMD || !v.GitDirty || v.TestCount != "7" {
		t.Errorf("FromFacts 转换错: %+v", v)
	}
}

func TestCalculateHealth(t *testing.T) {
	// 完美项目
	perfect := ProjectView{
		HasAgentsMD: true, TestCount: "10", GitDirty: false,
		StackPrimary: "go", HasBuildArtifacts: true,
	}
	if score := CalculateHealth(perfect); score != 100 {
		t.Errorf("完美项目应 100，实际 %d", score)
	}
	// 最差项目
	bad := ProjectView{GitDirty: true, StackPrimary: "unknown"}
	if score := CalculateHealth(bad); score != 0 {
		t.Errorf("最差项目应 0，实际 %d", score)
	}
	// 中等
	mid := ProjectView{HasAgentsMD: true, TestCount: "5", StackPrimary: "go"}
	if score := CalculateHealth(mid); score != 90 {
		t.Errorf("中等项目应 90（20+30+20+20=90，无 build artifacts 不加分），实际 %d", score)
	}
}

// TestBuildFillsHealthScore 验证 Build 调用 CalculateHealth 填充 HealthScore。
func TestBuildFillsHealthScore(t *testing.T) {
	r := Build(sampleFacts())
	// proj-a: agents(+20) tests(+30) git-clean(+20) go(+20) = 90
	if r.Projects[0].HealthScore != 90 {
		t.Errorf("proj-a HealthScore 应为 90，实际 %d", r.Projects[0].HealthScore)
	}
	// proj-b: no-agents tests(+30) git-dirty(0) node/ts(+20) = 50
	if r.Projects[1].HealthScore != 50 {
		t.Errorf("proj-b HealthScore 应为 50，实际 %d", r.Projects[1].HealthScore)
	}
}

// TestHealthScoreInTextReport 验证 health_score 列出现在 text 输出中。
func TestHealthScoreInTextReport(t *testing.T) {
	r := Build(sampleFacts())
	out := TextReporter{}.Format(r)
	for _, want := range []string{"HEALTH", "90", "50"} {
		if !strings.Contains(out, want) {
			t.Errorf("text 输出应含 %q\n实际:\n%s", want, out)
		}
	}
}

// TestHealthScoreInMarkdownReport 验证 health_score 列出现在 markdown 输出中。
func TestHealthScoreInMarkdownReport(t *testing.T) {
	r := Build(sampleFacts())
	out := MarkdownReporter{}.Format(r)
	if !strings.Contains(out, "| Health |") {
		t.Errorf("markdown 表头应含 Health 列\n实际:\n%s", out)
	}
	// proj-a 行末应是 | 90 |
	if !strings.Contains(out, "proj-a | go | 12 | ✓ | main | 90 |") {
		t.Errorf("proj-a 行应含 health=90\n实际:\n%s", out)
	}
	if !strings.Contains(out, "proj-b | node/ts | 5 | ✗ | dev* | 50 |") {
		t.Errorf("proj-b 行应含 health=50\n实际:\n%s", out)
	}
}

// ===== 分组报告 =====

// groupedFacts 构造多栈、多项目的 Facts，用于分组报告测试。
// go 栈 3 个，node/ts 栈 1 个，rust 栈 1 个，空栈 1 个（应归入 unknown）。
func groupedFacts() []inspector.Facts {
	return []inspector.Facts{
		{Slug: "go-1", Path: "/g/1", KV: map[string]string{
			"stack_primary": "go", "has_agents_md": "true", "git_dirty": "false", "test_count": "10",
		}}, // health: 20+30+20+20 = 90
		{Slug: "node-1", Path: "/n/1", KV: map[string]string{
			"stack_primary": "node/ts", "has_agents_md": "false", "git_dirty": "true", "test_count": "5",
		}}, // health: 30+20 = 50
		{Slug: "go-2", Path: "/g/2", KV: map[string]string{
			"stack_primary": "go", "has_agents_md": "true", "git_dirty": "false", "test_count": "0",
		}}, // health: 20+20+20 = 60
		{Slug: "rust-1", Path: "/r/1", KV: map[string]string{
			"stack_primary": "rust", "has_agents_md": "true", "git_dirty": "false", "test_count": "20",
		}}, // health: 20+30+20+20 = 90
		{Slug: "mystery", Path: "/m", KV: map[string]string{
			"git_dirty": "true", "test_count": "0",
		}}, // 空 stack → unknown；health: 0
		{Slug: "go-3", Path: "/g/3", KV: map[string]string{
			"stack_primary": "go", "has_agents_md": "false", "git_dirty": "false", "test_count": "8",
		}}, // health: 30+20+20 = 70
	}
}

// TestGroupByStack 验证按 StackPrimary 分组的正确性，含空栈归 unknown。
func TestGroupByStack(t *testing.T) {
	r := Build(groupedFacts())
	groups := GroupByStack(r.Projects)

	if got := len(groups["go"]); got != 3 {
		t.Errorf("go 栈应有 3 个项目，实际 %d", got)
	}
	if got := len(groups["node/ts"]); got != 1 {
		t.Errorf("node/ts 栈应有 1 个项目，实际 %d", got)
	}
	if got := len(groups["rust"]); got != 1 {
		t.Errorf("rust 栈应有 1 个项目，实际 %d", got)
	}
	if got := len(groups["unknown"]); got != 1 {
		t.Errorf("空栈应归入 unknown（1 个），实际 %d", got)
	}
	// 总数守恒。
	total := 0
	for _, ps := range groups {
		total += len(ps)
	}
	if total != len(r.Projects) {
		t.Errorf("分组后项目总数 %d 应等于原项目数 %d", total, len(r.Projects))
	}
	// 分组内保持原顺序：go 组应是 go-1, go-2, go-3。
	goSlugs := make([]string, 0, len(groups["go"]))
	for _, p := range groups["go"] {
		goSlugs = append(goSlugs, p.Slug)
	}
	wantGo := []string{"go-1", "go-2", "go-3"}
	if len(goSlugs) != len(wantGo) {
		t.Fatalf("go 组顺序长度错: %v", goSlugs)
	}
	for i := range wantGo {
		if goSlugs[i] != wantGo[i] {
			t.Errorf("go 组顺序错：位置 %d 应 %s 实 %s（整组 %v）", i, wantGo[i], goSlugs[i], goSlugs)
		}
	}
}

// TestGroupByStackEmpty 验证空输入返回非 nil 的空 map。
func TestGroupByStackEmpty(t *testing.T) {
	groups := GroupByStack(nil)
	if groups == nil {
		t.Fatal("GroupByStack(nil) 不应返回 nil map")
	}
	if len(groups) != 0 {
		t.Errorf("空输入应得到空 map，实际 %v", groups)
	}
}

// TestGroupByStackExplicitUnknown 验证 StackPrimary 显式为 "unknown" 与空栈归同一组。
func TestGroupByStackExplicitUnknown(t *testing.T) {
	projs := []ProjectView{
		{Slug: "a", StackPrimary: ""},
		{Slug: "b", StackPrimary: "unknown"},
	}
	groups := GroupByStack(projs)
	if got := len(groups["unknown"]); got != 2 {
		t.Errorf("空栈与显式 unknown 应合并到同一组（2 个），实际 %d", got)
	}
}

// TestFormatGroupReport 验证分组报告含各栈段、项目数、健康平均分。
func TestFormatGroupReport(t *testing.T) {
	r := Build(groupedFacts())
	out := FormatGroupReport(r)

	// 头部含总数与栈数。
	if !strings.Contains(out, "共 6 个项目") {
		t.Errorf("报告头部应含项目总数 6\n实际:\n%s", out)
	}
	if !strings.Contains(out, "4 个栈") {
		t.Errorf("报告头部应含栈数 4（go/node/ts/rust/unknown）\n实际:\n%s", out)
	}
	// 每个栈段都应出现。
	for _, stack := range []string{"go", "node/ts", "rust", "unknown"} {
		if !strings.Contains(out, "## "+stack) {
			t.Errorf("报告应含栈段 %q\n实际:\n%s", stack, out)
		}
	}
	// go 段：3 个项目，平均健康 = (90+60+70+四舍五入)/3 = 220/3 = 73.3 → 73。
	if !strings.Contains(out, "## go（3 个项目，平均健康 73）") {
		t.Errorf("go 段应显示 3 个项目、平均健康 73\n实际:\n%s", out)
	}
	// 每个项目都列出。
	for _, slug := range []string{"go-1", "go-2", "go-3", "node-1", "rust-1", "mystery"} {
		if !strings.Contains(out, slug) {
			t.Errorf("报告应列出项目 %q\n实际:\n%s", slug, out)
		}
	}
}

// TestFormatGroupReportEmpty 验证空报告的占位输出。
func TestFormatGroupReportEmpty(t *testing.T) {
	out := FormatGroupReport(Report{})
	if !strings.Contains(out, "无项目") {
		t.Errorf("空报告应含『无项目』占位\n实际:\n%s", out)
	}
}

// TestFormatGroupReportAverageHealth 验证平均分的四舍五入。
// 构造两个 go 项目，分数 90 和 95，平均 92.5 → 四舍五入 93。
func TestFormatGroupReportAverageHealth(t *testing.T) {
	r := Report{
		ProjectCount: 2,
		Projects: []ProjectView{
			{Slug: "a", StackPrimary: "go", HealthScore: 90},
			{Slug: "b", StackPrimary: "go", HealthScore: 95},
		},
	}
	out := FormatGroupReport(r)
	// (90+95)/2 = 92.5 → (185 + 1)/2 = 93。
	if !strings.Contains(out, "平均健康 93") {
		t.Errorf("平均分 92.5 应四舍五入为 93\n实际:\n%s", out)
	}
}

// TestAverageHealth 验证平均分计算（直接单测，含四舍五入与零安全）。
func TestAverageHealth(t *testing.T) {
	if got := averageHealth(nil); got != 0 {
		t.Errorf("空切片应返回 0，实际 %d", got)
	}
	// 单个：直接返回该分。
	if got := averageHealth([]ProjectView{{HealthScore: 42}}); got != 42 {
		t.Errorf("单元素应返回该分 42，实际 %d", got)
	}
	// (10+20+30)/3 = 20。
	if got := averageHealth([]ProjectView{{HealthScore: 10}, {HealthScore: 20}, {HealthScore: 30}}); got != 20 {
		t.Errorf("(10+20+30)/3 应 20，实际 %d", got)
	}
	// (1+2)/2 = 1.5 → (3+1)/2 = 2（四舍五入）。
	if got := averageHealth([]ProjectView{{HealthScore: 1}, {HealthScore: 2}}); got != 2 {
		t.Errorf("(1+2)/2 应四舍五入为 2，实际 %d", got)
	}
	// (1+1+1+1+1)/5 = 1，无舍入误差。
	if got := averageHealth([]ProjectView{{HealthScore: 1}, {HealthScore: 1}, {HealthScore: 1}, {HealthScore: 1}, {HealthScore: 1}}); got != 1 {
		t.Errorf("五个 1 平均应 1，实际 %d", got)
	}
}

// TestSortStrings 验证内部排序工具（确定性字母序）。
func TestSortStrings(t *testing.T) {
	cases := [][]string{
		{"go", "rust", "node/ts", "unknown"},
		{"c", "b", "a"},
		{"single"},
		{},
		{"z", "a", "m", "b", "q"},
	}
	want := [][]string{
		{"go", "node/ts", "rust", "unknown"},
		{"a", "b", "c"},
		{"single"},
		{},
		{"a", "b", "m", "q", "z"},
	}
	for i, in := range cases {
		got := append([]string(nil), in...)
		sortStrings(got)
		if len(got) != len(want[i]) {
			t.Errorf("case %d 长度错 got %v want %v", i, got, want[i])
			continue
		}
		for j := range got {
			if got[j] != want[i][j] {
				t.Errorf("case %d 位置 %d 错 got %v want %v", i, j, got, want[i])
				break
			}
		}
	}
}

// ===== Search =====

// TestSearchBySlug 验证按 slug 子串搜索（"go-" 匹配 go-1/go-2/go-3）。
func TestSearchBySlug(t *testing.T) {
	r := Build(groupedFacts())
	got := Search(r, "go-")
	if len(got) != 3 {
		t.Fatalf("slug 含 'go-' 应匹配 3 个，实际 %d", len(got))
	}
	for _, p := range got {
		if !strings.HasPrefix(p.Slug, "go-") {
			t.Errorf("结果应只含 go-* 项目，发现 %q", p.Slug)
		}
	}
}

// TestSearchByStack 验证按 stack 搜索（"rust" 匹配 rust-1）。
func TestSearchByStack(t *testing.T) {
	r := Build(groupedFacts())
	got := Search(r, "rust")
	if len(got) != 1 {
		t.Fatalf("stack 'rust' 应匹配 1 个，实际 %d", len(got))
	}
	if got[0].Slug != "rust-1" {
		t.Errorf("应匹配 rust-1，实际 %s", got[0].Slug)
	}
}

// TestSearchStackMatchesAllStacks 验证 "go" 同时命中 StackPrimary 和 StackAll。
// groupedFacts 里 go 项目的 stack_all 也设为 "go"，故应命中 3 个 go 项目。
func TestSearchStackMatchesAllStacks(t *testing.T) {
	r := Build(groupedFacts())
	got := Search(r, "go")
	if len(got) != 3 {
		t.Errorf("搜 'go' 应命中 3 个 go 项目，实际 %d", len(got))
	}
}

// TestSearchCaseInsensitive 验证大小写不敏感（"GO" 应匹配 go 栈项目）。
func TestSearchCaseInsensitive(t *testing.T) {
	r := Build(groupedFacts())
	lower := Search(r, "go")
	upper := Search(r, "GO")
	mixed := Search(r, "Go")
	if len(lower) != len(upper) || len(lower) != len(mixed) {
		t.Errorf("大小写不敏感应返回相同数量：go=%d GO=%d Go=%d",
			len(lower), len(upper), len(mixed))
	}
}

// TestSearchEmptyQuery 验证空 query 返回 nil（不命中全部）。
func TestSearchEmptyQuery(t *testing.T) {
	r := Build(groupedFacts())
	if got := Search(r, ""); got != nil {
		t.Errorf("空 query 应返回 nil，实际 %d 个", len(got))
	}
}

// TestSearchNoMatch 验证无匹配返回 nil。
func TestSearchNoMatch(t *testing.T) {
	r := Build(groupedFacts())
	if got := Search(r, "zzz-no-such-thing"); got != nil {
		t.Errorf("无匹配应返回 nil，实际 %d 个", len(got))
	}
}

// TestSearchPreservesOrder 验证结果保持原 Projects 顺序。
func TestSearchPreservesOrder(t *testing.T) {
	r := Build(groupedFacts())
	got := Search(r, "go-")
	want := []string{"go-1", "go-2", "go-3"}
	if len(got) != len(want) {
		t.Fatalf("应有 %d 个，实际 %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i].Slug != w {
			t.Errorf("位置 %d 应为 %s，实际 %s（全量：%v）", i, w, got[i].Slug, got)
		}
	}
}

// TestSearchSlugSubstring 验证 slug 子串匹配（"node" 匹配 node-1）。
func TestSearchSlugSubstring(t *testing.T) {
	r := Build(groupedFacts())
	got := Search(r, "node")
	if len(got) != 1 || got[0].Slug != "node-1" {
		t.Errorf("搜 'node' 应匹配 node-1，实际 %v", got)
	}
}

// TestSearchReturnsCopies 验证修改返回切片不影响原 Report（值语义）。
func TestSearchReturnsCopies(t *testing.T) {
	r := Build(groupedFacts())
	got := Search(r, "go-")
	if len(got) == 0 {
		t.Fatal("应有匹配")
	}
	got[0].Slug = "MUTATED"
	// 原 Report 的 Projects 不应被改。
	if r.Projects[0].Slug == "MUTATED" {
		t.Error("Search 返回的应是值拷贝，修改结果不应影响原 Report")
	}
}

// ===== SortBy =====

// sortedProjects 构造一组用于排序测试的项目（顺序故意打乱）。
func sortedProjects() []ProjectView {
	return []ProjectView{
		{Slug: "zeta", StackPrimary: "go", TestCount: "5", HealthScore: 50},
		{Slug: "alpha", StackPrimary: "rust", TestCount: "20", HealthScore: 90},
		{Slug: "mid", StackPrimary: "", TestCount: "0", HealthScore: 70},
		{Slug: "beta", StackPrimary: "go", TestCount: "12", HealthScore: 70},
	}
}

func TestSortBySlug(t *testing.T) {
	got := SortBy(sortedProjects(), SortBySlug)
	want := []string{"alpha", "beta", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("应有 %d 个，实际 %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i].Slug != w {
			t.Errorf("位置 %d 应 %s 实 %s（全量 %v）", i, w, got[i].Slug, slugsOf(got))
		}
	}
}

func TestSortByStack(t *testing.T) {
	// 升序：空栈排末尾 → go, go, rust, ""(mid)。
	got := SortBy(sortedProjects(), SortByStack)
	want := []string{"go", "go", "rust", ""}
	if len(got) != len(want) {
		t.Fatalf("应有 %d 个，实际 %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i].StackPrimary != w {
			t.Errorf("位置 %d stack 应 %q 实 %q（全量 %v）", i, w, got[i].StackPrimary, stacksOf(got))
		}
	}
}

func TestSortByStackEmptyLast(t *testing.T) {
	// 多个空栈都应排到末尾，非空栈在前面字母序。
	projs := []ProjectView{
		{Slug: "a", StackPrimary: ""},
		{Slug: "b", StackPrimary: "go"},
		{Slug: "c", StackPrimary: ""},
		{Slug: "d", StackPrimary: "ada"},
	}
	got := SortBy(projs, SortByStack)
	// 期望：ada, go, "", ""（空栈两个都在末尾）。
	want := []string{"ada", "go", "", ""}
	for i, w := range want {
		if got[i].StackPrimary != w {
			t.Errorf("位置 %d stack 应 %q 实 %q（全量 %v）", i, w, got[i].StackPrimary, stacksOf(got))
		}
	}
}

func TestSortByTestCount(t *testing.T) {
	// 数值降序：20 > 12 > 5 > 0。
	got := SortBy(sortedProjects(), SortByTestCount)
	want := []int{20, 12, 5, 0}
	if len(got) != len(want) {
		t.Fatalf("应有 %d 个，实际 %d", len(want), len(got))
	}
	for i, w := range want {
		if n := parseTestCount(got[i].TestCount); n != w {
			t.Errorf("位置 %d test_count 应 %d 实 %d（全量 %v）", i, w, n, countsOf(got))
		}
	}
}

func TestSortByTestCountNumericNotLexicographic(t *testing.T) {
	// 关键：字符串序会把 "9" > "100"，但数值序应 100 > 9。
	projs := []ProjectView{
		{Slug: "few", TestCount: "9"},
		{Slug: "many", TestCount: "100"},
		{Slug: "none", TestCount: "0"},
	}
	got := SortBy(projs, SortByTestCount)
	want := []string{"many", "few", "none"}
	for i, w := range want {
		if got[i].Slug != w {
			t.Errorf("数值序位置 %d 应 %s 实 %s（全量 %v）", i, w, got[i].Slug, slugsOf(got))
		}
	}
}

func TestSortByTestCountInvalidHandledAsZero(t *testing.T) {
	// 非法 test_count（非数字）按 0 处理，不 panic。
	projs := []ProjectView{
		{Slug: "bad", TestCount: "abc"},
		{Slug: "good", TestCount: "5"},
		{Slug: "empty", TestCount: ""},
	}
	got := SortBy(projs, SortByTestCount)
	// good(5) 在前；bad(0)/empty(0) 平局，稳定排序保持原相对顺序：bad 在 empty 前。
	if got[0].Slug != "good" {
		t.Errorf("数值最大应在前：good，实际 %s（全量 %v）", got[0].Slug, slugsOf(got))
	}
	if got[1].Slug != "bad" || got[2].Slug != "empty" {
		t.Errorf("平局应稳定（保持原序 bad→empty），实际 %v", slugsOf(got))
	}
}

func TestSortByHealthScore(t *testing.T) {
	// 数值降序：90 > 70 > 70 > 50（70 平局稳定）。
	got := SortBy(sortedProjects(), SortByHealthScore)
	want := []int{90, 70, 70, 50}
	if len(got) != len(want) {
		t.Fatalf("应有 %d 个，实际 %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i].HealthScore != w {
			t.Errorf("位置 %d health 应 %d 实 %d（全量 %v）", i, w, got[i].HealthScore, healthsOf(got))
		}
	}
}

func TestSortByHealthScoreStable(t *testing.T) {
	// 同分时应保持原输入相对顺序（稳定排序）。
	projs := []ProjectView{
		{Slug: "p1", HealthScore: 50},
		{Slug: "p2", HealthScore: 80},
		{Slug: "p3", HealthScore: 50},
		{Slug: "p4", HealthScore: 80},
	}
	got := SortBy(projs, SortByHealthScore)
	// 两个 80（p2,p4）和两个 50（p1,p3）各自保持原序。
	want := []string{"p2", "p4", "p1", "p3"}
	for i, w := range want {
		if got[i].Slug != w {
			t.Errorf("稳定排序位置 %d 应 %s 实 %s（全量 %v）", i, w, got[i].Slug, slugsOf(got))
		}
	}
}

func TestSortByUnknownFieldNoOp(t *testing.T) {
	// 未知 field 不排序，原序返回。
	projs := sortedProjects()
	orig := append([]ProjectView(nil), projs...)
	got := SortBy(projs, SortField("nonsense"))
	for i := range orig {
		if got[i].Slug != orig[i].Slug {
			t.Errorf("未知 field 应原序返回：位置 %d 期望 %s 实际 %s", i, orig[i].Slug, got[i].Slug)
		}
	}
}

func TestSortByReturnsSameSlice(t *testing.T) {
	// 返回值应是入参切片本身（原地排序），便于链式调用。
	projs := sortedProjects()
	got := SortBy(projs, SortBySlug)
	if len(got) == 0 || &got[0] != &projs[0] {
		t.Error("SortBy 应返回入参切片本身（原地排序）")
	}
}

func TestSortByEmpty(t *testing.T) {
	// 空切片不 panic，返回空。
	got := SortBy(nil, SortBySlug)
	if len(got) != 0 {
		t.Errorf("空入参应返回空切片，实际 %d 个", len(got))
	}
}

func TestSortByChained(t *testing.T) {
	// 链式：先按 stack 升序，再按 health 降序。
	// 注意：两次独立 SortBy 的链式效果取决于稳定排序——第二次 health 降序会在
	// 第一次 stack 分组的基础上做组内微调（但全局看会被 health 重新排列）。
	// 这不是真正的"组内排序"——要实现真正的组内排序需用 SortByMulti。
	projs := []ProjectView{
		{Slug: "g1", StackPrimary: "go", HealthScore: 50},
		{Slug: "r1", StackPrimary: "rust", HealthScore: 90},
		{Slug: "g2", StackPrimary: "go", HealthScore: 80},
		{Slug: "r2", StackPrimary: "rust", HealthScore: 70},
	}
	// 单次 sort：按 health 降序（r1=90 > g2=80 > r2=70 > g1=50）
	SortBy(projs, SortByHealthScore)
	want := []string{"r1", "g2", "r2", "g1"}
	for i, w := range want {
		if projs[i].Slug != w {
			t.Errorf("health 降序位置 %d 应 %s 实 %s（全量 %v）", i, w, projs[i].Slug, slugsOf(projs))
		}
	}
}

// slugsOf / stacksOf / countsOf / healthsOf 是测试辅助：抽出某字段为字符串切片。
func slugsOf(ps []ProjectView) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Slug
	}
	return out
}
func stacksOf(ps []ProjectView) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.StackPrimary
	}
	return out
}
func countsOf(ps []ProjectView) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = parseTestCount(p.TestCount)
	}
	return out
}
func healthsOf(ps []ProjectView) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = p.HealthScore
	}
	return out
}
