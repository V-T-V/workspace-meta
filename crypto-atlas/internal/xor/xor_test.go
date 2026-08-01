package xor

import (
	"bytes"
	"context"
	"testing"

	"github.com/QiuShichang/crypto-atlas/internal/core"
)

func TestEncrypt(t *testing.T) {
	cases := []struct {
		name    string
		plain   []byte
		key     []byte
		wantHex string
	}{
		{
			name:    "Hello XOR key (循环复用)",
			plain:   []byte("Hello"),
			key:     []byte("key"),
			wantHex: "230015070a", // H^k=0x23 e^e=0x00 l^y=0x15 l^k=0x07 o^e=0x0a
		},
		{
			name:    "等长密钥",
			plain:   []byte{0x01, 0x02, 0x03},
			key:     []byte{0xFF, 0xFF, 0xFF},
			wantHex: "fefdfc",
		},
		{
			name:    "密钥长于明文",
			plain:   []byte("AB"),
			key:     []byte("KEY"),
			wantHex: core.HexEncode([]byte{0x41 ^ 'K', 0x42 ^ 'E'}),
		},
	}
	for _, c := range cases {
		got := Encrypt(c.plain, c.key)
		if core.HexEncode(got) != c.wantHex {
			t.Errorf("%s: Encrypt = %s, want %s", c.name, core.HexEncode(got), c.wantHex)
		}
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// 循环复用 + 等长 + 长密钥 都应能往返还原
	plain := []byte("The quick brown fox jumps over the lazy dog")
	for _, key := range [][]byte{
		[]byte("key"),
		[]byte("k"), // 单字节密钥
		[]byte("LongerKeyThanPlaintextSometimes"),
	} {
		ct := Encrypt(plain, key)
		pt := Decrypt(ct, key)
		if !bytes.Equal(pt, plain) {
			t.Errorf("key=%q 往返失败: %v → %v → %v", key, plain, ct, pt)
		}
	}
}

func TestDecryptEqualsEncrypt(t *testing.T) {
	// XOR 自反：解密 == 加密（同函数）
	plain := []byte("Hello")
	key := []byte("secret")
	if !bytes.Equal(Encrypt(plain, key), Decrypt(plain, key)) {
		t.Error("对 XOR，Encrypt 与 Decrypt 应是同一操作")
	}
}

func TestEmptyKey(t *testing.T) {
	// 空 key 等价于不加密：原样返回（拷贝，非别名）
	plain := []byte{1, 2, 3}
	got := Encrypt(plain, []byte{})
	if !bytes.Equal(got, plain) {
		t.Errorf("空 key 应原样返回: %v", got)
	}
	// 改动返回值不应影响原 plain（证明是拷贝）
	got[0] = 99
	if plain[0] == 99 {
		t.Error("空 key 路径应返回拷贝，不能别名原切片")
	}
}

func TestEmptyInput(t *testing.T) {
	got := Encrypt([]byte{}, []byte("key"))
	if len(got) != 0 {
		t.Error("空明文应返回空切片")
	}
}

func TestDeterministic(t *testing.T) {
	plain := []byte("Attack at dawn")
	key := []byte("LEMON")
	a := Encrypt(plain, key)
	b := Encrypt(plain, key)
	if !bytes.Equal(a, b) {
		t.Error("加密应确定性")
	}
}

func TestKeyReuseLeak(t *testing.T) {
	// 已知明文攻击：P XOR C = K（密钥泄露），这是 XOR 不安全的本质。
	// 密钥循环复用时，攻击者拿到 (明文, 密文) 对，能恢复出与明文等长的
	// "密钥流"（即 key 循环）；其前 len(key) 字节正是原始 key。
	plain := []byte("Hello")
	key := []byte("key")
	ct := Encrypt(plain, key)
	keystream := core.XorBytes(plain, ct) // 长度 = len(plain) = 5
	// keystream 应等于 key 循环到 5 字节："keyke"
	wantStream := []byte{'k', 'e', 'y', 'k', 'e'}
	if !bytes.Equal(keystream, wantStream) {
		t.Errorf("恢复的密钥流 = %v, want %v", keystream, wantStream)
	}
	// 前 len(key) 字节即原始 key
	if !bytes.Equal(keystream[:len(key)], key) {
		t.Errorf("已知明文攻击应恢复原始密钥: got %v want %v", keystream[:len(key)], key)
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.Decrypted, r.Plaintext) {
		t.Error("解密应还原明文")
	}
	if r.CiphertextHex == "" {
		t.Error("应输出十六进制密文")
	}
	// "Hello" XOR "key" 的密文不应等于明文（否则就是空 key 路径）
	if bytes.Equal(r.Ciphertext, r.Plaintext) {
		t.Error("密文不应等于明文")
	}
}
