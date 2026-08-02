// Package mtls 实现 gpu-mesh 的双向 TLS 证书管理（内置 CA 模式）。
//
// 生产加固（Phase 6）核心安全组件：
//   - Relay 内置自签 CA（首次启动自动生成根证书，持久化到 ca-state.json）
//   - Agent 用一次性 enroll token + CSR 换取短期客户端证书
//   - 证书撤销列表（CRL）支持：撤销后 Agent 秒级下线
//
// 这是公网部署的必备安全层：防止未授权 Agent 接入、防止中间人攻击。
package mtls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"
)

// CA 证书颁发机构（Relay 内置）。
type CA struct {
	mu       sync.Mutex
	cert     *x509.Certificate
	certDER  []byte
	key      *rsa.PrivateKey
	caPEM    []byte

	// 撤销列表
	revoked map[string]bool   // agentID → 是否撤销
	tokens  map[string]int64  // enroll token → 过期时间戳

	statePath string // ca-state.json 持久化路径
}

// LoadOrInit 加载已有 CA，不存在则初始化新 CA。
func LoadOrInit(statePath string) (*CA, error) {
	ca := &CA{
		revoked:   make(map[string]bool),
		tokens:    make(map[string]int64),
		statePath: statePath,
	}
	if err := ca.load(); err != nil {
		// 加载失败，初始化新 CA
		if err := ca.init(); err != nil {
			return nil, err
		}
		if err := ca.save(); err != nil {
			return nil, fmt.Errorf("CA 状态持久化失败: %w", err)
		}
	}
	return ca, nil
}

// init 生成自签根证书。
func (c *CA) init() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "gpu-mesh-relay-ca", Organization: []string{"gpu-mesh"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour), // 10 年
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	c.cert = cert
	c.certDER = der
	c.key = key
	c.caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return nil
}

// CAPEM 返回 CA 根证书 PEM。
func (c *CA) CAPEM() []byte { return c.caPEM }

// caState 持久化结构。
type caState struct {
	CAPEM    string            `json:"ca_pem"`
	KeyPEM   string            `json:"key_pem"`
	Revoked  map[string]bool   `json:"revoked"`
}

func (c *CA) load() error {
	data, err := os.ReadFile(c.statePath)
	if err != nil {
		return err
	}
	var s caState
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	caBlock, _ := pem.Decode([]byte(s.CAPEM))
	if caBlock == nil {
		return errors.New("CA PEM 解析失败")
	}
	cert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}
	keyBlock, _ := pem.Decode([]byte(s.KeyPEM))
	if keyBlock == nil {
		return errors.New("key PEM 解析失败")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}
	c.cert = cert
	c.certDER = caBlock.Bytes
	c.key = key
	c.caPEM = []byte(s.CAPEM)
	c.revoked = s.Revoked
	if c.revoked == nil {
		c.revoked = make(map[string]bool)
	}
	return nil
}

func (c *CA) save() error {
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(c.key)})
	s := caState{
		CAPEM:   string(c.caPEM),
		KeyPEM:  string(keyPEM),
		Revoked: c.revoked,
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(c.statePath, data, 0o600)
}

// GenerateEnrollmentToken 生成一次性注册令牌（1 小时有效）。
func (c *CA) GenerateEnrollmentToken(agentID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	tok := randHex(32)
	c.tokens[tok] = time.Now().Add(time.Hour).Unix()
	return tok
}

// ValidateEnrollmentToken 校验令牌（一次性，校验后作废）。
func (c *CA) ValidateEnrollmentToken(token, agentID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp, ok := c.tokens[token]
	if !ok {
		return false
	}
	delete(c.tokens, token) // 一次性
	if time.Now().Unix() > exp {
		return false
	}
	return true
}

// SignAgentCSR 用 CA 签发 Agent 的 CSR（短期客户端证书）。
func (c *CA) SignAgentCSR(csrPEM []byte, agentID string, ttl time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, errors.New("CSR PEM 解析失败")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("CSR 解析失败: %w", err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: agentID, Organization: []string{"gpu-mesh-agent"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(ttl),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, csr.PublicKey, c.key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// RevokeAgent 撤销 Agent（加入 CRL）。
func (c *CA) RevokeAgent(agentID string) {
	c.mu.Lock()
	c.revoked[agentID] = true
	c.mu.Unlock()
	_ = c.save()
}

// IsRevoked 查询是否撤销。
func (c *CA) IsRevoked(agentID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.revoked[agentID]
}

// TLSConfig 构造 Relay 的 TLS 配置（要求并校验客户端证书 + CRL 撤销检查）。
//
// 必须设 VerifyPeerCertificate 钩子：Go 的 tls 在 ClientAuth=RequireAndVerifyClientCert
// 时只校验证书链签名有效期，不查 CRL。撤销的 Agent 若证书未过期仍能连上。
// 此钩子在握手阶段解析客户端证书 CN，查 CRL，已撤销则拒绝握手。
func (c *CA) TLSConfig() *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(c.cert)
	return &tls.Config{
		ClientCAs:  pool,
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS12,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			// verifiedChains[0][0] 是叶子证书（Agent 客户端证书）
			if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
				return errors.New("无客户端证书链")
			}
			return c.VerifyAgentCert(verifiedChains[0][0])
		},
	}
}

// VerifyAgentCert 校验客户端证书（mTLS 握手时调用），结合 CRL。
//
// 返回 nil 表示通过，非 nil 拒绝握手。
func (c *CA) VerifyAgentCert(cert *x509.Certificate) error {
	if c.IsRevoked(cert.Subject.CommonName) {
		return fmt.Errorf("agent %s 证书已撤销", cert.Subject.CommonName)
	}
	return nil
}

// randHex 生成 n 字节的十六进制随机字符串。
func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}
