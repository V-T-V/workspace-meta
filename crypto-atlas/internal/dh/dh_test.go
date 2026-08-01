package dh

import (
	"context"
	"math/big"
	"testing"
)

func TestGenerateParams(t *testing.T) {
	p, g := GenerateParams()
	if p.Int64() != 23 {
		t.Errorf("p = %d, want 23", p.Int64())
	}
	if g.Int64() != 5 {
		t.Errorf("g = %d, want 5", g.Int64())
	}
}

func TestPublicKey_BookValues(t *testing.T) {
	// 教材值：A=5^6 mod 23=8，B=5^15 mod 23=19
	p, g := GenerateParams()
	A := PublicKey(big.NewInt(6), g, p)
	B := PublicKey(big.NewInt(15), g, p)
	if A.Int64() != 8 {
		t.Errorf("A=5^6 mod 23 = %d, want 8", A.Int64())
	}
	if B.Int64() != 19 {
		t.Errorf("B=5^15 mod 23 = %d, want 19", B.Int64())
	}
}

func TestSharedSecret_BothSidesEqual(t *testing.T) {
	// 核心：双方算出的共享密钥必须相等。
	p, g := GenerateParams()
	A := PublicKey(big.NewInt(6), g, p)  // Alice 公钥
	B := PublicKey(big.NewInt(15), g, p) // Bob   公钥

	sAlice := SharedSecret(B, big.NewInt(6), p) // B^a
	sBob := SharedSecret(A, big.NewInt(15), p)  // A^b

	if sAlice.Cmp(sBob) != 0 {
		t.Fatalf("共享密钥不相等: Alice=%d, Bob=%d", sAlice, sBob)
	}
	if sAlice.Int64() != 2 {
		t.Errorf("共享密钥 = %d, want 2", sAlice.Int64())
	}
}

func TestSharedSecret_DifferentPrivatesDifferentSecret(t *testing.T) {
	// 不同私钥 → 不同共享密钥。
	p, g := GenerateParams()
	// 第一对 a=6, b=15
	A1 := PublicKey(big.NewInt(6), g, p)
	s1 := SharedSecret(A1, big.NewInt(15), p)
	// 第二对 a'=7（不同）
	A2 := PublicKey(big.NewInt(7), g, p)
	s2 := SharedSecret(A2, big.NewInt(15), p)
	if s1.Cmp(s2) == 0 {
		t.Error("不同私钥应导出不同共享密钥")
	}
}

func TestSharedSecret_Commutativity(t *testing.T) {
	// g^(ab) = g^(ba)：交换 a/b 顺序共享密钥不变。
	p, g := GenerateParams()
	a, b := big.NewInt(6), big.NewInt(15)
	s1 := SharedSecret(PublicKey(b, g, p), a, p) // B^a
	s2 := SharedSecret(PublicKey(a, g, p), b, p) // A^b
	if s1.Cmp(s2) != 0 {
		t.Error("DH 共享密钥应满足交换律 g^(ab)=g^(ba)")
	}
}

func TestPrivateKey_Range(t *testing.T) {
	// 私钥应在 [1, p) 内，且非 0。
	p, _ := GenerateParams()
	for i := 0; i < 50; i++ {
		x, err := PrivateKey(p)
		if err != nil {
			t.Fatalf("PrivateKey: %v", err)
		}
		if x.Sign() <= 0 || x.Cmp(p) >= 0 {
			t.Errorf("私钥 %d 不在 [1,%d) 内", x, p)
		}
	}
}

func TestPeerSession_BookValues(t *testing.T) {
	p, g := GenerateParams()
	alice := NewPeer("Alice", p, g, big.NewInt(6))
	bob := NewPeer("Bob", p, g, big.NewInt(15))
	if alice.Public.Int64() != 8 {
		t.Errorf("Alice 公钥 = %d, want 8", alice.Public)
	}
	if bob.Public.Int64() != 19 {
		t.Errorf("Bob 公钥 = %d, want 19", bob.Public)
	}
	sAlice := alice.Shared(bob.Public)
	sBob := bob.Shared(alice.Public)
	if sAlice.Cmp(sBob) != 0 || sAlice.Int64() != 2 {
		t.Errorf("双方共享密钥 = %d / %d，应相等且为 2", sAlice, sBob)
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.AliceShared.Cmp(r.BobShared) != 0 {
		t.Fatal("demo 中双方共享密钥应相等")
	}
	if r.AliceShared.Int64() != 2 {
		t.Errorf("demo 共享密钥 = %d, want 2", r.AliceShared.Int64())
	}
}
