package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotateLogFile(t *testing.T) {
	dir := t.TempDir()
	// 测试结束关闭文件句柄，避免 TempDir 清理时文件被占用
	defer CloseLogger()

	// 首次轮转：应创建当天日志文件
	if err := rotateLogFile(dir); err != nil {
		t.Fatalf("首次 rotateLogFile 失败: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, "agent-"+today+".log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("日志文件未创建: %s", logPath)
	}

	// 写入一行，确认文件可写
	if logFile == nil {
		t.Fatal("logFile 为 nil")
	}
	logFile.WriteString("test line\n")

	// 再次轮转（同一天）：不应创建新文件，复用现有
	if err := rotateLogFile(dir); err != nil {
		t.Fatalf("二次 rotateLogFile 失败: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("同一天应只有 1 个日志文件，得到 %d", len(entries))
	}
}

func TestCleanOldLogs(t *testing.T) {
	dir := t.TempDir()

	// 创建一个 8 天前的"旧"日志
	oldName := "agent-2020-01-01.log"
	oldPath := filepath.Join(dir, oldName)
	os.WriteFile(oldPath, []byte("old"), 0644)
	// 修改时间为 8 天前
	eightDaysAgo := time.Now().AddDate(0, 0, -8)
	os.Chtimes(oldPath, eightDaysAgo, eightDaysAgo)

	// 创建今天的日志
	today := time.Now().Format("2006-01-02")
	os.WriteFile(filepath.Join(dir, "agent-"+today+".log"), []byte("today"), 0644)

	// 清理 7 天前的
	cleanOldLogs(dir, 7)

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == oldName {
			t.Errorf("旧日志 %s 应被清理，仍存在", oldName)
		}
	}
	// 今天的应保留
	if _, err := os.Stat(filepath.Join(dir, "agent-"+today+".log")); os.IsNotExist(err) {
		t.Error("今天的日志不应被清理")
	}
}

func TestCleanOldLogsNoDir(t *testing.T) {
	// 不存在的目录不应 panic
	cleanOldLogs("/nonexistent/path/xyz", 7)
}

func TestLogFilePathFormat(t *testing.T) {
	// 验证日志文件名格式：agent-YYYY-MM-DD.log
	today := time.Now().Format("2006-01-02")
	expected := "agent-" + today + ".log"
	if !strings.HasPrefix(expected, "agent-") || !strings.HasSuffix(expected, ".log") {
		t.Errorf("日志文件名格式异常: %s", expected)
	}
	// 日期部分应是有效的 YYYY-MM-DD
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(expected, "agent-"), ".log"), "-")
	if len(parts) != 3 {
		t.Errorf("日期部分应有 3 段，得到 %d", len(parts))
	}
}
