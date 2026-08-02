// Package core 的 random.go：基于 crypto/rand 的密码学安全随机数。
//
// 为什么单独放这里：
//   - crypto-atlas 的算法包（AES-CBC IV、盐、OTP 密钥流等）普遍需要"不可预测"的字节。
//     math/rand 是伪随机，绝不能用于密钥/IV/盐/nonce；必须用 crypto/rand。
//   - 抽出一个 SecureRandom(n) 统一入口，避免各包重复 io.ReadFull(rand.Reader, ...) 样板。
//
// 设计：
//   - 零依赖，纯标准库（crypto/rand + errors）。
//   - 返回的错误透传 crypto/rand 的读取错误（OS 熵源故障时才出现）。
//   - n<=0 返回 nil + error，调用方按需处理（避免静默返回空切片）。
package core

import (
	"crypto/rand"
	"fmt"
	"io"
)

// SecureRandom 生成 n 字节的密码学安全随机数。
//
// 使用 crypto/rand.Read（内部 io.ReadFull(rand.Reader, ...)）：
//   - 成功：返回长度恰为 n 的随机字节切片。
//   - 失败：仅在操作系统熵源故障时出现（极罕见，常见于容器/受限环境），
//     此时返回 nil 和非 nil error，调用方绝不能把 nil 当合法密钥用。
//
// n<=0 视为非法请求，返回 (nil, error)：避免"静默成功返回空切片"埋下密钥长度
// 为 0 的隐患（历史上不少 CVE 源于此类空密钥被当成有效密钥）。
func SecureRandom(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("crypto-atlas/core: SecureRandom: n 不能为负数，实际 %d", n)
	}
	if n == 0 {
		return nil, fmt.Errorf("crypto-atlas/core: SecureRandom: n 不能为 0（拒绝返回空切片以防被当成空密钥）")
	}
	out := make([]byte, n)
	// io.ReadFull 保证要么填满 out，要么返回非 nil error（不会返回短读）。
	if _, err := io.ReadFull(rand.Reader, out); err != nil {
		return nil, fmt.Errorf("crypto-atlas/core: SecureRandom: 读取 %d 字节失败: %w", n, err)
	}
	return out, nil
}

// RandomDemoResult 是 RandomDemo 的输出摘要。
// Hex 字段方便打印（人类可读），Raw 是原始字节。
type RandomDemoResult struct {
	N   int
	Hex string
	Raw []byte
}

// RandomDemo 演示用 SecureRandom 生成 16 / 32 字节的随机字节（典型密钥/IV 长度），
// 并打印十六进制。返回摘要供上层（测试 / CLI）断言用。
//
// 注意：本函数不依赖 context（纯计算），与其它包的 Demo(ctx) 形态不同，
// 因为 core 是工具包而非"算法 demo"集合。
func RandomDemo() (*RandomDemoResult, error) {
	// 32 字节 = AES-256 密钥或 SHA-256 salt 的常见长度。
	const n = 32
	b, err := SecureRandom(n)
	if err != nil {
		return nil, err
	}
	h := HexEncode(b)
	fmt.Printf("=== crypto-atlas/core SecureRandom demo ===\n")
	fmt.Printf("请求字节数: %d\n", n)
	fmt.Printf("十六进制  : %s\n", h)
	fmt.Printf("说明      : 用 crypto/rand 生成，适合做密钥/IV/盐/nonce\n")
	return &RandomDemoResult{N: n, Hex: h, Raw: b}, nil
}
