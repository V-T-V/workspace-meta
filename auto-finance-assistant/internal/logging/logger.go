// Package logging 提供结构化日志（JSON）+ 文件滚动 + 按大小/天数自动清理。
// 对应原计划第十九节。
//
// 滚动策略：
//   - 单文件达到 maxSize（默认 20MB）时轮转：app.log → app-001.log → app-002.log ...
//   - 最多保留 maxFiles 个旧文件（默认按 retainDays × 24h 估算）
//   - 启动时清理超过 retainDays 天的旧日志
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RotateWriter 是带文件大小轮转的 io.Writer。
// 同时写当前日志文件 + stderr（终端可见）。
type RotateWriter struct {
	mu        sync.Mutex
	dir       string // 日志目录
	baseName  string // 基础文件名，如 "app"
	ext       string // 扩展名，如 ".log"
	maxSize   int64  // 单文件最大字节
	maxFiles  int    // 保留的旧文件数（不含当前）
	currentFD *os.File
	currentSz int64
	stderr    *os.File
}

// NewRotateWriter 构造。dir 为日志目录，maxSizeMB 为单文件大小上限。
func NewRotateWriter(dir, baseName string, maxSizeMB int, maxFiles int) (*RotateWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("[logging] 创建日志目录失败: %w", err)
	}
	if maxSizeMB <= 0 {
		maxSizeMB = 20
	}
	if maxFiles <= 0 {
		maxFiles = 7
	}
	// 解析扩展名：如果传入 "app.log"，ext=".log", stem="app"；如果 "app"，ext=".log", stem="app"
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	if ext == "" {
		ext = ".log"
	}

	w := &RotateWriter{
		dir:      dir,
		baseName: stem,
		ext:      ext,
		maxSize:  int64(maxSizeMB) * 1024 * 1024,
		maxFiles: maxFiles,
		stderr:   os.Stderr,
	}

	// 打开当前日志文件（追加模式）
	path := filepath.Join(dir, stem+ext)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("[logging] 打开日志文件失败: %w", err)
	}
	info, _ := f.Stat()
	w.currentFD = f
	if info != nil {
		w.currentSz = info.Size()
	}

	return w, nil
}

// Write 实现 io.Writer。写文件 + stderr，超限轮转。
func (w *RotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 写 stderr（终端可见，即使文件写入失败也不丢）
	_, _ = w.stderr.Write(p)

	// 检查是否需要轮转
	if w.currentFD != nil && w.currentSz+int64(len(p)) > w.maxSize {
		w.rotateLocked()
	}

	// 文件 FD 可能因轮转失败变 nil → 退化到只写 stderr
	if w.currentFD == nil {
		return len(p), nil // 告诉调用方全部"写入"成功（stderr 已写）
	}

	n, err := w.currentFD.Write(p)
	w.currentSz += int64(n)
	return n, err
}

// rotateLocked 执行轮转（调用方已持锁）。
func (w *RotateWriter) rotateLocked() {
	if w.currentFD != nil {
		_ = w.currentFD.Close()
		w.currentFD = nil // 标记为关闭，防止后续写入已关闭的 FD
	}

	// 当前文件重命名为带时间戳的归档
	ts := time.Now().Format("20060102-150405.000") // 含毫秒，防同秒覆盖
	archivePath := filepath.Join(w.dir, fmt.Sprintf("%s-%s%s", w.baseName, ts, w.ext))
	currentPath := filepath.Join(w.dir, w.baseName+w.ext)
	if err := os.Rename(currentPath, archivePath); err != nil {
		// rename 失败（可能被占用）：尝试删除旧文件内容重新开始
		_ = os.Remove(currentPath)
	}

	// 清理超量的旧文件
	w.cleanupLocked()

	// 创建新文件
	f, err := os.OpenFile(currentPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// 无法创建新文件 → currentFD 保持 nil，Write 退化到只写 stderr
		return
	}
	w.currentFD = f
	// stat 新文件获取真实大小（rename 失败时文件可能仍有残留内容）
	if info, err := f.Stat(); err == nil {
		w.currentSz = info.Size()
	} else {
		w.currentSz = 0
	}
}

// cleanupLocked 删除超过 maxFiles 数量的最旧归档。
func (w *RotateWriter) cleanupLocked() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	// 收集归档文件（匹配 baseName-*.ext 模式）
	prefix := w.baseName + "-"
	var archives []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, w.ext) {
			archives = append(archives, filepath.Join(w.dir, name))
		}
	}
	// 按名称排序（时间戳有序，旧的在后）
	sort.Strings(archives)
	// 删除超量的（从最旧开始）
	excess := len(archives) - w.maxFiles
	for i := 0; i < excess; i++ {
		_ = os.Remove(archives[i])
	}
}

// Close 关闭当前文件。
func (w *RotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentFD != nil {
		return w.currentFD.Close()
	}
	return nil
}

// PurgeOlderThan 删除超过 maxAge 天的归档日志文件。
// 在启动时调用。
func (w *RotateWriter) PurgeOlderThan(maxAgeDays int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if maxAgeDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	prefix := w.baseName + "-"
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// New 根据 level 构造 slog.Logger。
// 如果 directory 非空，同时写文件（带轮转）+ stderr。
// 否则仅写 stderr。
func New(level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}

// NewWithFile 构造写文件（带轮转）+ stderr 的 Logger。
// dir: 日志目录, level: 日志级别, maxSizeMB: 单文件上限, maxFiles: 保留旧文件数, retainDays: 清理超 N 天的归档。
func NewWithFile(dir, level string, maxSizeMB, maxFiles, retainDays int) (*slog.Logger, *RotateWriter, error) {
	if dir == "" {
		return New(level), nil, nil
	}
	rw, err := NewRotateWriter(dir, "app", maxSizeMB, maxFiles)
	if err != nil {
		// 文件创建失败时退化到 stderr
		return New(level), nil, nil
	}
	// 启动时清理过期日志
	rw.PurgeOlderThan(retainDays)
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	logger := slog.New(slog.NewJSONHandler(rw, opts))
	return logger, rw, nil
}

// NewWithWriter 用指定 writer（供测试）。
func NewWithWriter(w io.Writer, level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewJSONHandler(w, opts))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
