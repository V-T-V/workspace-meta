// Package logging 封装 log/slog，按配置创建 JSON 或文本 logger。
// 对齐 generic-admin / auto-finance-assistant 的 logging 范式。
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New 按级别/格式创建 *slog.Logger。
// level: debug/info/warn/error；format: json/text。
func New(level, format string) *slog.Logger {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if strings.ToLower(format) == "text" {
		h = slog.NewTextHandler(os.Stderr, opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(h)
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
