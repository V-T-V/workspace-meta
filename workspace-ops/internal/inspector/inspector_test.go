package inspector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QiuShichang/workspace-ops/internal/workspace"
)

// allChecks 返回全开的 Checks，便于测试。
func allChecks() Checks {
	return Checks{Stack: true, Dependencies: true, AgentsMD: true, GitStatus: false, Tests: true, BuildArtifacts: true}
}

// TestInspectStack 验证技术栈识别。
func TestInspectStack(t *testing.T) {
	root := t.TempDir()
	goProj := mkdir(t, root, "go-p", "go.mod", "go 1.25.6\n")
	tsProj := mkdir(t, root, "ts-p", "package.json", `{"name":"x"}`)

	insp := New(allChecks(), "git")
	f := insp.Inspect(workspace.Project{Slug: "go-p", Path: goProj})
	if f.Get("stack_primary") != "go" {
		t.Errorf("go-proj stack 应为 go，实际 %q", f.Get("stack_primary"))
	}
	if f.Get("go_version") != "1.25.6" {
		t.Errorf("go_version 应为 1.25.6，实际 %q", f.Get("go_version"))
	}

	f2 := insp.Inspect(workspace.Project{Slug: "ts-p", Path: tsProj})
	if f2.Get("stack_primary") != "node/ts" {
		t.Errorf("ts-proj stack 应为 node/ts，实际 %q", f2.Get("stack_primary"))
	}
	if f2.Get("npm_name") != "x" {
		t.Errorf("npm_name 应为 x，实际 %q", f2.Get("npm_name"))
	}
}

// TestInspectAgentsMD 验证 AGENTS.md 检测。
func TestInspectAgentsMD(t *testing.T) {
	root := t.TempDir()
	with := mkdir(t, root, "with", "go.mod", "")
	without := mkdir(t, root, "without", "go.mod", "")

	// with 目录写一个 AGENTS.md
	if err := os.WriteFile(filepath.Join(with, "AGENTS.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}

	insp := New(allChecks(), "git")
	if insp.Inspect(workspace.Project{Slug: "with", Path: with}).Get("has_agents_md") != "true" {
		t.Error("with 应有 AGENTS.md")
	}
	if insp.Inspect(workspace.Project{Slug: "without", Path: without}).Get("has_agents_md") != "false" {
		t.Error("without 应无 AGENTS.md")
	}
}

// TestInspectTests 验证测试文件计数（含跳过 node_modules）。
func TestInspectTests(t *testing.T) {
	root := t.TempDir()
	proj := mkdir(t, root, "p", "go.mod", "")
	// 2 个 go 测试文件
	writeFile(t, filepath.Join(proj, "a_test.go"), "")
	writeFile(t, filepath.Join(proj, "b_test.go"), "")
	// 1 个 ts 测试文件（在 node_modules 里，应被跳过）
	nmDir := filepath.Join(proj, "node_modules", "lib")
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nmDir, "skip.test.ts"), "")

	insp := New(allChecks(), "git")
	f := insp.Inspect(workspace.Project{Slug: "p", Path: proj})
	if f.Get("test_count") != "2" {
		t.Errorf("test_count 应为 2（跳过 node_modules），实际 %q", f.Get("test_count"))
	}
}

// TestInspectBuildArtifacts 验证构建产物检测。
func TestInspectBuildArtifacts(t *testing.T) {
	root := t.TempDir()
	proj := mkdir(t, root, "p", "go.mod", "")
	if err := os.Mkdir(filepath.Join(proj, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	insp := New(allChecks(), "git")
	f := insp.Inspect(workspace.Project{Slug: "p", Path: proj})
	if f.Get("has_build_artifacts") != "true" {
		t.Error("应检测到 bin/ 构建产物")
	}
	if f.Get("build_artifacts") != "bin" {
		t.Errorf("build_artifacts 应含 bin，实际 %q", f.Get("build_artifacts"))
	}
}

// TestInspectDeps 验证依赖解析（含嵌套 engines.node、依赖计数、go.mod require 计数）。
// 这覆盖了原 extractJSONString / countJSONKeys 被删除后的逻辑。
func TestInspectDeps(t *testing.T) {
	root := t.TempDir()
	// package.json：含嵌套 engines.node + dependencies + devDependencies
	tsProj := mkdir(t, root, "ts-p", "package.json", `{
		"name": "my-app",
		"version": "1.0.0",
		"engines": {"node": ">=20"},
		"dependencies": {"vue": "^3.4.0", "pinia": "^2.1.0"},
		"devDependencies": {"vite": "^5.0.0"}
	}`)
	insp := New(allChecks(), "git")
	f := insp.Inspect(workspace.Project{Slug: "ts-p", Path: tsProj})
	if f.Get("npm_name") != "my-app" {
		t.Errorf("npm_name 应为 my-app，实际 %q", f.Get("npm_name"))
	}
	// 关键断言：嵌套 engines.node 现在能正确提取（修复 S1 bug）
	if f.Get("node_version") != ">=20" {
		t.Errorf("node_version 应为 >=20（嵌套解析），实际 %q", f.Get("node_version"))
	}
	if got := f.Get("npm_dep_count"); got != "3" {
		t.Errorf("npm_dep_count 应为 3（2 deps + 1 devDep），实际 %q", got)
	}

	// go.mod：require 块 + 单行 require 混合
	goProj := mkdir(t, root, "go-p", "go.mod", `module example.com/p

go 1.25.6

require (
	github.com/foo/bar v1.2.3
	github.com/baz/qux v0.4.5
	golang.org/x/text v0.14.0
)

require github.com/solo/dep v1.0.0
`)
	fg := insp.Inspect(workspace.Project{Slug: "go-p", Path: goProj})
	if fg.Get("go_version") != "1.25.6" {
		t.Errorf("go_version 应为 1.25.6，实际 %q", fg.Get("go_version"))
	}
	// require 块 3 行 + 单行 require 1 个 = 4
	if got := fg.Get("go_dep_count_approx"); got != "4" {
		t.Errorf("go_dep_count_approx 应为 4（3 块内 + 1 单行），实际 %q", got)
	}
}

// TestInspectAll 验证批量检查。
func TestInspectAll(t *testing.T) {
	root := t.TempDir()
	p1 := mkdir(t, root, "p1", "go.mod", "")
	p2 := mkdir(t, root, "p2", "package.json", `{"name":"y"}`)
	insp := New(allChecks(), "git")
	facts := insp.InspectAll([]workspace.Project{
		{Slug: "p1", Path: p1},
		{Slug: "p2", Path: p2},
	})
	if len(facts) != 2 {
		t.Fatalf("应返回 2 个 facts，实际 %d", len(facts))
	}
}

// mkdir 建目录并写一个标志文件（含内容）。
func mkdir(t *testing.T, root, name, marker, content string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if marker != "" {
		writeFile(t, filepath.Join(dir, marker), content)
	}
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
