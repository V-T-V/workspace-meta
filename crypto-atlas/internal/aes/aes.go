// Package aes 用 Go 标准库 crypto/aes 演示 AES（高级加密标准）对称分组密码。
//
// AES 是当前最广泛使用的对称分组密码：固定 16 字节（128 位）分组，
// 密钥长度 16/24/32 字节对应 AES-128/192/256。
//
// 本包是教学库：不手写 S-Box/ShiftRows/MixColumns，而是用标准库的成熟实现，
// 重点展示 AES "是什么""怎么用"，以及 ECB/CBC 两种工作模式的差别——
// 特别是用重复明文块演示 ECB 的致命缺陷（相同明文块→相同密文块）。
//
// 安全性：截至 2026 年，AES 没有已知的可行（小于 2^128 工作量）攻击。
// 选用 ECB 仅为教学演示，**生产环境永远不要用 ECB**。
package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// blockSize 是 AES 的固定分组大小（字节）。无论密钥位数，分组都是 16 字节。
const blockSize = 16

// errKeyLen 是密钥长度非法错误。AES 只接受 16/24/32 字节密钥。
type errKeyLen struct{ got int }

func (e *errKeyLen) Error() string {
	return fmt.Sprintf("aes: 密钥长度非法 %d 字节，必须是 16/24/32（AES-128/192/256）", e.got)
}

// errIVLen 是 IV（初始向量）长度错误。CBC 模式要求 IV 长度等于分组大小（16 字节）。
type errIVLen struct{ got int }

func (e *errIVLen) Error() string {
	return fmt.Sprintf("aes: IV 长度非法 %d 字节，CBC 模式要求 %d 字节", e.got, blockSize)
}

// validateKey 校验密钥长度并返回对应的 block cipher。
func validateKey(key []byte) (cipher.Block, error) {
	switch len(key) {
	case 16, 24, 32: // AES-128 / AES-192 / AES-256
	default:
		return nil, &errKeyLen{got: len(key)}
	}
	return aes.NewCipher(key)
}

// EncryptECB 用 ECB（电子密码本）模式加密。
//
// ECB 是最简单的工作模式：每个 16 字节明文块独立加密，互不影响。
// 这导致一个致命缺陷——相同的明文块永远加密成相同的密文块，
// 从而泄露明文的模式（参见著名的"ECB 企鹅图"）。
//
// 本函数仅用于教学演示该缺陷，生产环境请用 EncryptCBC。
func EncryptECB(plaintext, key []byte) ([]byte, error) {
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

// DecryptECB 用 ECB 模式解密（EncryptECB 的逆运算）。
func DecryptECB(ciphertext, key []byte) ([]byte, error) {
	block, err := validateKey(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%blockSize != 0 {
		return nil, fmt.Errorf("aes: 密文长度 %d 非 %d 倍数", len(ciphertext), blockSize)
	}
	padded := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += blockSize {
		block.Decrypt(padded[i:i+blockSize], ciphertext[i:i+blockSize])
	}
	return core.PKCS7Unpad(padded, blockSize)
}

// EncryptCBC 用 CBC（密文分组链接）模式加密。
//
// CBC：每个明文块先与前一块的密文 XOR，再加密；第一块与 IV XOR。
// 这种"链式依赖"使得即使明文块相同，密文块也不同——消除 ECB 的模式泄露。
// IV 必须不可预测（随机），但不要求保密，通常随密文一起传输。
func EncryptCBC(plaintext, key, iv []byte) ([]byte, error) {
	block, err := validateKey(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != blockSize {
		return nil, &errIVLen{got: len(iv)}
	}
	padded := core.PKCS7Pad(plaintext, blockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext, nil
}

// DecryptCBC 用 CBC 模式解密（EncryptCBC 的逆运算）。
func DecryptCBC(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := validateKey(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != blockSize {
		return nil, &errIVLen{got: len(iv)}
	}
	if len(ciphertext) == 0 || len(ciphertext)%blockSize != 0 {
		return nil, fmt.Errorf("aes: 密文长度 %d 非 %d 倍数", len(ciphertext), blockSize)
	}
	padded := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(padded, ciphertext)
	return core.PKCS7Unpad(padded, blockSize)
}

// SupportedKeySizes 返回 AES 支持的密钥长度（字节）。
func SupportedKeySizes() []int {
	return []int{16, 24, 32}
}
