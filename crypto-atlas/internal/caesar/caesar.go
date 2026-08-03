// Package caesar 实现凯撒密码（最古老的替换式密码）。
//
// 凯撒密码：明文的每个字母按字母表向后移 key 位得到密文，解密时向前移。
// 例如 key=3 时：A→D, B→E, ..., X→A（环绕）。只处理英文字母，其他字符不变。
//
// 这是密码学的"零号算法"——教学用它引入"密钥/加密/解密"的概念。
// 安全性：极弱（只有 25 个可能的 key，暴力秒破）。
package caesar

import (
	"context"
	"fmt"
	"strings"
)

// Encrypt 加密明文。key 是位移量（可为负或 >26，自动取模到 0-25）。
// 只处理 A-Z/a-z，其他字符原样保留。保留大小写。
func Encrypt(plaintext string, key int) string {
	shift := ((key % 26) + 26) % 26 // 处理负数和大数，归一化到 0-25
	var sb strings.Builder
	for _, r := range plaintext {
		switch {
		case r >= 'A' && r <= 'Z':
			sb.WriteRune('A' + (r-'A'+rune(shift))%26)
		case r >= 'a' && r <= 'z':
			sb.WriteRune('a' + (r-'a'+rune(shift))%26)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// Decrypt 解密密文（等价于 Encrypt(ciphertext, -key)）。
func Decrypt(ciphertext string, key int) string {
	return Encrypt(ciphertext, -key)
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Plaintext  string
	Key        int
	Ciphertext string
	Decrypted  string
}

// Demo 演示凯撒密码（key=3，经典凯撒本人用的位移）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	plain := "Hello, Caesar!"
	key := 3
	ct := Encrypt(plain, key)
	pt := Decrypt(ct, key)
	fmt.Println("=== 凯撒密码 demo ===")
	fmt.Printf("明文: %s\n密钥: %d\n密文: %s\n解密: %s\n", plain, key, ct, pt)
	return &DemoResult{Plaintext: plain, Key: key, Ciphertext: ct, Decrypted: pt}, nil
}

// ROT13 是凯撒密码 key=13 的特例（自对合：加密=解密）。
func ROT13(text string) string {
	return Encrypt(text, 13)
}

// CaesarEncryptHex 加密并返回十六进制（调试用）。
func CaesarEncryptHex(plaintext string, key int) string {
	return core.HexEncode([]byte(Encrypt(plaintext, key)))
}
