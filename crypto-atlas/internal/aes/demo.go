package aes

import (
	"context"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// ECBResult 是 ECB demo 的输出摘要。
type ECBResult struct {
	PlaintextHex     string // 明文（hex）
	CiphertextHex    string // ECB 密文（hex）
	DecryptedHex     string // 解密还原（hex）
	Block1Ciphertext string // 第 1 个密文块（hex）
	Block2Ciphertext string // 第 2 个密文块（hex）
	RepeatedPattern  bool   // 两块密文是否相同（ECB 致命缺陷）
}

// CBCResult 是 CBC demo 的输出摘要。
type CBCResult struct {
	PlaintextHex     string // 明文（hex）
	IVHex            string // 初始向量（hex）
	CiphertextHex    string // CBC 密文（hex）
	DecryptedHex     string // 解密还原（hex）
	Block1Ciphertext string // 第 1 个密文块（hex）
	Block2Ciphertext string // 第 2 个密文块（hex）
	RepeatedPattern  bool   // 两块密文是否相同（应为 false——CBC 优势）
}

// DemoResult 是 AES demo 的总输出。
type DemoResult struct {
	KeyBits int       // 密钥位数（本 demo 用 AES-128 = 128）
	KeyHex  string    // 密钥（hex）
	ECB     ECBResult // ECB 模式结果
	CBC     CBCResult // CBC 模式结果
}

// Demo 演示 AES-128 在 ECB 与 CBC 两种模式下的差异。
//
// 关键教学点：用相同的重复明文块（"AAAAAAAAAAAAAAAA" × 2）分别加密：
//   - ECB：两块明文相同 → 两块密文也相同（模式泄露）
//   - CBC：两块明文相同，但因 IV/链式 XOR，两块密文不同
//
// 本 demo 完全确定性：key/iv 全为固定字节，不依赖随机数。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	// 固定的 16 字节密钥（AES-128）。教学用，实际应用绝不可硬编码。
	key := []byte("0123456789abcdef") // 恰好 16 字节
	if len(key) != blockSize {
		return nil, fmt.Errorf("demo: 内置 key 长度 %d 非 %d", len(key), blockSize)
	}

	// 重复明文块：两块完全相同（各 16 字节）。
	// 用这个刻意构造的数据，凸显 ECB 的模式泄露问题。
	plain := []byte("AAAAAAAAAAAAAAAA" + "AAAAAAAAAAAAAAAA") // 32 字节

	fmt.Println("=== AES 对称加密 demo（AES-128）===")
	fmt.Printf("密钥（%d 位）: %s\n", len(key)*8, core.HexEncode(key))
	fmt.Printf("明文        : %s\n", core.HexEncode(plain))

	// ---- ECB ----
	ecbCT, err := EncryptECB(plain, key)
	if err != nil {
		return nil, fmt.Errorf("ECB 加密失败: %w", err)
	}
	ecbPT, err := DecryptECB(ecbCT, key)
	if err != nil {
		return nil, fmt.Errorf("ECB 解密失败: %w", err)
	}
	b1, b2 := ecbCT[:blockSize], ecbCT[blockSize:2*blockSize]
	ecbRepeat := string(b1) == string(b2)

	fmt.Println("\n--- ECB 模式（不安全，教学用）---")
	fmt.Printf("密文        : %s\n", core.HexEncode(ecbCT))
	fmt.Printf("块1         : %s\n", core.HexEncode(b1))
	fmt.Printf("块2         : %s\n", core.HexEncode(b2))
	fmt.Printf("两块密文相同? %v  ← ECB 致命缺陷：相同明文块→相同密文块，泄露明文模式\n", ecbRepeat)
	fmt.Printf("解密还原    : %s\n", core.HexEncode(ecbPT))

	// ---- CBC ----
	// 固定 IV（教学确定性）；实际应用应使用随机 IV。
	iv := []byte("abcdef9876543210") // 恰好 16 字节
	if len(iv) != blockSize {
		return nil, fmt.Errorf("demo: 内置 iv 长度 %d 非 %d", len(iv), blockSize)
	}
	cbcCT, err := EncryptCBC(plain, key, iv)
	if err != nil {
		return nil, fmt.Errorf("CBC 加密失败: %w", err)
	}
	cbcPT, err := DecryptCBC(cbcCT, key, iv)
	if err != nil {
		return nil, fmt.Errorf("CBC 解密失败: %w", err)
	}
	cb1, cb2 := cbcCT[:blockSize], cbcCT[blockSize:2*blockSize]
	cbcRepeat := string(cb1) == string(cb2)

	fmt.Println("\n--- CBC 模式（推荐，链式依赖 + IV）---")
	fmt.Printf("IV          : %s\n", core.HexEncode(iv))
	fmt.Printf("密文        : %s\n", core.HexEncode(cbcCT))
	fmt.Printf("块1         : %s\n", core.HexEncode(cb1))
	fmt.Printf("块2         : %s\n", core.HexEncode(cb2))
	fmt.Printf("两块密文相同? %v  ← CBC 改进：即使明文块相同，密文也不同\n", cbcRepeat)
	fmt.Printf("解密还原    : %s\n", core.HexEncode(cbcPT))

	return &DemoResult{
		KeyBits: len(key) * 8,
		KeyHex:  core.HexEncode(key),
		ECB: ECBResult{
			PlaintextHex:     core.HexEncode(plain),
			CiphertextHex:    core.HexEncode(ecbCT),
			DecryptedHex:     core.HexEncode(ecbPT),
			Block1Ciphertext: core.HexEncode(b1),
			Block2Ciphertext: core.HexEncode(b2),
			RepeatedPattern:  ecbRepeat,
		},
		CBC: CBCResult{
			PlaintextHex:     core.HexEncode(plain),
			IVHex:            core.HexEncode(iv),
			CiphertextHex:    core.HexEncode(cbcCT),
			DecryptedHex:     core.HexEncode(cbcPT),
			Block1Ciphertext: core.HexEncode(cb1),
			Block2Ciphertext: core.HexEncode(cb2),
			RepeatedPattern:  cbcRepeat,
		},
	}, nil
}
