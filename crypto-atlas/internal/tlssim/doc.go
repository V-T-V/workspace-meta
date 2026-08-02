// Package tlssim 把 TLS 1.2 握手拆成 5 个清晰阶段做教学演示（非真实 TLS）。
//
// 设计目标：复用本仓库已有的 rsa / dh / aes / hmac 四个教学包，把"一次握手"
// 变成"四个原语的串讲"——读者看完 rsa（身份认证）、dh（密钥协商）、
// aes（对称加密）、hmac（消息认证）后，本包回答："它们如何拼成一个协议？"
//
// 阶段对应 TLS 1.2 的 DHE_RSA 密码套件：
//
//  1. ClientHello  —— ClientRandom（防重放）
//  2. ServerHello  —— ServerRandom + 证书（RSA 公钥）+ DH 公开值（DHE）
//  3. 密钥协商     —— 客户端验证书；双方 DH 算 preMaster
//  4. 密钥派生     —— 双块 KDF 展开 preMaster+CR+SR 成 64 字节；切出 AES/HMAC 密钥
//  5. record layer —— AES 加密 + HMAC 认证应用数据
//
// 实现见 tlssim.go，demo 见 demo.go，背景/局限/演进见 NOTES.md。
//
// 重要：本包是教学版，绝不可用于真实通信。真实 TLS 1.2 还包含 PRF 多轮扩展、
// Finished 握手摘要验证、X.509 证书链校验、AEAD（GCM/ChaCha20-Poly1305）、
// 版本与套件协商、重协商/会话恢复/0-RTT 等。TLS 1.3 进一步把握手改成 1-RTT、
// 移除了静态 RSA 密钥交换、强制 AEAD。详见 NOTES.md。
package tlssim
