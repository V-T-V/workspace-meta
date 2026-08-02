// Package rsa 手写实现 RSA 公钥密码（教学版）。
//
// RSA 是现代公钥密码的基石：用一对数学上绑定的密钥——公钥加密/验签、
// 私钥解密/签名——解决了对称密码"如何安全分发密钥"的难题。
//
// 本包不使用标准库 crypto/rsa，而是用 math/big 直接展示 RSA 的数学原理：
//
//	N = p * q               （两个大素数之积）
//	φ(N) = (p-1)(q-1)        （欧拉函数）
//	E   与 φ(N) 互素         （公钥指数，常取 17 / 65537）
//	D = E^-1 mod φ(N)        （私钥指数，模逆元）
//
//	加密：c = m^E mod N       （公钥加密，任何人都能做）
//	解密：m = c^D mod N       （私钥解密，只有持有 D 的人能做）
//	签名：s = m^D mod N       （私钥签名，等价于"用私钥加密"）
//	验签：m == s^E mod N      （公钥验签）
//
// 安全性根基：分解大整数 N 难，但已知 p/q 算 φ(N) 和 D 容易。
// 因此公钥 (N, E) 可以公开，私钥 (N, D) 必须保密；分解 N 即等价于破解私钥。
//
// 注意：教学用小素数（p=61, q=53 → N=3233）极不安全，仅用于演示数学；
// 实际 RSA 用 2048/3072/4096 位的 N，且配合填充方案（OAEP / PSS）使用。
package rsa

import (
	"context"
	"errors"
	"fmt"
	"math/big"
)

// 公钥指数默认值。教学取 17（教材 RSA-3233 经典示例）。
const DefaultE = 17

// PublicKey 是 RSA 公钥，可公开给任何人：用来加密、验签。
type PublicKey struct {
	N *big.Int // 模数，p*q 之积
	E *big.Int // 公钥指数
}

// PrivateKey 是 RSA 私钥，必须保密：用来解密、签名。
type PrivateKey struct {
	N *big.Int // 模数（与公钥相同）
	D *big.Int // 私钥指数，E 对 φ(N) 的模逆元
}

// 公共错误。
var (
	ErrMessageTooLarge = errors.New("rsa: 明文 m 必须满足 0 <= m < N")
	ErrNotCoprime      = errors.New("rsa: 公钥指数 E 必须与 φ(N)=(p-1)(q-1) 互素")
	ErrNonPrime        = errors.New("rsa: p、q 必须是素数")
)

// GenerateKey 用两个素数 p、q 生成一对 RSA 密钥。
//
// 公钥指数取 DefaultE（=17），要求 gcd(E, (p-1)(q-1)) == 1，
// 否则返回 ErrNotCoprime（模逆元不存在，无法构造 D）。
//
// 教学示例：p=61, q=53 → N=3233, φ=3120, E=17, D=2753。
func GenerateKey(p, q int64) (PublicKey, PrivateKey, error) {
	bp := big.NewInt(p)
	bq := big.NewInt(q)

	// 素性检查（教学：用 ProbablyPrime 足够，不追求密码学级确定性）。
	if !bp.ProbablyPrime(20) || !bq.ProbablyPrime(20) {
		return PublicKey{}, PrivateKey{}, ErrNonPrime
	}

	N := new(big.Int).Mul(bp, bq) // N = p * q

	// φ(N) = (p-1)*(q-1)
	pMinus1 := new(big.Int).Sub(bp, big.NewInt(1))
	qMinus1 := new(big.Int).Sub(bq, big.NewInt(1))
	phi := new(big.Int).Mul(pMinus1, qMinus1)

	E := big.NewInt(DefaultE)

	// 要求 gcd(E, φ) == 1，否则 E 在 mod φ 下无逆元。
	gcd := new(big.Int).GCD(nil, nil, E, phi)
	if gcd.Cmp(big.NewInt(1)) != 0 {
		return PublicKey{}, PrivateKey{}, ErrNotCoprime
	}

	// D = E^-1 mod φ(N)。ModInverse 返回 nil 表示无解（前面已挡，理论不会到这里）。
	D := new(big.Int).ModInverse(E, phi)
	if D == nil {
		return PublicKey{}, PrivateKey{}, ErrNotCoprime
	}

	return PublicKey{N: N, E: E}, PrivateKey{N: N, D: D}, nil
}

// Encrypt 用公钥加密明文 m：c = m^E mod N。
//
// 要求 0 <= m < N（否则无法完整还原，返回 ErrMessageTooLarge）。
// 真实 RSA 还需对 m 做随机填充（OAEP）以抵抗多种攻击，本教学版省略。
func Encrypt(m *big.Int, pub PublicKey) (*big.Int, error) {
	if m.Sign() < 0 || m.Cmp(pub.N) >= 0 {
		return nil, ErrMessageTooLarge
	}
	// m^E mod N —— math/big.Exp 的第 3 个参数非 nil 时即做模幂。
	return new(big.Int).Exp(m, pub.E, pub.N), nil
}

// Decrypt 用私钥解密密文 c：m = c^D mod N。
//
// 与 Encrypt 互逆：Decrypt(Encrypt(m, pub), priv) == m。
func Decrypt(c *big.Int, priv PrivateKey) (*big.Int, error) {
	if c.Sign() < 0 || c.Cmp(priv.N) >= 0 {
		return nil, ErrMessageTooLarge
	}
	return new(big.Int).Exp(c, priv.D, priv.N), nil
}

// Sign 用私钥对消息 m 签名：s = m^D mod N。
//
// 注意：真实场景下应对 m 先做哈希（如 SHA-256）再签摘要，
// 否则易受存在性伪造攻击；本教学版直接对数值签名以展示数学。
func Sign(m *big.Int, priv PrivateKey) (*big.Int, error) {
	if m.Sign() < 0 || m.Cmp(priv.N) >= 0 {
		return nil, ErrMessageTooLarge
	}
	return new(big.Int).Exp(m, priv.D, priv.N), nil
}

// Verify 用公钥验证签名：s^E mod N 是否等于 m。
//
// 返回 true 表示签名有效（未被篡改、确为对应私钥所签）。
func Verify(m, sig *big.Int, pub PublicKey) bool {
	if m.Sign() < 0 || m.Cmp(pub.N) >= 0 {
		return false
	}
	if sig.Sign() < 0 || sig.Cmp(pub.N) >= 0 {
		return false
	}
	recovered := new(big.Int).Exp(sig, pub.E, pub.N)
	return recovered.Cmp(m) == 0
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Pub       PublicKey
	Priv      PrivateKey
	Message   *big.Int // 明文数值
	Cipher    *big.Int // 加密结果
	Decrypted *big.Int // 解密结果，应等于 Message
	Signature *big.Int // 签名
	Verified  bool     // 验签结果
}

// Demo 用经典教材参数演示 RSA 完整流程：
//
//	p=61, q=53 → N=3233, E=17, D=2753
//	加密 'A'(=65) → 解密还原 65
//	对 65 签名 → 公钥验签通过
//
// 全程确定性（固定参数），便于教学复现。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	pub, priv, err := GenerateKey(61, 53)
	if err != nil {
		return nil, err
	}

	m := big.NewInt(65) // 字符 'A' 的 ASCII 码
	c, err := Encrypt(m, pub)
	if err != nil {
		return nil, err
	}
	pt, err := Decrypt(c, priv)
	if err != nil {
		return nil, err
	}
	sig, err := Sign(m, priv)
	if err != nil {
		return nil, err
	}
	ok := Verify(m, sig, pub)

	fmt.Println("=== RSA 公钥密码 demo ===")
	fmt.Printf("素数 p=61, q=53\n")
	fmt.Printf("公钥:  N=%s, E=%d\n", pub.N.String(), pub.E)
	fmt.Printf("私钥:  N=%s, D=%s\n", priv.N.String(), priv.D.String())
	fmt.Printf("明文 m = %s (字符 'A')\n", m.String())
	fmt.Printf("加密 c = m^E mod N = %s\n", c.String())
	fmt.Printf("解密 m = c^D mod N = %s\n", pt.String())
	fmt.Printf("签名 s = m^D mod N = %s\n", sig.String())
	fmt.Printf("验签 s^E mod N == m ? %v\n", ok)

	return &DemoResult{
		Pub: pub, Priv: priv,
		Message: m, Cipher: c, Decrypted: pt,
		Signature: sig, Verified: ok,
	}, nil
}
