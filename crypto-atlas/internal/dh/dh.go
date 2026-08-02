// Package dh 手写实现 Diffie-Hellman 密钥交换（教学版）。
//
// Diffie-Hellman 解决了一个看似矛盾的问题：双方**从不共享任何秘密**，
// 却能在被窃听的信道上协商出一把**只有他俩知道**的共享密钥。
//
// 数学骨架（全部运算 mod p）：
//
//	公开参数：大素数 p、生成元 g（任何人都可见）
//	Alice 私钥 a（保密）  →  公钥 A = g^a mod p
//	Bob   私钥 b（保密）  →  公钥 B = g^b mod p
//
//	双方互换公钥 A、B（可被窃听），然后各自计算：
//	  Alice: s = B^a mod p = (g^b)^a = g^(ab) mod p
//	  Bob:   s = A^b mod p = (g^a)^b = g^(ab) mod p
//
//	因为 (ab) 可交换，双方得到同一个 s —— 这就是共享密钥。
//
// 安全性根基：**离散对数难题（DLP）**。窃听者看到 p、g、A=g^a、B=g^b，
// 想算出 a（或 b）等价于解 `g^x ≡ A (mod p)`，这在 p 足够大时是难的。
// 算不出 a/b，就得不到 g^(ab)，密钥保密。
//
// 本包用经典教材参数 p=23, g=5（确定性 demo：a=6, b=15 → 共享密钥=2）
// 演示原理；真实 DH 用几千位的 p，且常配合签名/证书防中间人。
package dh

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

// 经典教材公共参数（确定性 demo）。
const (
	DemoP = 23 // 素数
	DemoG = 5  // 生成元
)

// GenerateParams 返回教学用的公共参数 (p, g)。
//
// 用固定小素数 p=23、生成元 g=5（教材经典示例），保证 demo 可复现。
// 真实场景应选密码学级的素数（如 RFC 7919 的 FFDHE 群）。
func GenerateParams() (p, g *big.Int) {
	return big.NewInt(DemoP), big.NewInt(DemoG)
}

// PrivateKey 生成一个随机私钥 x，满足 1 <= x < p。
//
// 内部用 crypto/rand 保证密码学随机性。Demo 场景若需可复现，
// 直接传固定值即可（参见 Demo 中 a=6, b=15 的用法）。
func PrivateKey(p *big.Int) (*big.Int, error) {
	// rand.Int 返回 [0, p) 内均匀随机数；私钥不能为 0，故取 [1, p)。
	one := big.NewInt(1)
	upper := new(big.Int).Sub(p, one) // upper = p-1，范围 [0, p-1)
	x, err := rand.Int(rand.Reader, upper)
	if err != nil {
		return nil, fmt.Errorf("dh: 生成私钥失败: %w", err)
	}
	x.Add(x, one) // 平移到 [1, p-1] ⊂ [1, p)
	return x, nil
}

// PublicKey 由私钥派生公钥：Y = g^private mod p。
//
// 公钥可公开传输；窃听者无法从 Y 反推 private（离散对数难题）。
func PublicKey(private, g, p *big.Int) *big.Int {
	return new(big.Int).Exp(g, private, p)
}

// SharedSecret 计算共享密钥：s = theirPublic^myPrivate mod p。
//
// Alice 持有 a 收到 Bob 的 B 后算 B^a；
// Bob 持有 b 收到 Alice 的 A 后算 A^b。
// 两者结果都是 g^(ab) mod p，必然相等。
func SharedSecret(theirPublic, myPrivate, p *big.Int) *big.Int {
	return new(big.Int).Exp(theirPublic, myPrivate, p)
}

// PeerSession 封装一方的 DH 交换状态，便于演示流程。
type PeerSession struct {
	Name    string   // "Alice" / "Bob"
	P       *big.Int // 公共素数
	G       *big.Int // 生成元
	Private *big.Int // 私钥（保密）
	Public  *big.Int // 公钥（可公开）
}

// NewPeer 用给定私钥构造一方会话（确定性，教学用）。
func NewPeer(name string, p, g, private *big.Int) *PeerSession {
	return &PeerSession{
		Name:    name,
		P:       new(big.Int).Set(p),
		G:       new(big.Int).Set(g),
		Private: new(big.Int).Set(private),
		Public:  PublicKey(private, g, p),
	}
}

// Shared 计算对方公钥与本方私钥导出的共享密钥。
func (s *PeerSession) Shared(theirPublic *big.Int) *big.Int {
	return SharedSecret(theirPublic, s.Private, s.P)
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	P, G        *big.Int
	Alice       *PeerSession
	Bob         *PeerSession
	AliceShared *big.Int // Alice 算出的共享密钥
	BobShared   *big.Int // Bob   算出的共享密钥
}

// Demo 用经典教材参数演示 DH 完整流程：
//
//	p=23, g=5
//	Alice a=6 → A=5^6 mod 23=8
//	Bob   b=15 → B=5^15 mod 23=19
//	共享密钥：Alice 算 19^6 mod 23=2，Bob 算 8^15 mod 23=2（相等！）
//
// 全程确定性（固定 a/b），便于教学复现。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	p, g := GenerateParams() // p=23, g=5

	alice := NewPeer("Alice", p, g, big.NewInt(6)) // a=6
	bob := NewPeer("Bob", p, g, big.NewInt(15))    // b=15

	aliceShared := alice.Shared(bob.Public) // B^a = 19^6 mod 23
	bobShared := bob.Shared(alice.Public)   // A^b = 8^15 mod 23

	fmt.Println("=== Diffie-Hellman 密钥交换 demo ===")
	fmt.Printf("公共参数: p=%s, g=%s\n", p.String(), g.String())
	fmt.Printf("Alice 私钥 a=%s → 公钥 A=g^a mod p=%s\n", alice.Private, alice.Public)
	fmt.Printf("Bob   私钥 b=%s → 公钥 B=g^b mod p=%s\n", bob.Private, bob.Public)
	fmt.Printf("Alice 算共享密钥 = B^a mod p = %s\n", aliceShared)
	fmt.Printf("Bob   算共享密钥 = A^b mod p = %s\n", bobShared)
	if aliceShared.Cmp(bobShared) == 0 {
		fmt.Printf("✓ 双方共享密钥相等：%s\n", aliceShared)
	} else {
		fmt.Println("✗ 双方密钥不相等（数学出错）")
	}

	return &DemoResult{
		P: p, G: g,
		Alice: alice, Bob: bob,
		AliceShared: aliceShared,
		BobShared:   bobShared,
	}, nil
}
