package sha256

import (
	"context"
	"encoding/hex"
	"testing"
)

// 已知的 SHA-256 测试向量（标准库与各实现一致，可作交叉验证）。
var knownVectors = map[string]string{
	"":      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	"hello": "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
	"Hello": "185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969",
	"The quick brown fox jumps over the lazy dog": "d7a8fbb307d7809469ca9abcb0082e4f8d5651e46d3cdb762d02d0bf37c9e592",
}

func TestHashHexKnownVectors(t *testing.T) {
	for in, want := range knownVectors {
		got := HashHex([]byte(in))
		if got != want {
			t.Errorf("HashHex(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestHashFixedLength(t *testing.T) {
	// 定长输出：无论输入多长，摘要恒为 32 字节
	for _, in := range []string{"", "a", "hello", string(make([]byte, 10000))} {
		h := Hash([]byte(in))
		if len(h) != 32 {
			t.Errorf("SHA-256 摘要应恒为 32 字节, 输入 len=%d 得到 %d", len(in), len(h))
		}
	}
}

func TestHashHexLength(t *testing.T) {
	// hex 表示恒为 64 字符
	for _, in := range []string{"", "a", "hello world"} {
		hx := HashHex([]byte(in))
		if len(hx) != 64 {
			t.Errorf("SHA-256 hex 应为 64 字符, got %d (%q)", len(hx), hx)
		}
	}
}

func TestAvalanche(t *testing.T) {
	// 雪崩效应：仅差 1 位大小写的输入，摘要应完全不同
	a := HashHex([]byte("hello"))
	b := HashHex([]byte("Hello"))
	if a == b {
		t.Error("hello 与 Hello 的 SHA-256 不应相同（雪崩效应）")
	}
}

func TestDeterministic(t *testing.T) {
	// 同输入同输出
	a := HashHex([]byte("deterministic input"))
	b := HashHex([]byte("deterministic input"))
	if a != b {
		t.Error("SHA-256 应确定性")
	}
}

func TestHashConsistentWithHex(t *testing.T) {
	// Hash 与 HashHex 应一致
	h := Hash([]byte("abc"))
	hx := HashHex([]byte("abc"))
	if hex.EncodeToString(h[:]) != hx {
		t.Error("Hash 与 HashHex 不一致")
	}
}

func TestEmptyInput(t *testing.T) {
	// 空输入也有确定摘要
	h := Hash([]byte{})
	if len(h) != 32 {
		t.Error("空输入摘要仍应为 32 字节")
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) < 3 {
		t.Fatal("demo 应至少演示 3 组输入")
	}
	// 每组都应是 32 字节 / 64 hex
	for _, it := range r.Items {
		if it.ByteLen != 32 || len(it.HashHex) != 64 {
			t.Errorf("条目 %q 字节数=%d hexlen=%d 不合法", it.Input, it.ByteLen, len(it.HashHex))
		}
	}
	// hello 与 Hello 在 demo 中应不同（雪崩）
	var helloHash, HelloHash string
	for _, it := range r.Items {
		switch it.Input {
		case "hello":
			helloHash = it.HashHex
		case "Hello":
			HelloHash = it.HashHex
		}
	}
	if helloHash == "" || HelloHash == "" {
		t.Fatal("demo 应同时包含 hello 和 Hello")
	}
	if helloHash == HelloHash {
		t.Error("demo 应展示雪崩效应：hello 与 Hello 摘要应不同")
	}
}
