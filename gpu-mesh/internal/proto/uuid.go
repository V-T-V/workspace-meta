package proto

import "github.com/google/uuid"

// uuidString 生成 UUIDv4 字符串。
func uuidString() string {
	return uuid.NewString()
}

// uuidShort 生成短 ID（前 8 位），用于人类可读的标识。
func uuidShort() string {
	return uuid.NewString()[:8]
}
