// Package workspace 实现工作区的项目发现：扫描根目录下的子目录，
// 识别哪些是"项目"（有项目标志文件），排除非项目目录（引擎源码副本/工具目录/依赖）。
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project 描述工作区里识别到的一个项目。
type Project struct {
	Slug string // 目录名（唯一标识）
	Path string // 绝对路径
}

// Discover 扫描 root 下的直接子目录，返回识别为项目的列表。
// ignoreDirs 里的目录名直接跳过；没有"项目标志文件"的目录也跳过。
//
// "项目标志文件"：存在以下任一即认为是项目（而非空目录/杂项）：
//   - 代码标志：go.mod / package.json / Cargo.toml / pyproject.toml / project.godot / pubspec.yaml
//   - 文档标志：AGENTS.md / README.md（根目录有这两个之一也认作内容项目）
//   - 网页标志：index.html
func Discover(root string, ignoreDirs []string) ([]Project, error) {
	ignore := make(map[string]bool, len(ignoreDirs))
	for _, d := range ignoreDirs {
		ignore[d] = true
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("读取工作区根目录失败 %s: %w", root, err)
	}

	var out []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// 跳过隐藏目录（.git / .portfolio 等）。
		if strings.HasPrefix(name, ".") {
			continue
		}
		if ignore[name] {
			continue
		}
		dirPath := filepath.Join(root, name)
		if !isProjectDir(dirPath) {
			continue
		}
		abs, err := filepath.Abs(dirPath)
		if err != nil {
			abs = dirPath
		}
		out = append(out, Project{Slug: name, Path: abs})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// isProjectDir 判断目录是否是项目（含至少一个标志文件）。
func isProjectDir(dir string) bool {
	for _, marker := range projectMarkers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// projectMarkers 是识别"这是一个项目"的标志文件名。
var projectMarkers = []string{
	// 代码标志
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"requirements.txt",
	"project.godot", // Godot 项目
	"pubspec.yaml",  // Flutter/Dart
	"tsconfig.json",
	// 文档/内容标志
	"AGENTS.md",
	"README.md",
	"readme.md",
	// 网页标志
	"index.html",
}
