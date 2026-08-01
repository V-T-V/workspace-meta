// Package md5 演示 MD5 哈希函数，作为与 SHA-256 的教学对比。
//
// MD5 把任意长度输入压缩成固定 16 字节（128 位）摘要，曾是工业标准，
// 但 **2004 年王小云团队发表了实用的碰撞攻击，能在秒级时间内构造碰撞**，
// 自此 MD5 退出所有安全敏感场景。本包用 Go 标准库 crypto/md5 实现，
// 教学目的是和 sha256 包对照：同样的输入，MD5 输出 16 字节、SHA-256 输出 32 字节，
// 并理解"为什么 MD5 还能用于非安全场景（文件校验和）但绝不能用于安全场景"。
package md5

import (
	"context"
	"crypto/md5"
	"fmt"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

// Hash 计算数据的 MD5 摘要，返回固定 16 字节。
func Hash(data []byte) [16]byte {
	return md5.Sum(data)
}

// HashHex 计算数据的 MD5 摘要，返回 32 字符的十六进制小写字符串。
func HashHex(data []byte) string {
	h := Hash(data)
	return core.HexEncode(h[:])
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Items []DemoItem
}

// DemoItem 是单组输入的哈希演示条目。
type DemoItem struct {
	Input   string
	HashHex string // 32 字符 hex 摘要
	ByteLen int    // 恒为 16
	Note    string
}

// Demo 演示 MD5 与 SHA-256 的对比：同一输入，MD5 输出 16 字节、SHA-256 输出 32 字节。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	inputs := []struct {
		s    string
		note string
	}{
		{"hello", "短串，输出 16 字节（对比 SHA-256 的 32 字节）"},
		{"Hello", "与 'hello' 仅差 1 位大小写 → 雪崩效应：摘要完全不同"},
		{"", "空串也能哈希，输出恒为 16 字节"},
		{"The quick brown fox jumps over the lazy dog", "经典长句"},
	}
	r := &DemoResult{}
	fmt.Println("=== MD5 demo（对比 SHA-256）===")
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
