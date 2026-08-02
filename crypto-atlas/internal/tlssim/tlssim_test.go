package tlssim

import (
	"bytes"
	"testing"

	"github.com/QiuShichang/crypto-atlas/internal/hmac"
)

func TestHandshakeBothSidesEqualAESKey(t *testing.T) {
	// 核心断言：握手后双方独立派生的 AES key 必须逐字节相同。
	client, server, err := Handshake()
	if err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	if !bytes.Equal(client.AESKey, server.AESKey) {
		t.Fatalf("双方 AES key 不一致：\n client=%x\n server=%x", client.AESKey, server.AESKey)
	}
	if !bytes.Equal(client.HMACKey, server.HMACKey) {
		t.Fatalf("双方 HMAC key 不一致：\n client=%x\n server=%x", client.HMACKey, server.HMACKey)
	}
	// AES key 必须是合法长度（16/24/32）。
	if len(client.AESKey) != 16 {
		t.Errorf("AES key 长度 = %d, want 16", len(client.AESKey))
	}
	if len(client.HMACKey) != 32 {
		t.Errorf("HMAC key 长度 = %d, want 32", len(client.HMACKey))
	}
}

func TestHandshakeDeterministic(t *testing.T) {
	// 同样输入（默认教材随机数）→ 同样密钥，可复现。
	c1, s1, _ := Handshake()
	c2, s2, _ := Handshake()
	if !bytes.Equal(c1.AESKey, c2.AESKey) {
		t.Error("相同随机数应派生相同 AES key")
	}
	if !bytes.Equal(s1.HMACKey, s2.HMACKey) {
		t.Error("相同随机数应派生相同 HMAC key")
	}
}

func TestHandshakeDifferentRandomsDifferentKeys(t *testing.T) {
	// 不同随机数 → 不同 master → 不同密钥（防重放/防预测的意义）。
	c1, _, _ := HandshakeWithSeeds([]byte("seed-A"), []byte("seed-A"))
	c2, _, _ := HandshakeWithSeeds([]byte("seed-B"), []byte("seed-B"))
	if bytes.Equal(c1.AESKey, c2.AESKey) {
		t.Error("不同随机数应派生不同 AES key")
	}
}

func TestHandshakeRandomsCaptured(t *testing.T) {
	// Session 应保留双方随机数（参与 KDF，便于上层审计/重放检测）。
	c, _, _ := HandshakeWithSeeds([]byte("client-rand"), []byte("server-rand"))
	if string(c.ClientRandom) != "client-rand" {
		t.Errorf("ClientRandom = %q, want %q", c.ClientRandom, "client-rand")
	}
	if string(c.ServerRandom) != "server-rand" {
		t.Errorf("ServerRandom = %q, want %q", c.ServerRandom, "server-rand")
	}
}

func TestSecureCommunication(t *testing.T) {
	// 端到端：握手 → 客户端加密 → 服务端解密 → 还原明文。
	client, server, err := Handshake()
	if err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	msg := []byte("GET /secret HTTP/1.1\r\nHost: server.example.com")

	ct, mac, err := SecureSend(client, msg)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if len(ct) == 0 || len(mac) == 0 {
		t.Fatal("密文 / MAC 不应为空")
	}

	pt, err := SecureReceive(server, ct, mac)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if !bytes.Equal(pt, msg) {
		t.Errorf("还原明文不匹配：got %q, want %q", pt, msg)
	}
}

func TestTamperDetectionCiphertext(t *testing.T) {
	// 篡改密文 → MAC 验证失败（Encrypt-then-MAC：MAC 覆盖密文）。
	client, server, _ := Handshake()
	ct, mac, _ := SecureSend(client, []byte("balance: 100"))

	// 翻转密文第一个字节。
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[0] ^= 0xff

	if _, err := SecureReceive(server, tampered, mac); err == nil {
		t.Error("篡改密文后应 MAC 验证失败，但解密成功")
	}
}

func TestTamperDetectionMAC(t *testing.T) {
	// 替换 MAC → 验证失败。
	client, server, _ := Handshake()
	ct, _, _ := SecureSend(client, []byte("hello"))

	fakeMAC := make([]byte, len(makeMAC(server, ct)))
	for i := range fakeMAC {
		fakeMAC[i] = 0xAA
	}
	if _, err := SecureReceive(server, ct, fakeMAC); err == nil {
		t.Error("替换 MAC 后应验证失败")
	}
}

func TestTamperDetectionWrongKey(t *testing.T) {
	// 用错的会话密钥（另一条会话）验证 → 必失败。
	// 两条不同随机数的会话密钥不同。
	client1, _, _ := HandshakeWithSeeds([]byte("s1"), []byte("s1"))
	_, server2, _ := HandshakeWithSeeds([]byte("s2"), []byte("s2"))

	ct, mac, _ := SecureSend(client1, []byte("cross-session"))
	if _, err := SecureReceive(server2, ct, mac); err == nil {
		t.Error("跨会话（不同密钥）应 MAC 验证失败")
	}
}

// makeMAC 用会话密钥对密文算 HMAC（仅测试辅助：复现合法 MAC 格式）。
func makeMAC(s Session, ct []byte) []byte {
	// 与 SecureSend 内部一致：HMAC over ciphertext。
	return hmac.Compute(s.HMACKey, ct)
}
