package testrunner

import (
	"os"
	"testing"
	"time"
)

func writeFileBytes(path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

func TestDetectCommand(t *testing.T) {
	cases := []struct {
		stack, want string
	}{
		{"go", "go test ./..."},
		{"node/ts", "node --test"},
		{"ts", "node --test"},
		{"python", "python -m unittest discover -s ."},
		{"rust", "cargo test"},
		{"godot", ""},
		{"flutter", ""},
		{"unknown", ""},
	}
	for _, c := range cases {
		got := DetectCommand(c.stack)
		if got != c.want {
			t.Errorf("DetectCommand(%q) = %q, want %q", c.stack, got, c.want)
		}
	}
}

func TestRunSkippedNoCommand(t *testing.T) {
	// godot 栈无测试命令，应 skipped
	r := Run("some-godot-game", "godot", "/tmp", DefaultConfig())
	if r.Status != "skipped" {
		t.Errorf("godot 应 skipped，实际 %s", r.Status)
	}
}

func TestRunGoSelfTest(t *testing.T) {
	// 这个测试真跑 go test（含编译），较慢且受环境速度影响。
	// testing.Short 模式（go test -short）跳过。
	if testing.Short() {
		t.Skip("跳过慢测试（真跑 go test）")
	}
	// 建一个临时 go 项目（含 go.mod + 一个通过的测试），跑 go test 应 pass
	dir := t.TempDir()
	osWriteFile(t, dir+"/go.mod", "module testproj\n\ngo 1.25\n")
	osWriteFile(t, dir+"/main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")
	r := Run("testproj", "go", dir, Config{Timeout: 90 * time.Second})
	if r.Status != "pass" {
		t.Errorf("临时 go 项目应 pass，实际 %s（tail: %s）", r.Status, r.OutputTail)
	}
	if r.Duration <= 0 {
		t.Error("Duration 应 > 0")
	}
}

func osWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := writeFileBytes(path, []byte(content)); err != nil {
		t.Fatal(err)
	}
}

func TestRunGoFailingTest(t *testing.T) {
	// 跑一个不存在的目录，应 error 或 fail（go test 找不到包）
	r := Run("nonexistent", "go", "/nonexistent/path/xyz", Config{Timeout: 10 * time.Second})
	if r.Status != "error" && r.Status != "fail" {
		t.Errorf("不存在目录应 error/fail，实际 %s", r.Status)
	}
}

func TestFormatSummary(t *testing.T) {
	r := Result{Slug: "p", Stack: "go", Status: "pass", Duration: 1234 * time.Millisecond, Command: "go test"}
	s := FormatSummary(r)
	if s == "" {
		t.Error("FormatSummary 不应为空")
	}
	// 应含 slug 和 pass 标记
	if !contains(s, "p") {
		t.Errorf("summary 应含 slug，实际 %q", s)
	}
	// skipped 的格式
	rs := Result{Slug: "game", Stack: "godot", Status: "skipped"}
	if FormatSummary(rs) == "" {
		t.Error("skipped summary 不应为空")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
