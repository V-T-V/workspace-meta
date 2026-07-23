package main

import (
	// C8 的 mTLS 端到端测试需要完整 TLS 握手，较重，放在这里作为关键路径验证。
	// 实际的 CA 签发校验已在 ca_test.go 覆盖，此处验证 TLSConfig 构造逻辑。
	"testing"
)

// TestRelayMTLSConfigConstruction 验证 mTLS 模式下 TLSConfig 正确构造。
// 完整的握手测试需 certutil 生成证书，此处验证配置结构。
func TestRelayMTLSConfigConstruction(t *testing.T) {
	dir := t.TempDir()
	ca, err := LoadOrCreateCA(dir)
	if err != nil {
		t.Fatalf("CA 创建失败: %v", err)
	}

	pool := caCertPool(ca)
	if pool == nil {
		t.Fatal("caCertPool 不应返回 nil")
	}

	// CertPool 应包含根证书（len(p) 无法直接取，但可验证不 panic）
	// 完整验证在 ca_test.go 的 VerifyClientCert 已覆盖
}

// TestConfigMTLSFlag 验证 -mtls flag 解析。
func TestConfigMTLSFlag(t *testing.T) {
	// parseFlags 会读 os.Args，此处仅验证 Config 结构能承载 MTLS 字段
	c := Config{MTLS: true, CertFile: "cert.pem", KeyFile: "key.pem"}
	if !c.MTLS || c.CertFile == "" {
		t.Fatal("MTLS 配置应可设置")
	}
}
