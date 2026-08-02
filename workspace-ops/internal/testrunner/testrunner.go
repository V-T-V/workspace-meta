// Package testrunner 实测各项目的测试执行（M2）。
//
// 与 inspector 的启发式 test_count（只数测试文件）不同，testrunner 真跑各项目的
// 测试命令，采集成败 + 耗时 + 输出摘要。
//
// 按技术栈选命令：
//   - go:     go test ./...（无 build tag，默认）
//   - node/ts: node --test（若 package.json 有 test 脚本则 npm test）
//   - python:  python -m unittest discover（或 pytest 若装了）
//   - rust:    cargo test（CGO 默认关）
//   - godot/flutter: 无标准测试命令，跳过
package testrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Result 是单个项目测试执行的结果。
type Result struct {
	Slug       string
	Stack      string // 检测到的技术栈
	Command    string // 实际跑的命令
	Status     string // pass / fail / skipped / timeout / error
	Duration   time.Duration
	OutputTail string // 输出末尾（失败时便于诊断，截断到 500 字符）
}

// Config 控制 testrunner 行为。
type Config struct {
	Timeout time.Duration // 单项目超时（默认 120s）
}

// DefaultConfig 返回默认配置（120s 超时）。
func DefaultConfig() Config {
	return Config{Timeout: 120 * time.Second}
}

// DetectCommand 按 stack 返回该项目应跑的测试命令（纯栈判断，不读文件）。
// 返回空串表示该栈无标准测试命令（应 skipped）。
//
// 用栈分隔符拆分匹配，避免子串误判（"godot" 不应匹配 "go"）。
// stack 形如 "go" / "node/ts" / "godot" / "python" / "rust"。
//
// 注意：对 node/ts 项目，本函数返回 "node --test"（Node 原生测试运行器）。
// 但很多 TS 项目用 vitest/jest（通过 npm test）。生产用法应优先用 DetectCommandFor，
// 它会检查 package.json 是否有 test 脚本，有则用 npm test。
func DetectCommand(stack string) string {
	return DetectCommandFor(stack, "")
}

// DetectCommandFor 按 stack + 项目目录返回测试命令（增强版）。
// 对 node/ts 项目：若 dir/package.json 有 "scripts"."test" 字段，返回 "npm test"，
// 否则降级 "node --test"。
func DetectCommandFor(stack, dir string) string {
	set := map[string]bool{}
	for _, s := range strings.FieldsFunc(stack, func(r rune) bool { return r == '/' || r == ',' || r == ' ' }) {
		set[s] = true
	}
	switch {
	case set["godot"], set["flutter"], set["html"]:
		return "" // 无标准测试命令
	case set["go"]:
		return "go test ./..."
	case set["node"], set["ts"]:
		// 优先用 package.json 里的 test 脚本（支持 vitest/jest/mocha）
		if hasNPMScript(dir, "test") {
			return "npm test"
		}
		return "node --test"
	case set["python"]:
		return "python -m unittest discover -s ."
	case set["rust"]:
		return "cargo test"
	}
	return ""
}

// Run 在 dir 目录跑测试命令，返回结果。
// slug/stack 用于标识。cfg 控制超时。
func Run(slug, stack, dir string, cfg Config) Result {
	cmd := DetectCommandFor(stack, dir)
	if cmd == "" {
		return Result{Slug: slug, Stack: stack, Status: "skipped"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	parts := splitArgs(cmd)
	if len(parts) == 0 {
		return Result{Slug: slug, Stack: stack, Status: "error", OutputTail: "命令解析失败: " + cmd}
	}

	start := time.Now()
	c := exec.CommandContext(ctx, parts[0], parts[1:]...)
	c.Dir = dir
	// 合并 stdout/stderr
	out, err := c.CombinedOutput()
	duration := time.Since(start)

	r := Result{
		Slug: slug, Stack: stack,
		Command: cmd, Duration: duration,
		OutputTail: tail(string(out), 500),
	}
	if ctx.Err() == context.DeadlineExceeded {
		r.Status = "timeout"
		return r
	}
	if err == nil {
		r.Status = "pass"
		return r
	}
	// 非零退出码：可能是测试失败，也可能命令本身错误（如 node 未装）
	if isCommandNotFound(err) {
		r.Status = "error"
		r.OutputTail = "命令不可用: " + err.Error()
		return r
	}
	r.Status = "fail"
	return r
}

// splitArgs 把命令字符串按空白切成 args 切片。
func splitArgs(cmd string) []string {
	return strings.Fields(cmd)
}

// tail 截取字符串末尾 n 字符。
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// isCommandNotFound 判断 error 是否是"命令不存在"（如 node/go 未装）。
// 跨平台处理。
func isCommandNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// exec.Error 的 "not found" 或 "no such file"
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no such file") {
		return true
	}
	// Windows: exit code 9009 通常表示命令不存在
	if exitErr, ok := err.(*exec.ExitError); ok {
		if runtime.GOOS == "windows" && exitErr.ExitCode() == 9009 {
			return true
		}
	}
	return false
}

// FormatSummary 把 Result 格式成单行摘要（用于报告）。
func FormatSummary(r Result) string {
	status := map[string]string{
		"pass": "✓", "fail": "✗", "skipped": "⊘", "timeout": "⏱", "error": "?",
	}[r.Status]
	if status == "" {
		status = "?"
	}
	if r.Status == "skipped" {
		return fmt.Sprintf("%s %s  %s (skipped)", status, r.Slug, r.Stack)
	}
	return fmt.Sprintf("%s %-22s %6s %8s  %s",
		status, r.Slug, r.Stack, r.Duration.Round(time.Millisecond), r.Command)
}

// hasNPMScript 检查 dir/package.json 是否定义了指定脚本（如 "test"）。
// 用于判断 node/ts 项目是否有自定义测试命令（vitest/jest/mocha）。
func hasNPMScript(dir, script string) bool {
	if dir == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return false
	}
	_, ok := pkg.Scripts[script]
	return ok
}
