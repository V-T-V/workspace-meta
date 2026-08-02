package chat

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// complianceLogger 合规日志收集器。
// 统一记录所有输入/输出/拦截事件，构建完整证据链。
// 满足 GB/T 45654-2025 安全评估的"全链路可追溯"要求。
type complianceLogger struct {
	db  *sql.DB
	log *slog.Logger
}

func newComplianceLogger(db *sql.DB, log *slog.Logger) *complianceLogger {
	return &complianceLogger{db: db, log: log}
}

// record 异步写入合规日志（不阻塞主流程）。
func (cl *complianceLogger) record(entry *storage.ComplianceLog) {
	if cl == nil || cl.db == nil {
		return
	}
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := storage.CreateComplianceLog(ctx, cl.db, entry); err != nil {
			cl.log.Error("[compliance] 合规日志写入失败", "traceId", entry.TraceID,
				"eventType", entry.EventType, "err", err)
		}
	}()
}
