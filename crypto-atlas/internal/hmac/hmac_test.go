package hmac

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"testing"
)

// TestHMACMatchesStdLib 用 Go 标准库的 crypto/hmac 交叉验证。
func TestHMACMatchesStdLib(t *testing.T) {
	cases := []struct {
		key, msg string
	}{
		{"key", "hello"},
		{"secret", "message"},
		{"a-very-long-key-that-exceeds-the-block-size-of-64-bytes-for-sure-yes", "data"},
		{"", "empty key"},
		{"k", ""},
	}
	for _, c := range cases {
		got := Compute([]byte(c.key), []byte(c.msg))
		want := hmac.New(sha256.New, []byte(c.key))
		want.Write([]byte(c.msg))
		wantBytes := want.Sum(nil)
		if !equalBytes(got, wantBytes) {
			t.Errorf("HMAC(%q, %q) 与标准库不一致\n  got:  %x\n  want: %x", c.key, c.msg, got, wantBytes)
		}
	}
}

func TestVerifySuccess(t *testing.T) {
	key, msg := []byte("secret"), []byte("hello")
	mac := Compute(key, msg)
	if !Verify(key, msg, mac) {
		t.Error("正确密钥 + 原消息应验证通过")
	}
}

func TestVerifyWrongKey(t *testing.T) {
	key, msg := []byte("secret"), []byte("hello")
	mac := Compute(key, msg)
	if Verify([]byte("wrong"), msg, mac) {
		t.Error("错误密钥应验证失败")
	}
}

func TestVerifyTamperedMessage(t *testing.T) {
	key, msg := []byte("secret"), []byte("hello")
	mac := Compute(key, msg)
	if Verify(key, []byte("Hello"), mac) {
		t.Error("篡改消息应验证失败")
	}
}

func TestVerifyTamperedMAC(t *testing.T) {
	key, msg := []byte("secret"), []byte("hello")
	mac := Compute(key, msg)
	mac[0] ^= 0xFF // 篡改 MAC
	if Verify(key, msg, mac) {
		t.Error("篡改 MAC 应验证失败")
	}
}

func TestConstantTimeCompare(t *testing.T) {
	if !constantTimeCompare([]byte("abc"), []byte("abc")) {
		t.Error("相等应返回 true")
	}
	if constantTimeCompare([]byte("abc"), []byte("abd")) {
		t.Error("不等应返回 false")
	}
	if constantTimeCompare([]byte("abc"), []byte("ab")) {
		t.Error("不同长度应返回 false")
	}
}

func TestLongKeyHashed(t *testing.T) {
	// 长密钥（>64 字节）应先哈希再填充
	longKey := make([]byte, 100)
	for i := range longKey {
		longKey[i] = byte(i)
	}
	msg := []byte("test")
	got := Compute(longKey, msg)
	want := hmac.New(sha256.New, longKey)
	want.Write(msg)
	if !equalBytes(got, want.Sum(nil)) {
		t.Error("长密钥 HMAC 应与标准库一致")
	}
}

func TestDeterministic(t *testing.T) {
	key, msg := []byte("k"), []byte("m")
	m1 := Compute(key, msg)
	m2 := Compute(key, msg)
	if !equalBytes(m1, m2) {
		t.Error("HMAC 应确定性（同输入同输出）")
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !r.Verified {
		t.Error("原消息应验证通过")
	}
	if !r.Tampered {
		t.Error("篡改应被检测到（Tampered 应为 true）")
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHMACTamperProof(t *testing.T) {
	key := []byte("secret")
	msg := []byte("important message")
	mac := Compute(key, msg)

	// 1. 正确验证
	if !Verify(key, msg, mac) {
		t.Error("正确密钥+原消息应验证通过")
	}
	// 2. 篡改消息
	if Verify(key, []byte("tampered"), mac) {
		t.Error("篡改消息应验证失败")
	}
	// 3. 篡改 MAC
	tamperedMAC := make([]byte, len(mac))
	copy(tamperedMAC, mac)
	tamperedMAC[0] ^= 0xFF
	if Verify(key, msg, tamperedMAC) {
		t.Error("篡改 MAC 应验证失败")
	}
	// 4. 错误密钥
	if Verify([]byte("wrong"), msg, mac) {
		t.Error("错误密钥应验证失败")
	}
}
