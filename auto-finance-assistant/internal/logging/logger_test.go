package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRotateWriter_BasicWrite 验证基本写入。
func TestRotateWriter_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	rw, err := NewRotateWriter(dir, "app", 1, 3) // 1MB
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()

	n, err := rw.Write([]byte("hello log\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 10 {
		t.Errorf("写了 %d 字节，预期 10", n)
	}

	// 验证文件存在
	content, _ := os.ReadFile(filepath.Join(dir, "app.log"))
	if !strings.Contains(string(content), "hello log") {
		t.Errorf("日志文件应包含内容，实际 %q", content)
	}
}

// TestRotateWriter_SizeRotation 验证超限轮转。
func TestRotateWriter_SizeRotation(t *testing.T) {
	dir := t.TempDir()
	// maxSize 设很小（100 字节），便于快速触发轮转
	rw, err := NewRotateWriter(dir, "app", 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	// 手动调小 maxSize 触发轮转
	rw.maxSize = 100

	// 写入超过 100 字节
	line := strings.Repeat("x", 80) + "\n"
	rw.Write([]byte(line)) // 81 字节
	rw.Write([]byte(line)) // 162 > 100 → 轮转

	// 应有归档文件
	entries, _ := os.ReadDir(dir)
	hasArchive := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "app-2") && strings.HasSuffix(e.Name(), ".log") {
			hasArchive = true
		}
	}
	if !hasArchive {
		t.Errorf("应有归档文件 app-*.log，实际 %v", entries)
	}
}

// TestRotateWriter_CleanupExcess 验证超量旧文件清理。
func TestRotateWriter_CleanupExcess(t *testing.T) {
	dir := t.TempDir()
	rw, err := NewRotateWriter(dir, "app", 1, 2) // 保留 2 个旧文件
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	rw.maxSize = 50 // 很小

	// 连续写入触发多次轮转，产生超过 maxFiles 的归档
	for i := 0; i < 10; i++ {
		rw.Write([]byte(strings.Repeat("y", 40) + "\n"))
	}

	// 验证归档数量 <= maxFiles
	entries, _ := os.ReadDir(dir)
	archiveCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "app-2") {
			archiveCount++
		}
	}
	if archiveCount > 2 {
		t.Errorf("归档文件应 <= 2，实际 %d", archiveCount)
	}
}

// TestRotateWriter_PurgeOlderThan 验证过期清理。
func TestRotateWriter_PurgeOlderThan(t *testing.T) {
	dir := t.TempDir()
	// 创建一个"旧的"归档文件
	oldPath := filepath.Join(dir, "app-20200101-000000.log")
	os.WriteFile(oldPath, []byte("old"), 0o644)
	// 修改时间为 100 天前
	oldTime := time.Now().AddDate(0, 0, -100)
	os.Chtimes(oldPath, oldTime, oldTime)

	// 创建一个"新的"归档
	newPath := filepath.Join(dir, "app-20990101-000000.log")
	os.WriteFile(newPath, []byte("new"), 0o644)

	rw, _ := NewRotateWriter(dir, "app", 1, 5)
	defer rw.Close()
	rw.PurgeOlderThan(30) // 清理 30 天前的

	// 旧的应被删，新的应保留
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("100天前的日志应被删除")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Error("新日志不应被删除")
	}
}

// TestNewWithFile 验证文件 logger 构造。
func TestNewWithFile(t *testing.T) {
	dir := t.TempDir()
	logger, rw, err := NewWithFile(dir, "info", 1, 3, 7)
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("logger 不应为 nil")
	}
	if rw == nil {
		t.Fatal("RotateWriter 不应为 nil")
	}
	defer rw.Close()

	logger.Info("test message")

	content, _ := os.ReadFile(filepath.Join(dir, "app.log"))
	if !strings.Contains(string(content), "test message") {
		t.Errorf("日志应包含消息")
	}
}

// TestNewWithFile_EmptyDir 验证空目录退化到 stderr。
func TestNewWithFile_EmptyDir(t *testing.T) {
	logger, rw, err := NewWithFile("", "info", 1, 3, 7)
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("logger 不应为 nil")
	}
	if rw != nil {
		t.Error("空目录应返回 nil RotateWriter")
	}
}
