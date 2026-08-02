package mtls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestCA 在临时目录初始化一个 CA。
func newTestCA(t *testing.T) (*CA, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ca-state.json")
	ca, err := LoadOrInit(path)
	if err != nil {
		t.Fatalf("LoadOrInit 失败: %v", err)
	}
	return ca, path
}

func TestCA_InitGeneratesCert(t *testing.T) {
	ca, _ := newTestCA(t)
	if ca.caPEM == nil || len(ca.caPEM) == 0 {
		t.Fatal("CA PEM 为空")
	}
	if ca.cert == nil || !ca.cert.IsCA {
		t.Fatal("生成的证书不是 CA")
	}
	// 验证有效期（应 10 年）
	if time.Until(ca.cert.NotAfter) < 9*365*24*time.Hour {
		t.Error("CA 证书有效期不足 10 年")
	}
}

func TestCA_StatePersistence(t *testing.T) {
	// 初始化 → 重新加载 → 应一致
	ca1, path := newTestCA(t)
	ca1.RevokeAgent("agent-test-01")
	ca2, err := LoadOrInit(path)
	if err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}
	if !ca2.IsRevoked("agent-test-01") {
		t.Error("重载后撤销状态丢失")
	}
}

func TestCA_EnrollToken(t *testing.T) {
	ca, _ := newTestCA(t)
	tok := ca.GenerateEnrollmentToken("agent-01")
	if tok == "" {
		t.Fatal("token 为空")
	}
	// 校验通过
	if !ca.ValidateEnrollmentToken(tok, "agent-01") {
		t.Error("有效 token 校验失败")
	}
	// 一次性：第二次应失败
	if ca.ValidateEnrollmentToken(tok, "agent-01") {
		t.Error("一次性 token 被重复使用")
	}
}

func TestCA_EnrollTokenInvalid(t *testing.T) {
	ca, _ := newTestCA(t)
	if ca.ValidateEnrollmentToken("bogus", "a") {
		t.Error("假 token 不应通过")
	}
}

func TestCA_SignAgentCSR(t *testing.T) {
	ca, _ := newTestCA(t)
	// 生成一对密钥 + CSR
	csrPEM, _, err := genTestCSR("agent-sign-test")
	if err != nil {
		t.Fatalf("生成 CSR 失败: %v", err)
	}
	certPEM, err := ca.SignAgentCSR(csrPEM, "agent-sign-test", 24*time.Hour)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	// 解析签发的证书
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("签发的证书 PEM 解析失败")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("证书解析失败: %v", err)
	}
	if cert.Subject.CommonName != "agent-sign-test" {
		t.Errorf("CN 应为 agent-sign-test，得到 %s", cert.Subject.CommonName)
	}
	// 应是客户端证书（ExtKeyUsageClientAuth）
	hasClientAuth := false
	for _, ku := range cert.ExtKeyUsage {
		if ku == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
	}
	if !hasClientAuth {
		t.Error("签发的证书缺 ClientAuth 用途")
	}
}

func TestCA_RevokeAndVerify(t *testing.T) {
	ca, _ := newTestCA(t)
	csrPEM, _, _ := genTestCSR("agent-revoke-test")
	certPEM, _ := ca.SignAgentCSR(csrPEM, "agent-revoke-test", time.Hour)
	block, _ := pem.Decode(certPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)

	// 未撤销：校验通过
	if err := ca.VerifyAgentCert(cert); err != nil {
		t.Errorf("未撤销的证书不应报错: %v", err)
	}
	// 撤销后：校验失败
	ca.RevokeAgent("agent-revoke-test")
	if err := ca.VerifyAgentCert(cert); err == nil {
		t.Error("撤销后校验应失败")
	}
}

func TestCA_TLSConfig(t *testing.T) {
	ca, _ := newTestCA(t)
	cfg := ca.TLSConfig()
	if cfg.ClientAuth == 0 {
		t.Error("TLSConfig 未要求客户端证书")
	}
	if cfg.MinVersion == 0 {
		t.Error("TLSConfig 未设最低版本")
	}
}

func TestMain(m *testing.M) {
	// 确保 rand 可用（某些环境需种子）
	os.Exit(m.Run())
}

// genTestCSR 生成测试用 CSR + 私钥（返回 CSR PEM, key PEM, error）。
func genTestCSR(cn string) (csrPEM []byte, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: cn},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		return nil, nil, err
	}
	csrPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return csrPEM, keyPEM, nil
}
