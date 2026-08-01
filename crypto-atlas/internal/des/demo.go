package des

import (
	"context"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// DemoResult 是 DES demo 的输出摘要。
type DemoResult struct {
	KeyBits        int    // 有效密钥位数（DES 有效 56 位）
	KeyHex         string // 密钥（hex，8 字节）
	Plaintext      string // 明文（字符串）
	PlaintextHex   string // 明文（hex）
	CiphertextHex  string // 密文（hex）
	Decrypted      string // 解密还原（字符串）
	DecryptedHex   string // 解密还原（hex）
	BlockSizeBytes int    // DES 分组大小（8 字节）
	AESBlockSize   int    // 对照：AES 分组大小（16 字节）
}

// Demo 演示 DES 加密一段文本，并对比它的块大小（8 字节）与 AES（16 字节）。
//
// 本 demo 完全确定性：密钥固定，不依赖随机数。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	// 固定的 8 字节密钥（教学用，实际应用绝不可硬编码）。
	// 注意：DES 密钥虽是 8 字节，但每字节的最低位是奇偶校验，有效密钥只有 56 位。
	key := []byte("password") // 恰好 8 字节
	if len(key) != blockSize {
		return nil, fmt.Errorf("demo: 内置 key 长度 %d 非 %d", len(key), blockSize)
	}

	plain := "DES is old, AES is better!" // 26 字节

	fmt.Println("=== DES 对称加密 demo（历史算法）===")
	fmt.Printf("密钥（%d 有效位）: %s\n", 56, core.HexEncode(key))
	fmt.Printf("分组大小        : %d 字节（对比 AES 的 %d 字节）\n", blockSize, 16)
	fmt.Printf("明文            : %q\n", plain)

	ct, err := Encrypt([]byte(plain), key)
	if err != nil {
		return nil, fmt.Errorf("DES 加密失败: %w", err)
	}
	pt, err := Decrypt(ct, key)
	if err != nil {
		return nil, fmt.Errorf("DES 解密失败: %w", err)
	}

	fmt.Printf("密文（hex）      : %s\n", core.HexEncode(ct))
	fmt.Printf("密文块数        : %d 块 × %d 字节 = %d 字节（PKCS#7 填充后）\n",
		len(ct)/blockSize, blockSize, len(ct))
	fmt.Printf("解密还原        : %q\n", string(pt))

	fmt.Println("\n小结：")
	fmt.Printf("  - DES 块 = %d 字节、有效密钥 %d 位 → 已被暴力穷举攻破（1998 EFF 22 小时）\n", blockSize, 56)
	fmt.Printf("  - AES 块 = %d 字节、密钥 128/192/256 位 → 至今无有效攻击\n", 16)

	return &DemoResult{
		KeyBits:        56,
		KeyHex:         core.HexEncode(key),
		Plaintext:      plain,
		PlaintextHex:   core.HexEncode([]byte(plain)),
		CiphertextHex:  core.HexEncode(ct),
		Decrypted:      string(pt),
		DecryptedHex:   core.HexEncode(pt),
		BlockSizeBytes: blockSize,
		AESBlockSize:   16,
	}, nil
}
