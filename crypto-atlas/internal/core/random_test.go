package core

import (
	"encoding/hex"
	"testing"
)

// TestSecureRandomLength 长度正确：返回切片长度等于请求长度。
func TestSecureRandomLength(t *testing.T) {
	for _, n := range []int{1, 8, 16, 31, 32, 64, 100} {
		b, err := SecureRandom(n)
		if err != nil {
			t.Fatalf("SecureRandom(%d) 出错: %v", n, err)
		}
		if len(b) != n {
			t.Errorf("SecureRandom(%d) 长度 = %d, want %d", n, len(b), n)
		}
	}
}

// TestSecureRandomNonZero 随机字节几乎不可能全 0：用这个粗略验证"确实在产生随机数据"。
// 32 字节全 0 的概率是 2^-256，等于零，所以一旦命中必是 bug（例如未初始化）。
func TestSecureRandomNonZero(t *testing.T) {
	b, err := SecureRandom(32)
	if err != nil {
		t.Fatalf("SecureRandom(32) 出错: %v", err)
	}
	allZero := true
	for _, x := range b {
		if x != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("SecureRandom(32) 返回全 0，几乎不可能发生，疑似未填充随机源")
	}
}

// TestSecureRandomDistinct 两次调用返回不同结果（验证非确定 / 非固定种子）。
// 与 math/rand 配合固定 seed 的行为形成对比。
func TestSecureRandomDistinct(t *testing.T) {
	a, err := SecureRandom(32)
	if err != nil {
		t.Fatalf("第一次 SecureRandom(32): %v", err)
	}
	b, err := SecureRandom(32)
	if err != nil {
		t.Fatalf("第二次 SecureRandom(32): %v", err)
	}
	if hex.EncodeToString(a) == hex.EncodeToString(b) {
		t.Error("两次 SecureRandom(32) 返回相同结果，违反随机性")
	}
}

// TestSecureRandomInvalidN 非法 n 必须返回 error 而非静默返回空切片。
func TestSecureRandomInvalidN(t *testing.T) {
	if _, err := SecureRandom(0); err == nil {
		t.Error("SecureRandom(0) 应返回 error（拒绝空切片）")
	}
	if _, err := SecureRandom(-1); err == nil {
		t.Error("SecureRandom(-1) 应返回 error")
	}
	if _, err := SecureRandom(-100); err == nil {
		t.Error("SecureRandom(-100) 应返回 error")
	}
}

// TestSecureRandomDoesNotMutateInput/共享：每次调用必须返回独立的新切片，
// 后续调用不能改写前一次返回的切片（共享底层数组是常见 bug 来源）。
func TestSecureRandomIndependent(t *testing.T) {
	a, err := SecureRandom(16)
	if err != nil {
		t.Fatalf("SecureRandom(16) a: %v", err)
	}
	snapshot := append([]byte{}, a...)
	b, err := SecureRandom(16)
	if err != nil {
		t.Fatalf("SecureRandom(16) b: %v", err)
	}
	// 第二次调用不应改写 a（独立的底层数组）
	if hex.EncodeToString(a) != hex.EncodeToString(snapshot) {
		t.Error("第二次 SecureRandom 改写了第一次返回的切片内容（应各自独立）")
	}
	// 两次结果也应不同（前面已验过，这里再断言一次以明确语义）
	if hex.EncodeToString(a) == hex.EncodeToString(b) {
		t.Error("两次 16 字节随机数相同，违反随机性")
	}
}

// TestSecureRandomHexEncodable 返回的字节必须能被 HexEncode 正确编码（合法字节流）。
// 间接验证返回的不是非法值（如未初始化的内存指针等）。
func TestSecureRandomHexEncodable(t *testing.T) {
	b, err := SecureRandom(8)
	if err != nil {
		t.Fatalf("SecureRandom(8): %v", err)
	}
	h := HexEncode(b)
	if len(h) != 16 {
		t.Errorf("HexEncode 后长度应为 16，实际 %d", len(h))
	}
	// 往返：解码回来等于原字节
	dec, err := HexDecode(h)
	if err != nil {
		t.Fatalf("HexDecode(%q): %v", h, err)
	}
	if hex.EncodeToString(dec) != hex.EncodeToString(b) {
		t.Error("Hex 往返不等，返回的可能不是合法字节流")
	}
}

// TestRandomDemo 运行 RandomDemo，验证返回结果非空、长度为 32、Hex 长度 64。
func TestRandomDemo(t *testing.T) {
	r, err := RandomDemo()
	if err != nil {
		t.Fatalf("RandomDemo() 出错: %v", err)
	}
	if r.N != 32 {
		t.Errorf("RandomDemo N 应为 32，实际 %d", r.N)
	}
	if len(r.Raw) != 32 {
		t.Errorf("RandomDemo Raw 长度应为 32，实际 %d", len(r.Raw))
	}
	if len(r.Hex) != 64 {
		t.Errorf("RandomDemo Hex 长度应为 64，实际 %d", len(r.Hex))
	}
	// Hex 必须与 Raw 对应
	if HexEncode(r.Raw) != r.Hex {
		t.Error("RandomDemo 的 Hex 与 Raw 不一致")
	}
}
