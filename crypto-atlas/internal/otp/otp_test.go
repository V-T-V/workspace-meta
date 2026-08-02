package otp

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestEncryptRoundTrip(t *testing.T) {
	// 核心性质：Encrypt 生成的密钥能精确还原明文。
	cases := [][]byte{
		[]byte("Hello, OTP!"),
		[]byte("The quick brown fox jumps over the lazy dog"),
		bytes.Repeat([]byte{0xAB}, 256), // 较长输入
		{0x00, 0x01, 0x02, 0x03, 0xFF},
	}
	for _, plain := range cases {
		ct, key, err := Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		// 铁律：密钥长度必须等于明文长度。
		if len(key) != len(plain) {
			t.Errorf("密钥长度 %d != 明文长度 %d", len(key), len(plain))
		}
		// 密文长度也等于明文长度（XOR 不改变长度）。
		if len(ct) != len(plain) {
			t.Errorf("密文长度 %d != 明文长度 %d", len(ct), len(plain))
		}
		pt := Decrypt(ct, key)
		if !bytes.Equal(pt, plain) {
			t.Errorf("解密失败: got %v, want %v", pt, plain)
		}
	}
}

func TestEncryptKeyLengthEqualsPlaintext(t *testing.T) {
	// 密钥长度校验：每次加密密钥都恰好等于明文长度（不循环、不截断）。
	for _, n := range []int{1, 7, 32, 100} {
		plain := bytes.Repeat([]byte{0x41}, n)
		_, key, err := Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt 长度 %d: %v", n, err)
		}
		if len(key) != n {
			t.Errorf("明文长度 %d 时密钥长度应为 %d，实际 %d", n, n, len(key))
		}
	}
}

func TestEncryptProducesUniqueKeys(t *testing.T) {
	// 真随机性：同一明文加密两次，密钥和密文都应不同。
	plain := []byte("same message, different keys")
	_, key1, err := Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	_, key2, err := Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(key1, key2) {
		t.Error("两次加密的密钥相同——crypto/rand 应产出真随机")
	}
}

func TestDifferentKeyDecryptFails(t *testing.T) {
	// 用错误密钥（哪怕长度正确）解密 → 得不到原文（OTP 对任意密钥都"成功"
	// 解出某个结果，但不会等于原文）。
	plain := []byte("secret message")
	ct, _, err := Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	// 生成一个不同的密钥（全 0xFF，几乎不可能等于真随机密钥）
	wrongKey := bytes.Repeat([]byte{0xFF}, len(plain))
	pt := Decrypt(ct, wrongKey)
	if bytes.Equal(pt, plain) {
		t.Error("错误密钥不应能解出原文")
	}
}

func TestEncryptWithKeyRoundTrip(t *testing.T) {
	plain := []byte("fixed key roundtrip")
	// 构造与明文严格等长的密钥（避免手数字节数出错）。
	key := bytes.Repeat([]byte{0x5A}, len(plain))
	ct, err := EncryptWithKey(plain, key)
	if err != nil {
		t.Fatalf("EncryptWithKey: %v", err)
	}
	pt := Decrypt(ct, key)
	if !bytes.Equal(pt, plain) {
		t.Errorf("EncryptWithKey 往返失败: got %v want %v", pt, plain)
	}
}

func TestEncryptWithKeyLengthMismatch(t *testing.T) {
	// 密钥长度 != 明文长度 → 报 ErrKeyLengthMismatch（OTP 绝不循环复用）。
	plain := []byte("plaintext")
	cases := []struct {
		name string
		key  []byte
	}{
		{"短密钥", []byte("short")},
		{"长密钥", []byte("this key is much longer than plaintext")},
		{"空密钥", []byte{}},
	}
	for _, c := range cases {
		_, err := EncryptWithKey(plain, c.key)
		if !errors.Is(err, ErrKeyLengthMismatch) {
			t.Errorf("%s: 应返回 ErrKeyLengthMismatch，实际 %v", c.name, err)
		}
	}
}

func TestEmptyInput(t *testing.T) {
	// 空明文：Encrypt 成功，返回空密钥、空密文，解密也是空。
	ct, key, err := Encrypt([]byte{})
	if err != nil {
		t.Fatalf("空明文 Encrypt: %v", err)
	}
	if len(key) != 0 || len(ct) != 0 {
		t.Errorf("空明文应产出空密钥/密文，got key=%v ct=%v", key, ct)
	}
	if pt := Decrypt(ct, key); len(pt) != 0 {
		t.Errorf("空输入解密应返回空，got %v", pt)
	}

	// EncryptWithKey 空 + 空也合法（0 == 0）。
	out, err := EncryptWithKey([]byte{}, []byte{})
	if err != nil {
		t.Errorf("空+空 EncryptWithKey 不应报错: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("空+空应返回空切片")
	}
}

func TestCiphertextRevealsNothing(t *testing.T) {
	// OTP 安全性直觉：密文各字节应近似均匀分布（不像 ASCII 明文那样集中在 0x20-0x7e）。
	// 这里统计密文字节值的范围；真随机密钥 XOR 后密文不应全落在可打印 ASCII 区间。
	plain := bytes.Repeat([]byte("A"), 1000) // 明文全是 0x41
	ct, _, err := Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	// 数有多少字节落在可打印 ASCII [0x20, 0x7e] 外
	var nonAscii int
	for _, b := range ct {
		if b < 0x20 || b > 0x7e {
			nonAscii++
		}
	}
	// 真随机下约 37% 字节落在 [0x20,0x7e] 外（95/256 ≈ 0.37）。
	// 1000 字节里非 ASCII 应明显 > 0（密文不是另一段 "AAAA..."）。
	if nonAscii < 100 {
		t.Errorf("密文过于集中在可打印 ASCII 区间（非 ASCII 仅 %d/1000），疑似密钥非真随机", nonAscii)
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.Decrypted, r.Plaintext) {
		t.Error("demo 解密应还原明文")
	}
	if r.CiphertextHex == "" || r.KeyHex == "" {
		t.Error("demo 应输出密文和密钥的十六进制")
	}
	// 密文不应等于明文（密钥不可能恰好全 0）。
	if bytes.Equal(r.Ciphertext, r.Plaintext) {
		t.Error("密文不应等于明文")
	}
}
