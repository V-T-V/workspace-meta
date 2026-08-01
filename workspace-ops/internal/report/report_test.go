package report

import (
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
	if len(formats) != 3 {
		t.Errorf("应有 3 种格式，实际 %d", len(formats))
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
