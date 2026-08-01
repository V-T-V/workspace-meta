package md5

import (
	"context"
	"encoding/hex"
	"testing"
)

// 已知的 MD5 测试向量。
var knownVectors = map[string]string{
	"":      "d41d8cd98f00b204e9800998ecf8427e",
	"hello": "5d41402abc4b2a76b9719d911017c592",
	"Hello": "8b1a9953c4611296a827abf8c47804d7",
	"The quick brown fox jumps over the lazy dog": "9e107d9d372bb6826bd81d3542a419d6",
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
	// 定长输出：恒为 16 字节
	for _, in := range []string{"", "a", "hello", string(make([]byte, 10000))} {
		h := Hash([]byte(in))
		if len(h) != 16 {
			t.Errorf("MD5 摘要应恒为 16 字节, 输入 len=%d 得到 %d", len(in), len(h))
		}
	}
}

func TestHashHexLength(t *testing.T) {
	// hex 表示恒为 32 字符
	for _, in := range []string{"", "a", "hello world"} {
		hx := HashHex([]byte(in))
		if len(hx) != 32 {
			t.Errorf("MD5 hex 应为 32 字符, got %d (%q)", len(hx), hx)
		}
	}
}

func TestAvalanche(t *testing.T) {
	// 雪崩效应：仅差 1 位大小写，摘要应完全不同
	a := HashHex([]byte("hello"))
	b := HashHex([]byte("Hello"))
	if a == b {
		t.Error("hello 与 Hello 的 MD5 不应相同（雪崩效应）")
	}
}

func TestDeterministic(t *testing.T) {
	a := HashHex([]byte("deterministic input"))
	b := HashHex([]byte("deterministic input"))
	if a != b {
		t.Error("MD5 应确定性")
	}
}

func TestHashConsistentWithHex(t *testing.T) {
	h := Hash([]byte("abc"))
	hx := HashHex([]byte("abc"))
	if hex.EncodeToString(h[:]) != hx {
		t.Error("Hash 与 HashHex 不一致")
	}
}

func TestEmptyInput(t *testing.T) {
	h := Hash([]byte{})
	if len(h) != 16 {
		t.Error("空输入摘要仍应为 16 字节")
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
	for _, it := range r.Items {
		if it.ByteLen != 16 || len(it.HashHex) != 32 {
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
