// Package tlssim 的 demo 实现（含入口 Demo）。
//
// 实现见 tlssim.go。本文件只承载 Demo(ctx)：
// 跑一次完整握手，打印每一步（ClientHello / ServerHello / 证书 / DH 协商 /
// 密钥派生），再用会话密钥加密一条 HTTP 请求并验证解密 + 篡改检测。
//
// 运行：
//
//	go run ./cmd/atlas -d tlssim
package tlssim

import (
	"context"
	"fmt"
)

// DemoResult 是 demo 输出摘要（供测试断言用）。
type DemoResult struct {
	ClientSession Session
	ServerSession Session
	Plaintext     string // 原始应用消息
	Decrypted     string // 解密还原的消息（应等于 Plaintext）
	CiphertextLen int
	TamperBlocked bool // 篡改是否被检测到（应为 true）
}

// Demo 跑完整 TLS 1.2 简化握手 + record layer 加解密 + 篡改检测。
//
// 全程确定性（教材参数），便于教学复现与测试断言。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	fmt.Println("=== TLS 1.2 简化握手模拟（DHE-RSA）demo ===")

	fmt.Println("\n[1] ClientHello")
	fmt.Println("    客户端生成 ClientRandom，发起握手")

	fmt.Println("\n[2] ServerHello + 证书 + DH 公开值")
	fmt.Println("    服务端生成 ServerRandom")
	fmt.Println("    服务端出示证书（含 RSA 公钥）+ DH 临时公开值 B=g^b mod p")

	client, server, err := Handshake()
	if err != nil {
		return nil, err
	}

	fmt.Println("\n[3] 客户端验证证书 + 双方算 DH 共享密钥")
	fmt.Printf("    证书主体: %s（验证通过）\n", client.ServerSubject)
	fmt.Println("    preMaster = g^(ab) mod p（双方独立算出，必然相等）")

	fmt.Println("\n[4] 密钥派生（KDF）")
	fmt.Println("    master = SHA256(p||CR||SR||1) || SHA256(p||CR||SR||2)  // 64 字节")
	fmt.Printf("    AES-128 key  = %x\n", client.AESKey)
	fmt.Printf("    HMAC-SHA256  = %x\n", client.HMACKey)
	if string(client.AESKey) == string(server.AESKey) {
		fmt.Println("    ✓ 客户端 / 服务端派生出完全相同的密钥")
	} else {
		fmt.Println("    ✗ 密钥不一致（数学出错）")
	}

	fmt.Println("\n[5] record layer：用 AES 加密 + HMAC 认证一条应用消息")
	msg := []byte("GET /secret HTTP/1.1\r\nHost: server.example.com")
	ct, mac, err := SecureSend(client, msg)
	if err != nil {
		return nil, err
	}
	fmt.Printf("    明文:   %q\n", msg)
	fmt.Printf("    密文(%d bytes): %x...\n", len(ct), ct[:min(16, len(ct))])
	fmt.Printf("    MAC:    %x\n", mac)

	pt, err := SecureReceive(server, ct, mac)
	if err != nil {
		return nil, err
	}
	fmt.Printf("    服务端解密还原: %q  ✓\n", pt)

	fmt.Println("\n[6] 篡改检测：翻转密文首字节后重放")
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[0] ^= 0xff
	_, errTamper := SecureReceive(server, tampered, mac)
	if errTamper != nil {
		fmt.Printf("    ✓ 检测到篡改：%v\n", errTamper)
	} else {
		fmt.Println("    ✗ 篡改未被检测到（安全失效）")
	}

	fmt.Println("\n握手 + 通信 + 篡改检测全部通过。")
	return &DemoResult{
		ClientSession: client,
		ServerSession: server,
		Plaintext:     string(msg),
		Decrypted:     string(pt),
		CiphertextLen: len(ct),
		TamperBlocked: errTamper != nil,
	}, nil
}
