package api

import (
	"net/http"
	"path/filepath"

	"github.com/QiuShichang/auto-finance-assistant/internal/backup"
)

// SetBackupManager 注入备份管理器（M9）。
func (s *Server) SetBackupManager(bm *backup.Manager) { s.backup = bm }

// handleBackup 触发一次备份（需认证）。
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if s.backup == nil {
		writeError(w, http.StatusServiceUnavailable, "backup_disabled", "备份未启用")
		return
	}
	path, err := s.backup.Backup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "备份失败，请查看服务端日志")
		return
	}
	// 只返回文件名，不泄露绝对路径
	name := filepath.Base(path)
	writeJSON(w, http.StatusOK, map[string]any{
		"name":   name,
		"status": "ok",
	})
}

// handleListBackups 列出备份（需认证）。
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	if s.backup == nil {
		writeError(w, http.StatusServiceUnavailable, "backup_disabled", "备份未启用")
		return
	}
	items, err := s.backup.ListBackups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, b := range items {
		out = append(out, map[string]any{
			"name":     b.Name,
			"size":     b.Size,
			"modTime":  b.ModTime.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}
