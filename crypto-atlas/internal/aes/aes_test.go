package aes

import (
	"bytes"
	"context"
	"testing"
)

// TestECBRoundTrip 验证 ECB 加密后解密能还原（覆盖 AES-128/192/256）。
func TestECBRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		key  []byte // 16/24/32 字节
	}{
		{"AES-128", bytes.Repeat([]byte{0x01}, 16)},
		{"AES-192", bytes.Repeat([]byte{0x02}, 24)},
		{"AES-256", bytes.Repeat([]byte{0x03}, 32)},
	}
	plain := []byte("attack at dawn!!") // 15 字节，会触发 PKCS7 填充补 1 字节
	for _, c := range cases {
		ct, err := EncryptECB(plain, c.key)
		if err != nil {
			t.Fatalf("%s: EncryptECB 出错: %v", c.name, err)
		}
		pt, err := DecryptECB(ct, c.key)
		if err != nil {
			t.Fatalf("%s: DecryptECB 出错: %v", c.name, err)
		}
		if !bytes.Equal(pt, plain) {
			t.Errorf("%s: 往返失败 got %v want %v", c.name, pt, plain)
		}
	}
}

// TestCBCRoundTrip 验证 CBC 加密后解密能还原。
func TestCBCRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x0a}, 16) // AES-128
	iv := bytes.Repeat([]byte{0x0b}, 16)
	for _, plain := range [][]byte{
		[]byte(""),                // 空（补一整块）
		[]byte("exactly16bytes!"), // 16 字节（补一整块）
		[]byte("less than block"), // 15 字节
		[]byte("this is a longer multi-block plaintext msg!!!"), // 43 字节
	} {
		ct, err := EncryptCBC(plain, key, iv)
		if err != nil {
			t.Fatalf("EncryptCBC 出错: %v", err)
		}
		pt, err := DecryptCBC(ct, key, iv)
		if err != nil {
			t.Fatalf("DecryptCBC 出错: %v", err)
		}
		if !bytes.Equal(pt, plain) {
			t.Errorf("CBC 往返失败 got %q want %q", pt, plain)
		}
	}
}

// TestECBRepeatedPlaintextProducesRepeatedCiphertext 是 ECB 的核心缺陷验证：
// 相同明文块 → 相同密文块。
func TestECBRepeatedPlaintextProducesRepeatedCiphertext(t *testing.T) {
	key := bytes.Repeat([]byte{0x0c}, 16)
	// 两块完全相同的 16 字节明文
	plain := bytes.Repeat([]byte{'A'}, 32)

	ct, err := EncryptECB(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	b1, b2 := ct[:blockSize], ct[blockSize:2*blockSize]
	if !bytes.Equal(b1, b2) {
		t.Errorf("ECB 缺陷未复现：相同明文块应产生相同密文块\n块1=%x\n块2=%x", b1, b2)
	}
}

// TestCBCRepeatedPlaintextBreaksPattern 验证 CBC 消除了 ECB 的模式泄露：
// 相同明文块，因 IV/链式 XOR，密文块不同。
func TestCBCRepeatedPlaintextBreaksPattern(t *testing.T) {
	key := bytes.Repeat([]byte{0x0d}, 16)
	iv := []byte("0123456789abcdef") // 16 字节
	plain := bytes.Repeat([]byte{'A'}, 32)

	ct, err := EncryptCBC(plain, key, iv)
	if err != nil {
		t.Fatal(err)
	}
	b1, b2 := ct[:blockSize], ct[blockSize:2*blockSize]
	if bytes.Equal(b1, b2) {
		t.Errorf("CBC 不应让相同明文块产生相同密文块（块1==块2）")
	}
}

// TestInvalidKeyLength 验证非法密钥长度报错。
func TestInvalidKeyLength(t *testing.T) {
	badKeys := [][]byte{
		nil,
		[]byte("short"),                 // 5 字节
		bytes.Repeat([]byte{0xff}, 15),  // 15
		bytes.Repeat([]byte{0xff}, 17),  // 17
		bytes.Repeat([]byte{0xff}, 100), // 100
	}
	for _, k := range badKeys {
		if _, err := EncryptECB([]byte("x"), k); err == nil {
			t.Errorf("len(key)=%d 应报错，实际 nil", len(k))
		}
		if _, err := DecryptECB([]byte("0123456789abcdef"), k); err == nil {
			t.Errorf("len(key)=%d 应报错，实际 nil", len(k))
		}
		if _, err := EncryptCBC([]byte("x"), k, make([]byte, 16)); err == nil {
			t.Errorf("len(key)=%d 应报错，实际 nil", len(k))
		}
	}
}

// TestInvalidIVLength 验证 CBC 模式 IV 长度校验。
func TestInvalidIVLength(t *testing.T) {
	key := bytes.Repeat([]byte{0x00}, 16)
	for _, ivLen := range []int{0, 1, 15, 17, 32} {
		if _, err := EncryptCBC([]byte("x"), key, make([]byte, ivLen)); err == nil {
			t.Errorf("iv 长度 %d 应报错", ivLen)
		}
		if _, err := DecryptCBC(make([]byte, 16), key, make([]byte, ivLen)); err == nil {
			t.Errorf("iv 长度 %d 应报错", ivLen)
		}
	}
}

// TestCiphertextLengthAlignment 验证非法密文长度（非 blockSize 倍数）报错。
func TestCiphertextLengthAlignment(t *testing.T) {
	key := bytes.Repeat([]byte{0x00}, 16)
	iv := make([]byte, 16)
	for _, ctLen := range []int{1, 5, 15, 17, 33} {
		if _, err := DecryptECB(make([]byte, ctLen), key); err == nil {
			t.Errorf("ECB 密文长度 %d 应报错", ctLen)
		}
		if _, err := DecryptCBC(make([]byte, ctLen), key, iv); err == nil {
			t.Errorf("CBC 密文长度 %d 应报错", ctLen)
		}
	}
}

// TestWrongKeyFailsDecryption 验证用错误的 key 解密：
// 要么 PKCS7 校验失败报错，要么解出的明文与原文不符（绝不可能正确还原）。
func TestWrongKeyFailsDecryption(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 16)
	wrongKey := bytes.Repeat([]byte{0x22}, 16)
	plain := []byte("secret message!")
	iv := make([]byte, 16)
	ct, _ := EncryptCBC(plain, key, iv)

	got, err := DecryptCBC(ct, wrongKey, iv)
	if err == nil && bytes.Equal(got, plain) {
		t.Error("用错误 key 解密竟得到正确明文（不可能）")
	}
}

// TestDemoRuns 验证 Demo 可运行且结果符合教学断言。
func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.KeyBits != 128 {
		t.Errorf("demo 应使用 AES-128，实际 %d 位", r.KeyBits)
	}
	// ECB 必须复现模式泄露
	if !r.ECB.RepeatedPattern {
		t.Error("ECB demo 应复现相同明文块→相同密文块")
	}
	// CBC 必须打破模式泄露
	if r.CBC.RepeatedPattern {
		t.Error("CBC demo 中相同明文块不应产生相同密文块")
	}
	// 往还原成功：ECB/CBC 的解密都应等于明文
	if r.ECB.DecryptedHex != r.ECB.PlaintextHex {
		t.Error("ECB 解密未还原明文")
	}
	if r.CBC.DecryptedHex != r.CBC.PlaintextHex {
		t.Error("CBC 解密未还原明文")
	}
}
