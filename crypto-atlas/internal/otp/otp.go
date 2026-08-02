// Package otp 手写实现一次性密码本（One-Time Pad, OTP）——
// 唯一被信息论证明"绝对安全"的密码。
//
// 核心式（与 XOR 密码同形，但约束截然不同）：
//
//	加密：C = P XOR K
//	解密：P = C XOR K      （XOR 自反：(P XOR K) XOR K = P）
//
// 三条铁律（缺任一条即沦为可破的 XOR 密码）：
//  1. K 必须**真随机**（用 crypto/rand，不是 math/rand 伪随机）
//  2. len(K) == len(P)（密钥与明文等长，不循环、不截断）
//  3. K 只用一次（绝不复用——两次同密钥加密即"two-time pad"，立刻可破）
//
// 满足以上三条时，Shannon 1949 证明 OTP 是"绝对安全"（perfect secrecy）：
// 密文不含明文的任何信息，无论算力多强都不可能破解。代价是密钥与明文等长，
// 密钥分发难到不实用——所以现实里用 AES-GCM / ChaCha20 这类"计算安全"方案。
package otp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// ErrKeyLengthMismatch 在 EncryptWithKey 时密钥长度不等于明文长度时返回。
// OTP 铁律之一：密钥必须与明文等长（不循环、不截断）。
var ErrKeyLengthMismatch = errors.New("otp: 密钥长度必须等于明文长度")

// Encrypt 加密明文：用 crypto/rand 生成与明文等长的真随机密钥，XOR 得密文。
//
// 返回 (ciphertext, key, err)。key 是本次生成的真随机密钥，必须安全保管，
// 且只能用于解密这一次的 ciphertext——用后即弃。
//
// 安全性完全依赖 key 的真随机性：crypto/rand.Reader 是操作系统 CSPRNG
// （Windows CryptGenRandom / Linux getrandom），输出通过密码学随机性检验。
func Encrypt(plaintext []byte) (ciphertext, key []byte, err error) {
	key = make([]byte, len(plaintext))
	if _, err = rand.Read(key); err != nil {
		return nil, nil, fmt.Errorf("otp: 生成随机密钥失败: %w", err)
	}
	// 此处 len(key) == len(plaintext) 必然成立，直接 XOR 即可。
	ciphertext = core.XorBytes(plaintext, key)
	return ciphertext, key, nil
}

// EncryptWithKey 用指定密钥加密：密钥长度必须严格等于明文长度。
//
// 与 Encrypt 的区别：调用方自行提供密钥（便于复现/测试/外部密钥管理）。
// 密钥长度 != 明文长度时返回 ErrKeyLengthMismatch——这是 OTP 与循环 XOR
// 密码的分水岭：OTP 绝不循环复用密钥。
func EncryptWithKey(plaintext, key []byte) ([]byte, error) {
	if len(key) != len(plaintext) {
		return nil, fmt.Errorf("%w: 明文 %d 字节, 密钥 %d 字节", ErrKeyLengthMismatch, len(plaintext), len(key))
	}
	return core.XorBytes(plaintext, key), nil
}

// Decrypt 解密密文：用与加密时相同的密钥 XOR 还原明文。
//
// 解密 == 加密（XOR 自反）。密钥必须与加密时完全一致（同一段随机字节），
// 否则得到的是无意义字节流（但不会报错——OTP 解密对任意密钥都"成功"）。
//
// ciphertext 与 key 长度不一致时，按 min 取前缀 XOR（与 core.XorBytes 一致），
// 解密结果会比原文短；调用方应保证两者等长。
func Decrypt(ciphertext, key []byte) []byte {
	return core.XorBytes(ciphertext, key)
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Plaintext     []byte
	Key           []byte
	Ciphertext    []byte
	Decrypted     []byte
	CiphertextHex string // 密文的十六进制表示（密文含不可打印字节）
	KeyHex        string // 密钥的十六进制表示
}

// Demo 演示一次性密码本：加密一段消息，打印密文 hex + 密钥 hex + 解密还原。
//
// 每次运行的密钥/密文都不同（真随机），但解密必然还原明文。
// 这正是 OTP 的魅力：密文看起来纯随机，却能在持有密钥时精确还原。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	plain := []byte("One-Time Pad: perfect secrecy since 1917")
	ct, key, err := Encrypt(plain)
	if err != nil {
		return nil, err
	}
	pt := Decrypt(ct, key)

	fmt.Println("=== 一次性密码本 (One-Time Pad) demo ===")
	fmt.Printf("明文: %s\n", plain)
	fmt.Printf("密钥(hex): %s   ← 真随机，与明文等长，用后即弃\n", core.HexEncode(key))
	fmt.Printf("密文(hex): %s   ← 看起来纯随机，不含明文信息\n", core.HexEncode(ct))
	fmt.Printf("解密: %s\n", pt)
	fmt.Printf("✓ 密钥正确，明文还原\n")

	return &DemoResult{
		Plaintext:     plain,
		Key:           key,
		Ciphertext:    ct,
		Decrypted:     pt,
		CiphertextHex: core.HexEncode(ct),
		KeyHex:        core.HexEncode(key),
	}, nil
}
