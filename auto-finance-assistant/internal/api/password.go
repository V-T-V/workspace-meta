package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// hashPassword 对明文密码做 SHA-256 哈希。
// config.yaml 中可存明文或哈希：
//   - 明文密码：运行时自动转换为哈希比较（向后兼容）
//   - 哈希密码（sha256:前缀）：直接比较
func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return "sha256:" + hex.EncodeToString(h[:])
}

// verifyPassword 验证密码。
// storedPassword 可以是明文或 sha256: 前缀的哈希。
// 提供的密码始终先做 SHA-256 再比较（常量时间），防时序攻击。
func verifyPassword(provided, storedPassword string) bool {
	if storedPassword == "" {
		return true // 未设密码，放行
	}

	providedHash := hashPassword(provided)

	// 修复：用 strings.HasPrefix 判断前缀（原 [:8] == "sha256:" 误判，因前缀为 7 字符）。
	if strings.HasPrefix(storedPassword, "sha256:") {
		// 存储的是哈希：直接比较哈希
		return subtle.ConstantTimeCompare([]byte(providedHash), []byte(storedPassword)) == 1
	}

	// 存储的是明文：兼容模式，比较明文（仍用常量时间）
	return subtle.ConstantTimeCompare([]byte(provided), []byte(storedPassword)) == 1
}
