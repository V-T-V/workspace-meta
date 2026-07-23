// Package backup 实现数据备份与恢复。
// 对应原计划第二十节。使用 SQLite Online Backup API 保证一致性。
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manager 管理备份与恢复。
type Manager struct {
	db          *sql.DB
	sourcePath  string
	backupDir   string
	retainCount int
	log         *slog.Logger
}

// New 构造。
func New(db *sql.DB, sourcePath, backupDir string, retainCount int, log *slog.Logger) *Manager {
	if retainCount <= 0 {
		retainCount = 7
	}
	return &Manager{
		db: db, sourcePath: sourcePath, backupDir: backupDir,
		retainCount: retainCount, log: log,
	}
}

// Backup 执行一次备份。
// modernc.org/sqlite 不直接暴露 Online Backup API，改用文件级 checkpoint + 复制：
// 先执行 PRAGMA wal_checkpoint(TRUNCATE) 确保数据落盘，再复制 .db 文件。
func (m *Manager) Backup(ctx context.Context) (string, error) {
	if err := os.MkdirAll(m.backupDir, 0o755); err != nil {
		return "", fmt.Errorf("[backup] 创建备份目录失败: %w", err)
	}

	// checkpoint：把 WAL 写入主库
	if _, err := m.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return "", fmt.Errorf("[backup] WAL checkpoint 失败: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	dstName := fmt.Sprintf("assistant-%s.db", ts)
	dstPath := filepath.Join(m.backupDir, dstName)

	// 复制数据库文件
	if err := copyFile(m.sourcePath, dstPath); err != nil {
		return "", fmt.Errorf("[backup] 复制数据库失败: %w", err)
	}

	// 同时备份配置（若存在）
	configPath := filepath.Join(filepath.Dir(m.sourcePath), "..", "config.yaml")
	if absConfig, _ := filepath.Abs(configPath); absConfig != "" {
		if _, err := os.Stat(absConfig); err == nil {
			_ = copyFile(absConfig, filepath.Join(m.backupDir, "config-"+ts+".yaml"))
		}
	}

	// 清理过期备份
	removed := m.enforceRetention()
	m.log.Info("[backup] 备份完成", "path", dstPath, "removed", removed)
	return dstPath, nil
}

// enforceRetention 保留最近 retainCount 份备份，删除更早的。
func (m *Manager) enforceRetention() int {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		return 0
	}
	var backups []os.DirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "assistant-") && strings.HasSuffix(e.Name(), ".db") {
			backups = append(backups, e)
		}
	}
	if len(backups) <= m.retainCount {
		return 0
	}
	// 按名称排序（含时间戳，自然有序）
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() > backups[j].Name() // 新的在前
	})
	removed := 0
	for _, old := range backups[m.retainCount:] {
		oldPath := filepath.Join(m.backupDir, old.Name())
		if err := os.Remove(oldPath); err == nil {
			removed++
		}
	}
	return removed
}

// ListBackups 列出可用备份（按时间倒序）。
func (m *Manager) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []BackupInfo
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "assistant-") {
			continue
		}
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		out = append(out, BackupInfo{
			Name:    e.Name(),
			Path:    filepath.Join(m.backupDir, e.Name()),
			Size:    size,
			ModTime: time.Now(),
		})
		if info != nil {
			out[len(out)-1].ModTime = info.ModTime()
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

// BackupInfo 备份文件信息。
type BackupInfo struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
}

// copyFile 复制文件。
func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()
	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()
	_, err = io.Copy(dstF, srcF)
	return err
}
