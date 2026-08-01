// Package hmac 实现 HMAC（基于哈希的消息认证码）。
//
// HMAC 用一个密钥 + 一个哈希函数，产出消息认证码（MAC）。
// 接收方用同一密钥验证——若 MAC 不匹配，说明消息被篡改或发送方无密钥。
//
// HMAC = H((K' ⊕ opad) || H((K' ⊕ ipad) || message))
//
//	K' = 密钥填充到块大小
//	ipad = 0x36 重复，opad = 0x5c 重复
//	H = 哈希函数（本包用 SHA-256）
//
// 这是 RFC 2104 标准。应用：API 签名（AWS SigV4）、JWT、TLS 1.2 MAC。
package hmac

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

const (
	blockSize = 64 // SHA-256 的块大小（字节）
	opad      = 0x5c
	ipad      = 0x36
)

// Compute 计算 HMAC-SHA-256：用 key 对 message 计算消息认证码。
// 返回 32 字节的 MAC。
func Compute(key, message []byte) []byte {
	// 1. 密钥处理：若长于块大小则先哈希；短则补 0 到块大小
	k := processKey(key)

	// 2. 构造 ipad/opad 密钥
	ikeyPad := make([]byte, blockSize)
	okeyPad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		ikeyPad[i] = k[i] ^ ipad
		okeyPad[i] = k[i] ^ opad
	}

	// 3. 内层哈希：H(ipad_key || message)
	inner := sha256.New()
	inner.Write(ikeyPad)
	inner.Write(message)
	innerHash := inner.Sum(nil)

	// 4. 外层哈希：H(opad_key || innerHash)
	outer := sha256.New()
	outer.Write(okeyPad)
	outer.Write(innerHash)
	return outer.Sum(nil)
}

// ComputeHex 返回 HMAC 的十六进制字符串。
func ComputeHex(key, message []byte) string {
	return core.HexEncode(Compute(key, message))
}

// Verify 验证消息的 MAC 是否正确（恒定时间比较，防时序攻击）。
func Verify(key, message, expectedMAC []byte) bool {
	actual := Compute(key, message)
	return constantTimeCompare(actual, expectedMAC)
}

// processKey 处理密钥：过长则哈希，然后补 0 到块大小。
func processKey(key []byte) []byte {
	if len(key) > blockSize {
		h := sha256.Sum256(key)
		key = h[:]
	}
	k := make([]byte, blockSize)
	copy(k, key)
	return k
}

// constantTimeCompare 恒定时间比较（防时序攻击）。
// 无论是否匹配都遍历全部字节。
func constantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Message  string
	Key      string
	MAC      string // hex
	Verified bool
	Tampered bool
}

// Demo 演示 HMAC 的计算 + 验证 + 篡改检测。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	key := []byte("secret-key-12345")
	msg := []byte("transfer:alice→bob:100")

	mac := Compute(key, msg)
	macHex := core.HexEncode(mac)

	// 正常验证
	ok := Verify(key, msg, mac)

	// 篡改消息后验证（应失败）
	tamperedMsg := []byte("transfer:alice→bob:999")
	tampered := Verify(key, tamperedMsg, mac)

	fmt.Println("=== HMAC-SHA-256 demo ===")
	fmt.Printf("密钥: %q\n", string(key))
	fmt.Printf("消息: %q\n", string(msg))
	fmt.Printf("HMAC: %s\n", macHex)
	fmt.Printf("验证（原消息）: %v\n", ok)
	fmt.Printf("验证（篡改金额 100→999）: %v ← 篡改被检测到\n", tampered)
	return &DemoResult{
		Message: string(msg), Key: string(key), MAC: macHex,
		Verified: ok, Tampered: !tampered,
	}, nil
}
