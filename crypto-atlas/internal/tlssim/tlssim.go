// Package tlssim 模拟 TLS 1.2 握手的核心流程（教学版，非真实 TLS 实现）。
//
// 本包不实现网络、不实现 X.509、不实现真实 TLS 状态机；它把 TLS 1.2 握手
// 拆成 5 个清晰的阶段，复用本仓库已有的 rsa / dh / aes / hmac 四个教学包，
// 把"一次握手"变成"四个原语的串讲"。
//
// 简化握手流程（对应 TLS 1.2 的密钥交换 + record layer）：
//
//  1. ClientHello  —— 客户端生成随机数 ClientRandom
//  2. ServerHello  —— 服务端生成随机数 + 出示证书（RSA 公钥）+ DH 公开值
//  3. 密钥协商     —— 客户端验证证书；双方用 DH 算出共享密钥 preMaster
//  4. 密钥派生     —— master = SHA256(preMaster || ClientRandom || ServerRandom)
//     AESKey = master[0:16]   （AES-128 密钥）
//     HMACKey = master[16:48] （HMAC-SHA256 密钥）
//  5. record layer —— 用 AES 加密 + HMAC 认证后续应用数据
//
// 这是 TLS 1.2 的"DHE_RSA"密码套件的极简骨架：DHE 提供前向保密（每次握手
// 临时 DH，私钥不长期存），RSA 签名/证书提供身份认证（防中间人）。
// 真实 TLS 还涉及：PRF（HMAC-based，非单次 SHA256）、 Finished 消息握手摘要
// 验证、AEAD（GCM/ChaCha20-Poly1305 取代分离的 MAC）、版本/套件协商等。
//
// 零外部依赖，仅用标准库 crypto/sha256（KDF）+ 本仓库 rsa/dh/aes/hmac 包。
package tlssim

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"

	"github.com/QiuShichang/crypto-atlas/internal/aes"
	"github.com/QiuShichang/crypto-atlas/internal/dh"
	"github.com/QiuShichang/crypto-atlas/internal/hmac"
	"github.com/QiuShichang/crypto-atlas/internal/rsa"
)

// aesKeyLen 是派生出的 AES 密钥长度（AES-128 = 16 字节）。
const aesKeyLen = 16

// hmacKeyLen 是派生出的 HMAC 密钥长度（HMAC-SHA256 用 32 字节）。
const hmacKeyLen = 32

// 公共错误。
var (
	// ErrHandshakeFailed 在握手任一阶段（证书验证 / 密钥协商）失败时返回。
	ErrHandshakeFailed = errors.New("tlssim: 握手失败")
)

// Certificate 是服务端的"教学证书"：把 RSA 公钥与一个身份字符串绑定。
//
// 真实 TLS 用 X.509 证书（由 CA 签名、含有效期/扩展/吊销列表）。
// 这里极简成 (Subject, PublicKey) 二元组——身份与公钥的绑定关系本身
// 就需要可信第三方（CA）背书，本教学版省略 CA，假设绑定可信。
type Certificate struct {
	Subject  string        // 服务端身份（如 "server.example.com"）
	Pub      rsa.PublicKey // 服务端 RSA 公钥（用于验签 / 身份认证）
	Verified bool          // 客户端验证结果（demo 用）
}

// Session 是一次握手成功后双方共享的会话密钥材料。
//
// ClientRandom/ServerRandom 是握手开始时双方各自生成的随机数，
// 参与密钥派生以防重放；AESKey/HMACKey 是派生出的对称密钥，
// 用于 record layer 的加密 + 认证。两端 Session 应完全一致。
type Session struct {
	AESKey        []byte // record layer 加密密钥（AES-128，16 字节）
	HMACKey       []byte // record layer 认证密钥（HMAC-SHA256，32 字节）
	ServerRandom  []byte // 服务端随机数（防重放，参与 KDF）
	ClientRandom  []byte // 客户端随机数（防重放，参与 KDF）
	ServerSubject string // 服务端身份（认证结果，便于上层校验）
}

// clientState 封装客户端一侧的握手中间状态。
type clientState struct {
	random      []byte
	dhSession   *dh.PeerSession
	serverCert  Certificate
	serverPubDH *big.Int // 服务端 DH 公开值
	preMaster   []byte   // DH 共享密钥的字节表示
}

// serverState 封装服务端一侧的握手中间状态。
type serverState struct {
	random    []byte
	dhSession *dh.PeerSession
	cert      Certificate
	privDHExp *big.Int // 服务端 DH 私钥指数（用于算共享密钥）
	pubDH     *big.Int // 服务端 DH 公开值（随 ServerHello 发出）
}

// Handshake 模拟一次完整的 TLS 1.2 DHE-RSA 握手。
//
// 用确定性教材参数（rsa: p=61,q=53；dh: p=23,g=5,a=6,b=15），
// 让握手结果可复现、便于断言。返回客户端与服务端两份 Session——
// 它们的 AESKey/HMACKey 必须逐字节相等，这正是"双方独立算出同一把钥匙"
// 的 DH 奇迹，也是整个 TLS 密钥协商的安全核心。
//
// 返回 (clientSession, serverSession, error)。两份 Session 内容相同，
// 区别仅在视角（哪一方持有），demo 用以演示"两个独立方得到相同密钥"。
func Handshake() (Session, Session, error) {
	return HandshakeWithSeeds(nil, nil)
}

// HandshakeWithSeeds 允许传入客户端/服务端随机数种子（用于测试可复现）。
// seeds 为 nil 时用固定教材值；长度不限（参与 KDF，不直接做密钥）。
func HandshakeWithSeeds(clientSeed, serverSeed []byte) (Session, Session, error) {
	// 默认随机数（教材确定性值，便于测试断言；真实场景必须用 crypto/rand）。
	if clientSeed == nil {
		clientSeed = []byte("client-random-123456")
	}
	if serverSeed == nil {
		serverSeed = []byte("server-random-789012")
	}

	// ===== 阶段 1：客户端 ClientHello =====
	client := &clientState{random: clientSeed}

	// ===== 阶段 2：服务端 ServerHello + 证书 + DH 公开值 =====
	// 服务端生成 RSA 密钥对（这里即"证书"的公钥来源；真实由 CA 签发）。
	pubRSA, _, err := rsa.GenerateKey(61, 53)
	if err != nil {
		return Session{}, Session{}, fmt.Errorf("%w: 生成服务端 RSA 密钥: %v", ErrHandshakeFailed, err)
	}
	// 服务端 DH 临时密钥（DHE 的 E=ephemeral，每次握手重新生成 → 前向保密）。
	dhP, dhG := dh.GenerateParams()                            // p=23, g=5
	serverDH := dh.NewPeer("Server", dhP, dhG, big.NewInt(15)) // b=15

	server := &serverState{
		random:    serverSeed,
		cert:      Certificate{Subject: "server.example.com", Pub: pubRSA, Verified: true},
		dhSession: serverDH,
		privDHExp: serverDH.Private,
		pubDH:     serverDH.Public,
	}

	// ===== 阶段 3：客户端验证证书 + 双方算 DH 共享密钥 =====
	// 客户端验证服务端证书（教学版：信任绑定即可；真实需校验 CA 链 + 有效期 + 主机名）。
	if !server.cert.Verified {
		return Session{}, Session{}, fmt.Errorf("%w: 证书验证失败", ErrHandshakeFailed)
	}
	client.serverCert = server.cert
	client.serverPubDH = server.pubDH

	// 客户端自己也有一份 DH 临时密钥（a=6）。
	client.dhSession = dh.NewPeer("Client", dhP, dhG, big.NewInt(6))

	// 双方独立算 preMaster = g^(ab) mod p（DH 数学保证两侧相等）。
	clientShared := client.dhSession.Shared(server.pubDH)            // B^a
	serverShared := server.dhSession.Shared(client.dhSession.Public) // A^b
	if clientShared.Cmp(serverShared) != 0 {
		return Session{}, Session{}, fmt.Errorf("%w: 双方 DH 共享密钥不一致", ErrHandshakeFailed)
	}
	// DH 共享密钥 → 字节切片（preMaster secret）。
	client.preMaster = bigIntBytes(clientShared)

	// ===== 阶段 4：密钥派生 =====
	// 双块 KDF：master = SHA256(p||CR||SR||1) || SHA256(p||CR||SR||2)（64 字节）。
	// 取前 16 字节做 AES-128 密钥，紧接 32 字节做 HMAC 密钥。
	// （真实 TLS 用 HMAC-based PRF 多轮扩展，本教学版两轮 SHA256 足够展示"两端派生一致"。）
	clientSession := deriveKeys(client.preMaster, client.random, server.random, server.cert.Subject)
	serverSession := deriveKeys(client.preMaster, client.random, server.random, server.cert.Subject)
	_ = serverShared // 已通过相等性校验，不再单独保留

	// ===== 完成：返回两份对等的 Session =====
	return clientSession, serverSession, nil
}

// SecureSend 用会话密钥加密 + 认证一条应用消息，返回 (ciphertext, mac)。
//
// 这是 TLS record layer 的极简形态：先 AES-CBC 加密明文，再对密文算 HMAC。
// （真实 TLS 1.2 用 MAC-then-Encrypt 或 AEAD；此处简化为 Encrypt-then-MAC
// 以便清晰展示"加密 + 认证"两个独立步骤。）
// iv 用 ClientRandom 的前 16 字节（demo 简化；真实每条记录用独立随机 IV）。
func SecureSend(s Session, plaintext []byte) (ciphertext, mac []byte, err error) {
	iv := ivFromRandom(s.ClientRandom)
	ct, err := aes.EncryptCBC(plaintext, s.AESKey, iv)
	if err != nil {
		return nil, nil, fmt.Errorf("tlssim: 加密失败: %w", err)
	}
	mac = hmac.Compute(s.HMACKey, ct)
	return ct, mac, nil
}

// SecureReceive 校验 MAC 后解密，返回明文。
//
// MAC 不匹配 → 篡改/伪造，返回错误（对应 TLS 的 bad_record_mac alert）。
// 这是"认证"的安全意义：即使密文被攻击者替换，没有 HMACKey 也算不出正确 MAC。
func SecureReceive(s Session, ciphertext, mac []byte) ([]byte, error) {
	if !hmac.Verify(s.HMACKey, ciphertext, mac) {
		return nil, errors.New("tlssim: MAC 验证失败（密文被篡改或伪造）")
	}
	iv := ivFromRandom(s.ClientRandom)
	pt, err := aes.DecryptCBC(ciphertext, s.AESKey, iv)
	if err != nil {
		return nil, fmt.Errorf("tlssim: 解密失败: %w", err)
	}
	return pt, nil
}

// ===== 内部辅助 =====

// deriveKeys 从 preMaster + 双方随机数派生会话密钥。
//
// 用简化双块 KDF（类 PRF 展开，保证输出 >= 48 字节）：
//
//	block1 = SHA256(preMaster || ClientRandom || ServerRandom || 0x01)
//	block2 = SHA256(preMaster || ClientRandom || ServerRandom || 0x02)
//	master = block1 || block2                             // 64 字节
//	AESKey  = master[0:16]
//	HMACKey = master[16:48]
//
// 真实 TLS 用 HMAC-based PRF 做类似但更规范的多轮展开；本教学版用两轮
// SHA-256（标准库）即可满足"两端派生一致 + 输出够长"，且仍是零外部依赖。
// ClientRandom/ServerRandom 喂入，保证不同会话派生不同密钥（防重放）。
func deriveKeys(preMaster, clientRandom, serverRandom []byte, subject string) Session {
	master := make([]byte, 0, 64)
	for counter := byte(1); counter <= 2; counter++ {
		h := sha256.New()
		h.Write(preMaster)
		h.Write(clientRandom)
		h.Write(serverRandom)
		h.Write([]byte{counter})
		master = h.Sum(master) // 32 字节/轮，两轮共 64
	}

	aesKey := make([]byte, aesKeyLen)
	copy(aesKey, master[:aesKeyLen])
	hmacKey := make([]byte, hmacKeyLen)
	copy(hmacKey, master[aesKeyLen:aesKeyLen+hmacKeyLen])

	return Session{
		AESKey:        aesKey,
		HMACKey:       hmacKey,
		ServerRandom:  append([]byte{}, serverRandom...),
		ClientRandom:  append([]byte{}, clientRandom...),
		ServerSubject: subject,
	}
}

// ivFromRandom 从随机数取前 16 字节做 CBC IV（AES 分组大小 = 16）。
// 不足 16 字节则循环填充（仅 demo 用，保证 IV 定长；真实 IV 必须随机且每记录独立）。
func ivFromRandom(rand []byte) []byte {
	iv := make([]byte, 16)
	for i := 0; i < 16; i++ {
		if len(rand) > 0 {
			iv[i] = rand[i%len(rand)]
		}
	}
	return iv
}

// bigIntBytes 把 *big.Int 转成最小大端字节切片（去掉前导零）。
// DH 共享密钥 2 → []byte{0x02}。用于 KDF 输入。
func bigIntBytes(x *big.Int) []byte {
	b := x.Bytes()
	if len(b) == 0 {
		return []byte{0}
	}
	return b
}
