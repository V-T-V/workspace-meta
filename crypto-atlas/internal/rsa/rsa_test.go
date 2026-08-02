package rsa

import (
	"context"
	"math/big"
	"testing"
)

// mustKey 生成教学用 (p=61, q=53) 密钥对，失败即中断测试。
func mustKey(t *testing.T) (PublicKey, PrivateKey) {
	t.Helper()
	pub, priv, err := GenerateKey(61, 53)
	if err != nil {
		t.Fatalf("GenerateKey 失败: %v", err)
	}
	return pub, priv
}

func TestGenerateKey_BookValues(t *testing.T) {
	// 经典教材值：p=61, q=53 → N=3233, E=17, D=2753
	pub, priv, err := GenerateKey(61, 53)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if pub.N.Int64() != 3233 {
		t.Errorf("N = %d, want 3233", pub.N.Int64())
	}
	if pub.E.Int64() != 17 {
		t.Errorf("E = %d, want 17", pub.E.Int64())
	}
	if priv.D.Int64() != 2753 {
		t.Errorf("D = %d, want 2753", priv.D.Int64())
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	pub, priv := mustKey(t)
	// 遍历若干 m（含边界 0 和 N-1），加解密应完整还原。
	cases := []int64{0, 1, 2, 65, 100, 3232}
	for _, m := range cases {
		mi := big.NewInt(m)
		c, err := Encrypt(mi, pub)
		if err != nil {
			t.Fatalf("Encrypt(%d): %v", m, err)
		}
		got, err := Decrypt(c, priv)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got.Cmp(mi) != 0 {
			t.Errorf("m=%d: 解密得 %d，往返失败", m, got.Int64())
		}
	}
}

func TestEncrypt_TooLarge(t *testing.T) {
	pub, _ := mustKey(t)
	// m == N（等于 N）应被拒绝（不在 0..N-1 内）。
	if _, err := Encrypt(big.NewInt(3233), pub); err != ErrMessageTooLarge {
		t.Errorf("m==N 应报 ErrMessageTooLarge，got %v", err)
	}
	// 负数也应被拒绝。
	if _, err := Encrypt(big.NewInt(-1), pub); err != ErrMessageTooLarge {
		t.Errorf("m<0 应报 ErrMessageTooLarge，got %v", err)
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	pub, priv := mustKey(t)
	m := big.NewInt(65)
	c, _ := Encrypt(m, pub)

	// 篡改密文：c' = c + 1。解密结果不再等于原 m。
	tampered := new(big.Int).Add(c, big.NewInt(1))
	got, err := Decrypt(tampered, priv)
	if err != nil {
		t.Fatalf("Decrypt 篡改后不应报错（会解出错的值）: %v", err)
	}
	if got.Cmp(m) == 0 {
		t.Error("篡改密文后竟解出原明文，说明 RSA 没有完整性保护（这正是需填充/签名的理由）")
	}
}

func TestSignVerify(t *testing.T) {
	pub, priv := mustKey(t)
	m := big.NewInt(123)
	sig, err := Sign(m, priv)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !Verify(m, sig, pub) {
		t.Error("合法签名应验签通过")
	}
}

func TestVerify_TamperedMessage(t *testing.T) {
	pub, priv := mustKey(t)
	sig, _ := Sign(big.NewInt(123), priv)
	// 把消息改一个数，签名应验不过。
	if Verify(big.NewInt(124), sig, pub) {
		t.Error("篡改消息后验签应失败")
	}
}

func TestVerify_TamperedSignature(t *testing.T) {
	pub, priv := mustKey(t)
	m := big.NewInt(123)
	sig, _ := Sign(m, priv)
	bad := new(big.Int).Add(sig, big.NewInt(1))
	if Verify(m, bad, pub) {
		t.Error("篡改签名后验签应失败")
	}
}

func TestSignEqualsDecryptShape(t *testing.T) {
	// 签名与"用私钥解密"在数学上是同一个操作：m^D mod N。
	pub, priv := mustKey(t)
	m := big.NewInt(42)
	sig, _ := Sign(m, priv)
	dec, _ := Decrypt(m, priv) // 注意：这里把 m 当密文"解密"
	if sig.Cmp(dec) != 0 {
		t.Error("Sign(m) 应等于 Decrypt(m)（同为 m^D mod N）")
	}
	_ = pub
}

func TestDifferentKeyPairs(t *testing.T) {
	// 不同素数 → 不同 N/D，密文不可互换。
	pub1, priv1, err := GenerateKey(61, 53)
	if err != nil {
		t.Fatal(err)
	}
	pub2, priv2, err := GenerateKey(71, 73)
	if err != nil {
		t.Fatal(err)
	}
	if pub1.N.Cmp(pub2.N) == 0 {
		t.Error("不同素数对应不应得到相同 N")
	}
	// 用 key1 加密，用 key2 解密应失败（解出的值 ≠ 明文）。
	m := big.NewInt(50)
	c, _ := Encrypt(m, pub1)
	wrong, err := Decrypt(c, priv2)
	if err != nil {
		t.Fatalf("跨 key 解密不应报错: %v", err)
	}
	if wrong.Cmp(m) == 0 {
		t.Error("用错误私钥解出了明文——密钥隔离失败")
	}
	_ = priv1
}

func TestGenerateKey_NonPrime(t *testing.T) {
	// 非素数应被拒绝（否则 RSA 数学不成立）。
	if _, _, err := GenerateKey(60, 53); err != ErrNonPrime {
		t.Errorf("非素数应报 ErrNonPrime，got %v", err)
	}
}

func TestGenerateKey_NotCoprime(t *testing.T) {
	// φ(N)=(p-1)(q-1)。若取 p=3, q=7 → φ=12，gcd(17,12)=1 没问题；
	// 要构造 gcd(E,φ)≠1，取 p=2, q=5 → φ=1*4=4，gcd(17,4)=1 仍可。
	// 取 p=3, q=19 → φ=2*18=36，gcd(17,36)=1。gcd(E,φ)=2 的例子：
	// p=3,q=5 → φ=2*4=8, gcd(17,8)=1 ❌。
	// 直接选 φ 含因子 17 的：(p-1)(q-1) 是 17 倍数，p=18+1=19（素），q 选使 q-1 含 17 不易；
	// 更稳：p=103(素), q=5(素) → φ=102*4=408=17*24，gcd(17,408)=17≠1。
	if _, _, err := GenerateKey(103, 5); err != ErrNotCoprime {
		t.Errorf("E 与 φ(N) 不互素应报 ErrNotCoprime，got %v", err)
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Decrypted.Int64() != 65 {
		t.Errorf("demo 解密应还原 65，got %d", r.Decrypted.Int64())
	}
	if !r.Verified {
		t.Error("demo 验签应通过")
	}
}

// TestSignatureDemo 验证 SignatureDemo 完整流程的所有断言：
//   - 正常消息验签通过
//   - 篡改消息验签失败
//   - HMAC 正常验证通过、篡改验证失败
//   - RSA 签名与解密同形（Sign(m) == m^D mod N，与 Decrypt(m) 一致）
func TestSignatureDemo(t *testing.T) {
	r, err := SignatureDemo(context.Background())
	if err != nil {
		t.Fatalf("SignatureDemo: %v", err)
	}

	// 步骤 3：正常验签必须通过。
	if !r.VerifiedValid {
		t.Error("正常消息验签应通过（VerifiedValid 应为 true）")
	}

	// 步骤 4：篡改消息验签必须失败。
	if r.VerifiedTampered {
		t.Error("篡改消息后验签应失败（VerifiedTampered 应为 false）")
	}

	// 篡改消息必须与原消息不同（否则上面的断言没意义）。
	if r.TamperedMessage.Cmp(r.Message) == 0 {
		t.Error("TamperedMessage 应不同于原 Message")
	}

	// 步骤 5：HMAC 正常验证通过、篡改失败。
	if !r.HmacVerified {
		t.Error("HMAC 正常验证应通过（HmacVerified 应为 true）")
	}
	if r.HmacTamperedVerified {
		t.Error("HMAC 篡改验证应失败（HmacTamperedVerified 应为 false）")
	}

	// HMAC MAC 应为非空的 hex 字符串（SHA-256 → 32 字节 → 64 hex 字符）。
	if len(r.HmacMAC) != 64 {
		t.Errorf("HMAC-SHA256 应为 64 个 hex 字符，实际 %d", len(r.HmacMAC))
	}

	// Comparison 文本不能为空（教学对比说明）。
	if r.Comparison == "" {
		t.Error("Comparison 不应为空")
	}

	// 交叉校验：Signature 应等于把 Message 当密文做 Decrypt 的结果
	// （两者都是 m^D mod N）。
	if dec, err := Decrypt(r.Message, r.Priv); err != nil {
		t.Fatalf("Decrypt 交叉校验失败: %v", err)
	} else if dec.Cmp(r.Signature) != 0 {
		t.Errorf("Sign(m) 应等于 Decrypt(m)（同为 m^D mod N），实际 Sign=%s Decrypt=%s",
			r.Signature.String(), dec.String())
	}
}
