package des

import (
	"bytes"
	"context"
	"testing"
)

// TestRoundTrip 验证 DES 加密后解密能还原（覆盖多种明文长度，触发 PKCS7 填充边界）。
func TestRoundTrip(t *testing.T) {
	key := []byte("password") // 8 字节
	cases := [][]byte{
		[]byte(""),                               // 空（补一整块 8 字节）
		[]byte("hi"),                             // < 8
		[]byte("1234567"),                        // 7 字节（差 1 字节补齐）
		[]byte("12345678"),                       // 恰好 8 字节（补一整块）
		[]byte("two blocks msg!"),                // 16 字节（补一整块）
		[]byte("longer than two blocks here!!!"), // 32+ 字节
	}
	for _, plain := range cases {
		ct, err := Encrypt(plain, key)
		if err != nil {
			t.Fatalf("Encrypt 出错: %v", err)
		}
		// 密文长度必须是 blockSize 的整数倍
		if len(ct)%blockSize != 0 || len(ct) == 0 {
			t.Errorf("密文长度 %d 非 %d 倍数或为空", len(ct), blockSize)
		}
		pt, err := Decrypt(ct, key)
		if err != nil {
			t.Fatalf("Decrypt 出错: %v", err)
		}
		if !bytes.Equal(pt, plain) {
			t.Errorf("往返失败 got %q want %q", pt, plain)
		}
	}
}

// TestBlockSizeIsEight 验证 DES 分组大小为 8 字节（对比 AES 的 16）。
func TestBlockSizeIsEight(t *testing.T) {
	if blockSize != 8 {
		t.Errorf("DES 分组应为 8 字节，实际 %d", blockSize)
	}
}

// TestInvalidKeyLength 验证非法密钥长度报错。
func TestInvalidKeyLength(t *testing.T) {
	badKeys := [][]byte{
		nil,
		[]byte("short"),                // 5 字节
		bytes.Repeat([]byte{0xff}, 7),  // 7
		bytes.Repeat([]byte{0xff}, 9),  // 9
		bytes.Repeat([]byte{0xff}, 16), // 16（这是 AES-128 的长度，对 DES 非法）
		bytes.Repeat([]byte{0xff}, 32), // 32
	}
	for _, k := range badKeys {
		if _, err := Encrypt([]byte("x"), k); err == nil {
			t.Errorf("len(key)=%d 应报错", len(k))
		}
		if _, err := Decrypt(bytes.Repeat([]byte{0}, blockSize), k); err == nil {
			t.Errorf("len(key)=%d 应报错", len(k))
		}
	}
}

// TestValidKeyLengths 验证只有 8 字节密钥合法。
func TestValidKeyLengths(t *testing.T) {
	for _, n := range []int{7, 9, 16} {
		k := bytes.Repeat([]byte{0x01}, n)
		if _, err := Encrypt([]byte("x"), k); err == nil {
			t.Errorf("len(key)=%d 对 DES 应非法", n)
		}
	}
	// 8 字节应成功
	if _, err := Encrypt([]byte("x"), bytes.Repeat([]byte{0x01}, 8)); err != nil {
		t.Errorf("len(key)=8 应合法，got %v", err)
	}
}

// TestCiphertextLengthAlignment 验证非法密文长度（非 8 倍数）报错。
func TestCiphertextLengthAlignment(t *testing.T) {
	key := []byte("password")
	for _, ctLen := range []int{1, 5, 7, 9, 15} {
		if _, err := Decrypt(make([]byte, ctLen), key); err == nil {
			t.Errorf("密文长度 %d 应报错", ctLen)
		}
	}
}

// TestWrongKeyFailsDecryption 验证用错误 key 解密不会得到正确明文。
func TestWrongKeyFailsDecryption(t *testing.T) {
	key := []byte("password")
	wrongKey := []byte("WRONGKEY!")[:8] // 同为 8 字节但内容不同
	plain := []byte("top secret")
	ct, _ := Encrypt(plain, key)

	got, err := Decrypt(ct, wrongKey)
	if err == nil && bytes.Equal(got, plain) {
		t.Error("错误 key 不应解出正确明文")
	}
}

// TestPKCS7PaddingAddsFullBlock 验证恰好 blockSize 倍数明文会补一整块。
func TestPKCS7PaddingAddsFullBlock(t *testing.T) {
	key := []byte("password")
	plain := bytes.Repeat([]byte{'x'}, blockSize) // 恰好 8 字节
	ct, err := Encrypt(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct) != 2*blockSize {
		t.Errorf("恰好 %d 字节明文应补一整块→密文 %d 字节，实际 %d", blockSize, 2*blockSize, len(ct))
	}
}

// TestDemoRuns 验证 Demo 可运行且断言成立。
func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.KeyBits != 56 {
		t.Errorf("DES 有效密钥应为 56 位，实际 %d", r.KeyBits)
	}
	if r.BlockSizeBytes != 8 {
		t.Errorf("DES 块应为 8 字节，实际 %d", r.BlockSizeBytes)
	}
	if r.AESBlockSize != 16 {
		t.Errorf("对照 AES 块应为 16 字节，实际 %d", r.AESBlockSize)
	}
	if r.Decrypted != r.Plaintext {
		t.Error("Demo 解密未还原明文")
	}
}
