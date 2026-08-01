// Package vigenere 实现维吉尼亚密码（最经典的多表替换密码）。
//
// 维吉尼亚密码：明文每个字母用密钥对应字母的位移加密。密钥比明文短时循环复用。
// 例如明文 ATTACKATDAWN + 密钥 LEMON（循环成 LEMONLEMONLE）→ 密文 LXFOPVEFRNHR。
// 只处理 A-Z/a-z（密钥也只取字母部分），大小写不敏感，统一转大写处理。
//
// 与凯撒的区别：凯撒是"单表"（全文明文用同一位移），维吉尼亚是"多表"
// （每个明文字母用密钥不同位置的位移），因此能抵抗简单频率分析。
// 但它仍被 Kasiski 重复间距分析（1863）破解——这就是"多表 ≠ 安全"的教训。
package vigenere

import (
	"context"
	"fmt"
	"strings"
)

// normalize 把字符串里的字母全部转大写，丢弃非字母字符。
func normalize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r)
		case r >= 'a' && r <= 'z':
			sb.WriteRune(r - 'a' + 'A')
		default:
			// 非字母丢弃
		}
	}
	return sb.String()
}

// Encrypt 加密明文。只处理 A-Z/a-z（非字母原样保留，保留大小写）。
// 密钥大小写不敏感，密钥中的非字母字符被丢弃；密钥去掉非字母后为空则原样返回明文。
func Encrypt(plaintext, key string) string {
	k := normalize(key)
	if k == "" {
		return plaintext
	}
	ki := 0
	var sb strings.Builder
	for _, r := range plaintext {
		switch {
		case r >= 'A' && r <= 'Z':
			shift := rune(k[ki%len(k)] - 'A')
			sb.WriteRune('A' + (r-'A'+shift)%26)
			ki++
		case r >= 'a' && r <= 'z':
			shift := rune(k[ki%len(k)] - 'A')
			sb.WriteRune('a' + (r-'a'+shift)%26)
			ki++
		default:
			sb.WriteRune(r) // 非字母原样保留，且不消耗密钥
		}
	}
	return sb.String()
}

// Decrypt 解密密文（密钥相同时可还原 Encrypt 的输出）。
func Decrypt(ciphertext, key string) string {
	k := normalize(key)
	if k == "" {
		return ciphertext
	}
	ki := 0
	var sb strings.Builder
	for _, r := range ciphertext {
		switch {
		case r >= 'A' && r <= 'Z':
			shift := rune(k[ki%len(k)] - 'A')
			// +26 再 %26 处理减法后可能的负数
			sb.WriteRune('A' + (r-'A'-shift+26)%26)
			ki++
		case r >= 'a' && r <= 'z':
			shift := rune(k[ki%len(k)] - 'A')
			sb.WriteRune('a' + (r-'a'-shift+26)%26)
			ki++
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Plaintext  string
	Key        string
	Ciphertext string
	Decrypted  string
}

// Demo 演示维吉尼亚密码（经典 LEMON 例子：ATTACKATDAWN → LXFOPVEFRNHR）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	plain := "ATTACKATDAWN"
	key := "LEMON"
	ct := Encrypt(plain, key)
	pt := Decrypt(ct, key)
	fmt.Println("=== 维吉尼亚密码 demo ===")
	fmt.Printf("明文: %s\n密钥: %s\n密文: %s\n解密: %s\n", plain, key, ct, pt)
	return &DemoResult{Plaintext: plain, Key: key, Ciphertext: ct, Decrypted: pt}, nil
}
