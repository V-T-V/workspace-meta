// Package inspector 检查 workspace.Project 的各项属性，产出 Facts。
// 每个检查项对应一个方法，结果汇总到 Facts 的 KV 表里。
package inspector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QiuShichang/workspace-ops/internal/workspace"
)

// Facts 是一个项目的检查结果。键值对形式，便于入库（project_facts 表）和报告输出。
// 键的命名约定：<category>_<detail>，如 "stack_primary"、"go_version"、"test_count"。
type Facts struct {
	Slug string            // 项目 slug
	Path string            // 绝对路径
	KV   map[string]string // 检查结果（全转 string 便于存储/展示）
}

// Get 安全取值，不存在返回 ""。
func (f Facts) Get(key string) string {
	if f.KV == nil {
		return ""
	}
	return f.KV[key]
}

// Checks 控制哪些检查项启用（对应 config.ChecksConfig）。
type Checks struct {
	Stack          bool
	Dependencies   bool
	AgentsMD       bool
	GitStatus      bool
	Tests          bool
	BuildArtifacts bool
}

// Inspector 持有 git 命令路径（可注入），用于 git_status 检查。
// 如果 git 不可用，git_status 检查会优雅跳过。
type Inspector struct {
	Checks Checks
	gitCmd string // "git" 或注入的路径
}

// New 创建 Inspector。gitCmd 为空时默认 "git"。
func New(checks Checks, gitCmd string) *Inspector {
	if gitCmd == "" {
		gitCmd = "git"
	}
	return &Inspector{Checks: checks, gitCmd: gitCmd}
}

// Inspect 对单个项目跑所有启用的检查，返回 Facts。
func (in *Inspector) Inspect(p workspace.Project) Facts {
	f := Facts{Slug: p.Slug, Path: p.Path, KV: map[string]string{}}
	if in.Checks.Stack {
		in.inspectStack(p, &f)
	}
	if in.Checks.Dependencies {
		in.inspectDeps(p, &f)
	}
	if in.Checks.AgentsMD {
		in.inspectAgentsMD(p, &f)
	}
	if in.Checks.GitStatus {
		in.inspectGit(p, &f)
	}
	if in.Checks.Tests {
		in.inspectTests(p, &f)
	}
	if in.Checks.BuildArtifacts {
		in.inspectBuildArtifacts(p, &f)
	}
	return f
}

// inspectStack 识别技术栈（看标志文件）。
func (in *Inspector) inspectStack(p workspace.Project, f *Facts) {
	var stacks []string
	if exists(p.Path, "go.mod") {
		stacks = append(stacks, "go")
	}
	if exists(p.Path, "package.json") {
		stacks = append(stacks, "node/ts")
	}
	if exists(p.Path, "tsconfig.json") && !exists(p.Path, "package.json") {
		stacks = append(stacks, "ts")
	}
	if exists(p.Path, "Cargo.toml") {
		stacks = append(stacks, "rust")
	}
	if exists(p.Path, "pyproject.toml") || exists(p.Path, "requirements.txt") {
		stacks = append(stacks, "python")
	}
	if exists(p.Path, "project.godot") {
		stacks = append(stacks, "godot")
	}
	if exists(p.Path, "pubspec.yaml") {
		stacks = append(stacks, "flutter")
	}
	if exists(p.Path, "index.html") && !exists(p.Path, "package.json") {
		stacks = append(stacks, "html")
	}
	if len(stacks) == 0 {
		f.KV["stack_primary"] = "unknown"
		return
	}
	f.KV["stack_primary"] = stacks[0]
	f.KV["stack_all"] = strings.Join(stacks, ",")
}

// inspectDeps 读取依赖文件，提取关键信息。
func (in *Inspector) inspectDeps(p workspace.Project, f *Facts) {
	// go.mod: 提取 go 版本 + 启发式统计 require 依赖数
	if raw, err := os.ReadFile(filepath.Join(p.Path, "go.mod")); err == nil {
		f.KV["go_mod"] = "true"
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "go ") {
				f.KV["go_version"] = strings.TrimSpace(strings.TrimPrefix(line, "go "))
				break
			}
		}
		// go_dep_count_approx：启发式估算 require 块里的依赖数。
		// go.mod 中 require 块内的依赖行格式为以 tab 缩进开头的 "module version" 行，
		// 单行 require(...) 不计入。仅作近似估算，不引入 golang.org/x/mod/modfile。
		if reqCount := countGoModRequires(string(raw)); reqCount > 0 {
			f.KV["go_dep_count_approx"] = strconv.Itoa(reqCount)
		}
	}
	// package.json: 用 encoding/json 解析 name / engines.node / 依赖计数
	if raw, err := os.ReadFile(filepath.Join(p.Path, "package.json")); err == nil {
		f.KV["package_json"] = "true"
		var pkg packageJSON
		if err := json.Unmarshal(raw, &pkg); err == nil {
			if pkg.Name != "" {
				f.KV["npm_name"] = pkg.Name
			}
			if pkg.Engines.Node != "" {
				f.KV["node_version"] = pkg.Engines.Node
			}
			depCount := len(pkg.Dependencies) + len(pkg.DevDependencies)
			if depCount > 0 {
				f.KV["npm_dep_count"] = strconv.Itoa(depCount)
			}
		}
	}
	// Cargo.toml
	if exists(p.Path, "Cargo.toml") {
		f.KV["cargo_toml"] = "true"
	}
	// pyproject.toml
	if exists(p.Path, "pyproject.toml") {
		f.KV["pyproject"] = "true"
	}
}

// packageJSON 是 package.json 中 inspector 关心的字段子集。
type packageJSON struct {
	Name            string         `json:"name"`
	Engines         packageEngines `json:"engines"`
	Dependencies    map[string]any `json:"dependencies"`
	DevDependencies map[string]any `json:"devDependencies"`
}

// packageEngines 是 package.json 的 engines 子对象。
type packageEngines struct {
	Node string `json:"node"`
}

// countGoModRequires 启发式统计 go.mod 中 require 块里的依赖数。
// 同时支持单行 require 和 require ( ... ) 块两种形式。
func countGoModRequires(raw string) int {
	count := 0
	inBlock := false
	for _, line := range strings.Split(raw, "\n") {
		// 去掉行首空白便于判断
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		// require 块开始：require (
		if strings.HasPrefix(trimmed, "require") && strings.Contains(trimmed, "(") {
			inBlock = true
			continue
		}
		// 块结束
		if inBlock && trimmed == ")" {
			inBlock = false
			continue
		}
		if inBlock {
			// 块内每个非空、非注释行算一个依赖（行首通常以 tab 缩进）
			count++
			continue
		}
		// 单行 require：require module version
		if strings.HasPrefix(trimmed, "require ") {
			count++
		}
	}
	return count
}

// inspectAgentsMD 检查 AGENTS.md 是否存在。
func (in *Inspector) inspectAgentsMD(p workspace.Project, f *Facts) {
	if exists(p.Path, "AGENTS.md") {
		f.KV["has_agents_md"] = "true"
	} else {
		f.KV["has_agents_md"] = "false"
	}
}

// inspectGit 用 git CLI 查 branch 与 dirty 状态。
// git 不可用或非 git 仓库时优雅跳过（不写入 git_ 字段）。
func (in *Inspector) inspectGit(p workspace.Project, f *Facts) {
	branch, dirty, ok := gitStatus(in.gitCmd, p.Path)
	if !ok {
		return
	}
	f.KV["git_branch"] = branch
	f.KV["git_dirty"] = boolStr(dirty)
}

// walkSkipDirs 是 inspectTests 遍历时要整目录跳过的目录名集合。
// 这些目录要么体积巨大（依赖 / 缓存 / 构建产物），要么与测试文件统计无关，
// 跳过它们能把大项目（含 .next / coverage / .cache / out 等）的遍历耗时从秒级压到毫秒级。
var walkSkipDirs = map[string]bool{
	// 包管理器依赖（最大头）
	"node_modules": true,
	"vendor":       true,
	// VCS / 元数据
	".git": true,
	".svn": true,
	".hg":  true,
	// 构建产物 / 输出
	"dist":        true,
	"build":       true,
	"target":      true,
	"out":         true,
	"bin":         true,
	".next":       true, // Next.js 构建产物
	".nuxt":       true, // Nuxt 构建产物
	".output":     true, // Nitro/Nuxt 输出
	".svelte-kit": true, // SvelteKit 产物
	".turbo":      true, // Turborepo 缓存
	// 测试 / 覆盖率 / 缓存
	"coverage":      true,
	".cache":        true,
	".parcel-cache": true,
	"__pycache__":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
}

// inspectTests 启发式统计测试文件数（*.test.ts / *_test.go / test_*.py）。
func (in *Inspector) inspectTests(p workspace.Project, f *Facts) {
	count := 0
	_ = filepath.WalkDir(p.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if walkSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if isTestFile(d.Name()) {
			count++
		}
		return nil
	})
	f.KV["test_count"] = strconv.Itoa(count)
}

// inspectBuildArtifacts 检查常见构建产物目录是否存在。
func (in *Inspector) inspectBuildArtifacts(p workspace.Project, f *Facts) {
	var arts []string
	for _, dir := range []string{"dist", "bin", "build", "target", "out"} {
		if isDir(filepath.Join(p.Path, dir)) {
			arts = append(arts, dir)
		}
	}
	if len(arts) > 0 {
		f.KV["build_artifacts"] = strings.Join(arts, ",")
		f.KV["has_build_artifacts"] = "true"
	} else {
		f.KV["has_build_artifacts"] = "false"
	}
}

// ===== 辅助 =====

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// isTestFile 启发式判断是否是测试文件。
func isTestFile(name string) bool {
	// TS/JS: *.test.ts / *.spec.ts / *.test.js
	if strings.HasSuffix(name, ".test.ts") || strings.HasSuffix(name, ".spec.ts") ||
		strings.HasSuffix(name, ".test.js") || strings.HasSuffix(name, ".spec.js") {
		return true
	}
	// Go: *_test.go
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	// Python: test_*.py / *_test.py
	if strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py") {
		return true
	}
	if strings.HasSuffix(name, "_test.py") {
		return true
	}
	// Rust: tests/*.rs（粗略：文件名含 _test.rs 或在 tests/ 目录——这里只看文件名）
	if strings.HasSuffix(name, "_test.rs") {
		return true
	}
	return false
}

// InspectAll 批量检查多个项目，返回 Facts 切片。
func (in *Inspector) InspectAll(projects []workspace.Project) []Facts {
	out := make([]Facts, 0, len(projects))
	for _, p := range projects {
		out = append(out, in.Inspect(p))
	}
	return out
}

// Summary 返回 Facts 的简短摘要（用于日志）。
func (f Facts) Summary() string {
	parts := []string{f.Slug}
	if s := f.Get("stack_all"); s != "" {
		parts = append(parts, "stack="+s)
	} else if s := f.Get("stack_primary"); s != "" {
		parts = append(parts, "stack="+s)
	}
	if t := f.Get("test_count"); t != "" && t != "0" {
		parts = append(parts, "tests="+t)
	}
	if g := f.Get("git_branch"); g != "" {
		d := ""
		if f.Get("git_dirty") == "true" {
			d = "*"
		}
		parts = append(parts, fmt.Sprintf("git=%s%s", g, d))
	}
	return strings.Join(parts, " ")
}
