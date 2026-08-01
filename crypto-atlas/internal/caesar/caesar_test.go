package caesar

import (
	"context"
	"testing"
)

func TestEncrypt(t *testing.T) {
	cases := []struct {
		plain, want string
		key         int
	}{
		{"ABC", "DEF", 3},
		{"XYZ", "ABC", 3},     // 环绕
		{"abc", "def", 3},     // 小写
		{"Hello", "Khoor", 3}, // 混合大小写
		{"ABC", "ABC", 0},     // key=0 不变
		{"ABC", "ABC", 26},    // key=26 等于 0
		{"DEF", "ABC", -3},    // 负 key
		{"Hi!", "Kl!", 3},     // 非字母不变
	}
	for _, c := range cases {
		if got := Encrypt(c.plain, c.key); got != c.want {
			t.Errorf("Encrypt(%q,%d) = %q, want %q", c.plain, c.key, got, c.want)
		}
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// 加密后解密应还原
	for _, key := range []int{0, 1, 3, 13, 25, 26, 100, -5} {
		plain := "The Quick Brown Fox!"
		ct := Encrypt(plain, key)
		pt := Decrypt(ct, key)
		if pt != plain {
			t.Errorf("key=%d 往返失败: %q → %q → %q", key, plain, ct, pt)
		}
	}
}

func TestNonAlphaUnchanged(t *testing.T) {
	// 数字、标点、空格不变
	s := "123 !@# 中"
	if got := Encrypt(s, 5); got != s {
		t.Errorf("非英文字符应不变: %q → %q", s, got)
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Plaintext == r.Ciphertext {
		t.Error("密文不应等于明文")
	}
	if r.Decrypted != r.Plaintext {
		t.Error("解密应还原明文")
	}
}

func TestNegativeKeyNormalization(t *testing.T) {
	// -3 和 23 等价
	if Encrypt("A", -3) != Encrypt("A", 23) {
		t.Error("-3 应等价于 23")
	}
}
