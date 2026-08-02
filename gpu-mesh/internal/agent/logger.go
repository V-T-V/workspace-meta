package agent

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// === Agent 日志基础设施 ===
//
// 解决两个生产问题：
//  1. Windows 服务模式下 log.Printf 输出到虚空（无 stdout），运维看不到日志
//  2. 日志无限增长无轮转，长期运行撑爆磁盘
//
// 设计：
//   - 前台模式：输出到 stdout（开发可见）
//   - 服务模式：输出到文件（C:\Program Files\gpu-mesh\logs\agent.log），按天轮转
//   - 通过 SetupServiceLogger 切换

var (
	logMu      sync.Mutex
	logFile    *os.File
	logDateStr string // 当前日志文件日期（YYYY-MM-DD），用于轮转判断
)

// SetupServiceLogger 配置服务模式日志：输出到指定目录的 agent.log，按天轮转。
//
// 在 program.Start（服务模式）里调用。前台模式不调用，保持 stdout。
func SetupServiceLogger(logDir string) error {
	if logDir == "" {
		logDir = filepath.Join(os.Getenv("PROGRAMDATA"), "gpu-mesh", "logs")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	if err := rotateLogFile(logDir); err != nil {
		return err
	}
	// 定期检查轮转（每小时）
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := rotateLogFile(logDir); err != nil {
				// 轮转失败不致命，下次再试
				_ = err
			}
		}
	}()
	return nil
}

// rotateLogFile 按当天日期切换日志文件，并清理 7 天前的旧日志。
func rotateLogFile(logDir string) error {
	logMu.Lock()
	defer logMu.Unlock()

	today := time.Now().Format("2006-01-02")
	if logDateStr == today && logFile != nil {
		return nil // 已是当天文件，无需切换
	}

	// 关闭旧文件
	if logFile != nil {
		logFile.Close()
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("agent-%s.log", today))
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	logFile = f
	logDateStr = today

	// 同时输出到文件和 stdout（服务模式下 stdout 是虚空，但前台调试时有用）
	log.SetOutput(io.MultiWriter(os.Stdout, f))

	// 清理 7 天前的日志
	cleanOldLogs(logDir, 7)
	return nil
}

// cleanOldLogs 删除超过 keepDays 天的日志文件。
func cleanOldLogs(logDir string, keepDays int) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logDir, e.Name()))
		}
	}
}

// CloseLogger 关闭日志文件并重置状态（进程退出或测试结束时调用）。
func CloseLogger() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	logDateStr = ""
	// 恢复 log 输出到默认（stderr）
	log.SetOutput(os.Stderr)
}
