// Package sha256 演示 SHA-256 哈希函数。
//
// 哈希（散列）与加密有本质区别：无密钥、不可逆、定长输出。
// SHA-256 把任意长度输入压缩成固定 32 字节（256 位）摘要，
// 三大特性——定长输出 / 雪崩效应（改 1 位输入输出全变）/ 单向（无法从摘要反推输入）。
//
// 本包用 Go 标准库 crypto/sha256 实现。这是教学库，目标是展示"哈希是什么"，
// 而非手写 64 轮压缩函数（标准库的实现经过了严格审计与硬件加速，远比手写可靠）。
package sha256

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// Hash 计算数据的 SHA-256 摘要，返回固定 32 字节。
func Hash(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// HashHex 计算数据的 SHA-256 摘要，返回 64 字符的十六进制小写字符串。
func HashHex(data []byte) string {
	h := Hash(data)
	return core.HexEncode(h[:])
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	// Items 演示"定长输出 + 雪崩效应 + 单向"的多组输入/输出。
	Items []DemoItem
}

// DemoItem 是单组输入的哈希演示条目。
type DemoItem struct {
	Input   string // 原始输入（文本）
	HashHex string // 64 字符 hex 摘要
	ByteLen int    // 摘要字节数（恒为 32，演示"定长"）
	Note    string // 该条目的教学说明
}

// Demo 演示 SHA-256 的三大特性。
//
//   - 定长输出：空串、短串、长串的摘要都是 32 字节
//   - 雪崩效应：hello / Hello 仅差 1 位大小写，摘要完全不同
//   - 单向：从摘要无法反推输入（演示用，无代码可证，靠说明）
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	inputs := []struct {
		s    string
		note string
	}{
		{"hello", "短串"},
		{"Hello", "与 'hello' 仅差 1 位大小写 → 雪崩效应：摘要完全不同"},
		{"", "空串也能哈希（且摘要恒定），证明定长输出"},
		{"The quick brown fox jumps over the lazy dog", "经典长句"},
	}
	r := &DemoResult{}
	fmt.Println("=== SHA-256 demo ===")
	fmt.Printf("%-48s %-6s %s\n", "输入", "字节数", "摘要(hex)")
	for _, in := range inputs {
		h := Hash([]byte(in.s))
		hx := core.HexEncode(h[:])
		fmt.Printf("%-48q %-6d %s  (%s)\n", in.s, len(h), hx, in.note)
		r.Items = append(r.Items, DemoItem{
			Input:   in.s,
			HashHex: hx,
			ByteLen: len(h),
			Note:    in.note,
		})
	}
	return r, nil
}
