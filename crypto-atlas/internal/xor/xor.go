// Package xor 实现 XOR 密码（对称流密码的鼻祖）。
//
// XOR 密码：密文 = 明文 XOR 密钥；解密再 XOR 一次（XOR 自反性：
// (P XOR K) XOR K = P）。密钥短于明文时循环复用，这是现代流密码
// （RC4 / ChaCha20）的雏形——只是现代流密码用密钥扩展出与明文等长的伪随机密钥流。
//
// 安全性：单独的 XOR 几乎没有安全性。密钥空间取决于密钥长度；一旦密钥被复用
// 或攻击者拿到一组明文/密文对（已知明文攻击），密钥立刻泄露（P XOR C = K）。
// 它的教学价值在于演示"对称"和"流密码"两个核心概念。
package xor

import (
	"context"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// xorBytesRepeat 对 data 逐字节 XOR，密钥 key 短则循环复用。
// （core.XorBytes 只处理 min(len)，这里需要循环复用，所以本包自己实现循环。）
func xorBytesRepeat(data, key []byte) []byte {
	if len(key) == 0 {
		// 空 key 等价于不加密：直接拷贝返回
		out := make([]byte, len(data))
		copy(out, data)
		return out
	}
	out := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		out[i] = data[i] ^ key[i%len(key)]
	}
	return out
}

// Encrypt 加密明文：逐字节 XOR 密钥，密钥循环复用。
// 解密与加密是同一个操作（XOR 自反），即 Decrypt == Encrypt。
func Encrypt(plaintext, key []byte) []byte {
	return xorBytesRepeat(plaintext, key)
}

// Decrypt 解密密文。对 XOR 而言解密 == 加密（同函数）。
// 此处独立提供 Decrypt 仅为接口对称、与其他算法包签名一致。
func Decrypt(ciphertext, key []byte) []byte {
	return xorBytesRepeat(ciphertext, key)
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Plaintext  []byte
	Key        []byte
	Ciphertext []byte
	Decrypted  []byte
	// CiphertextHex 是密文的十六进制表示，便于打印不可见字节。
	CiphertextHex string
}

// Demo 演示 XOR 密码（"Hello" XOR "key"，演示循环复用与自反性）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	plain := []byte("Hello")
	key := []byte("key") // 短于明文，演示循环复用
	ct := Encrypt(plain, key)
	pt := Decrypt(ct, key)
	fmt.Println("=== XOR 密码 demo ===")
	fmt.Printf("明文: %s\n密钥: %s\n密文(hex): %s\n解密: %s\n",
		plain, key, core.HexEncode(ct), pt)
	return &DemoResult{
		Plaintext:     plain,
		Key:           key,
		Ciphertext:    ct,
		Decrypted:     pt,
		CiphertextHex: core.HexEncode(ct),
	}, nil
}
