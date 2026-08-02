// Package des 用 Go 标准库 crypto/des 演示 DES（数据加密标准）对称分组密码。
//
// DES 是 1970 年代诞生、统治业界约 30 年的对称分组密码，分组大小 **8 字节（64 位）**，
// 密钥有效长度仅 **56 位**（外加 8 位奇偶校验，密钥本身 8 字节）。因 56 位密钥在
// 现代算力下可被暴力穷举，DES 已于 2005 年被 NIST 正式退役，被 AES 取代。
//
// 本包是教学库：用标准库的成熟实现，重点展示 DES "是什么"、它的块大小（8 字节）
// 与 AES（16 字节）的对比，以及 56 位密钥为何不安全。
//
// 安全性：**已不安全**。新代码不要用 DES，用 AES（见 internal/aes）。
// 仍在用的"DES 系"实际是 3DES（加密-解密-加密三次调用），且也在逐步淘汰。
package des

import (
	"crypto/cipher"
	"crypto/des"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// blockSize 是 DES 的固定分组大小（字节）。DES 分组恒为 8 字节（64 位）。
// 对比：AES 分组是 16 字节（128 位）。
const blockSize = 8

// errKeyLen 是密钥长度非法错误。DES 只接受 8 字节密钥（其中有效 56 位）。
type errKeyLen struct{ got int }

func (e *errKeyLen) Error() string {
	return fmt.Sprintf("des: 密钥长度非法 %d 字节，必须是 8（含 8 位奇偶校验，有效密钥 56 位）", e.got)
}

// validateKey 校验密钥长度（必须 8 字节）并返回 DES block cipher。
func validateKey(key []byte) (cipher.Block, error) {
	if len(key) != blockSize {
		return nil, &errKeyLen{got: len(key)}
	}
	return des.NewCipher(key)
}

// Encrypt 用 DES 加密明文（多块 ECB 风格，PKCS#7 填充）。
//
// 每个明文块独立用同一个 56 位密钥加密。本包用 DES 仅作历史教学演示，
// 重点对比它的 8 字节小块 与 AES 的 16 字节块。
//
// 注意：DES 已不安全，新代码请用 AES（internal/aes）。
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := validateKey(key)
	if err != nil {
		return nil, err
	}
	padded := core.PKCS7Pad(plaintext, blockSize)
	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], padded[i:i+blockSize])
	}
	return ciphertext, nil
}

// Decrypt 用 DES 解密密文（Encrypt 的逆运算）。
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := validateKey(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%blockSize != 0 {
		return nil, fmt.Errorf("des: 密文长度 %d 非 %d 倍数", len(ciphertext), blockSize)
	}
	padded := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += blockSize {
		block.Decrypt(padded[i:i+blockSize], ciphertext[i:i+blockSize])
	}
	return core.PKCS7Unpad(padded, blockSize)
}
