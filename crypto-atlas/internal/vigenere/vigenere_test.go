package vigenere

import (
	"context"
	"testing"
)

func TestEncrypt(t *testing.T) {
	cases := []struct {
		plain, key, want string
	}{
		{"ATTACKATDAWN", "LEMON", "LXFOPVEFRNHR"}, // 经典例子（逐位推导：N+E=R）
		{"helloworld", "key", "rijvsuyvjn"},       // Wikipedia 权威例子（小写）
		{"ABC", "A", "ABC"},                       // key=A 等价于位移 0
		{"ABC", "B", "BCD"},                       // key=B 每位移 1
		{"HELLO", "abc", "HFNLP"},                 // 密钥大小写不敏感
		{"Hello, World!", "KEY", "Rijvs, Uyvjn!"}, // 非字母保留、不消耗密钥
		{"ABC", "lemon123", "LFO"},                // 密钥中非字母被丢弃（A+L=L,B+E=F,C+M=O）
		{"", "LEMON", ""},                         // 空明文
	}
	for _, c := range cases {
		if got := Encrypt(c.plain, c.key); got != c.want {
			t.Errorf("Encrypt(%q,%q) = %q, want %q", c.plain, c.key, got, c.want)
		}
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// 加密后解密应还原（包括混合大小写、非字母、长明文短密钥）
	plain := "The Quick Brown Fox Jumps Over The Lazy Dog! 123"
	for _, key := range []string{"LEMON", "A", "Z", "abc", "Key", "longkey"} {
		ct := Encrypt(plain, key)
		pt := Decrypt(ct, key)
		if pt != plain {
			t.Errorf("key=%q 往返失败: %q → %q → %q", key, plain, ct, pt)
		}
	}
}

func TestKeyCaseInsensitive(t *testing.T) {
	// 大小写不同的等价密钥应产出相同密文
	if Encrypt("ATTACK", "LEMON") != Encrypt("ATTACK", "lemon") {
		t.Error("LEMON 和 lemon 应产出相同密文")
	}
	if Encrypt("ATTACK", "LeMoN") != Encrypt("ATTACK", "LEMON") {
		t.Error("LeMoN 和 LEMON 应产出相同密文")
	}
}

func TestNonAlphaKeyFallback(t *testing.T) {
	// 密钥去掉非字母后为空：原样返回明文（不加密）
	plain := "Hello World"
	if got := Encrypt(plain, "123!@#"); got != plain {
		t.Errorf("空密钥应原样返回: %q", got)
	}
	if got := Encrypt(plain, ""); got != plain {
		t.Errorf("空密钥应原样返回: %q", got)
	}
}

func TestEmptyInput(t *testing.T) {
	if got := Encrypt("", "LEMON"); got != "" {
		t.Errorf("空明文应返回空: %q", got)
	}
	if got := Decrypt("", "LEMON"); got != "" {
		t.Errorf("空密文应返回空: %q", got)
	}
}

func TestDeterministic(t *testing.T) {
	// 同输入同输出
	a := Encrypt("Attack at dawn", "LEMON")
	b := Encrypt("Attack at dawn", "LEMON")
	if a != b {
		t.Error("加密应确定性")
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Ciphertext != "LXFOPVEFRNHR" {
		t.Errorf("经典 demo 期望 LXFOPVEFRNHR, got %q", r.Ciphertext)
	}
	if r.Decrypted != r.Plaintext {
		t.Error("解密应还原明文")
	}
}
