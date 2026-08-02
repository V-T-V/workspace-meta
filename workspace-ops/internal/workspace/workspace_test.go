package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscoverFindsProjects 验证：含标志文件的目录被识别为项目。
func TestDiscoverFindsProjects(t *testing.T) {
	root := t.TempDir()
	// 造 3 个项目目录（各放一个标志文件）+ 1 个空目录（非项目）
	mkdir(t, root, "go-proj", "go.mod")
	mkdir(t, root, "ts-proj", "package.json")
	mkdir(t, root, "godot-game", "project.godot")
	if err := os.Mkdir(filepath.Join(root, "empty-dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	projects, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Fatalf("应发现 3 个项目，实际 %d", len(projects))
	}
	// Discover 按 slug 升序返回；"go-proj" < "godot-game" < "ts-proj"（因 '-' 0x2d < 'd' 0x64）。
	want := []string{"go-proj", "godot-game", "ts-proj"}
	for i, p := range projects {
		if p.Slug != want[i] {
			t.Errorf("项目 %d: 想要 %s，实际 %s", i, want[i], p.Slug)
		}
	}
}

// TestDiscoverIgnoresDirs 验证：ignoreDirs 里的目录被跳过。
func TestDiscoverIgnoresDirs(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "real-proj", "go.mod")
	mkdir(t, root, "node_modules", "package.json") // 应被忽略
	mkdir(t, root, ".git", "config")               // 隐藏目录应被跳过

	projects, err := Discover(root, []string{"node_modules"})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Slug != "real-proj" {
		t.Fatalf("应只剩 real-proj，实际 %v", projects)
	}
}

// TestDiscoverAbsolutePath 验证返回的 Path 是绝对路径。
func TestDiscoverAbsolutePath(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "p", "go.mod")
	projects, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(projects[0].Path) {
		t.Errorf("Path 应为绝对路径，实际 %s", projects[0].Path)
	}
}

// TestIsProjectDir 验证各种标志文件的识别。
func TestIsProjectDir(t *testing.T) {
	root := t.TempDir()
	for _, marker := range projectMarkers {
		dir := filepath.Join(root, "d-"+marker)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, marker), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !isProjectDir(dir) {
			t.Errorf("含 %s 的目录应被识别为项目", marker)
		}
	}
}

// mkdir 在 root 下建一个名为 name 的目录，并写入一个空标志文件 marker。
func mkdir(t *testing.T, root, name, marker string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, marker), []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
}
