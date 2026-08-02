package rsa

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/QiuShichang/crypto-atlas/internal/core"
	"github.com/QiuShichang/crypto-atlas/internal/hmac"
)

// SigDemoResult 汇总一次数字签名演示的全部产物，便于断言/打印。
//
// 字段按演示步骤编号：
//  1. 生成密钥对
//  2. 签名
//  3. 正常验签（应通过）
//  4. 篡改消息后验签（应失败）
//  5. 与 HMAC 对称签名对比
type SigDemoResult struct {
	// —— RSA 部分 ——
	Pub  PublicKey
	Priv PrivateKey

	// Message 是被签名的原始消息（数值形式，教学用）。
	Message *big.Int
	// Signature 是对 Message 的 RSA 私钥签名：s = m^D mod N。
	Signature *big.Int

	// VerifiedValid：用原始消息验签的结果，应为 true。
	VerifiedValid bool
	// TamperedMessage：把 Message 改动一位后的消息，用于演示篡改检测。
	TamperedMessage *big.Int
	// VerifiedTampered：对篡改消息验签的结果，应为 false。
	VerifiedTampered bool

	// —— HMAC 对比部分 ——
	// HmacMessage：与 RSA 部分语义对应的文本消息（"transfer:100"）。
	HmacMessage string
	// HmacMAC：HMAC-SHA-256（hex）。
	HmacMAC string
	// HmacVerified：HMAC 正常验证结果，应为 true。
	HmacVerified bool
	// HmacTamperedVerified：篡改 HmacMessage 后验证结果，应为 false。
	HmacTamperedVerified bool

	// Comparison 对比 HMAC（对称）与 RSA（非对称）的关键差异，便于打印/教学。
	Comparison string
}

// SignatureDemo 演示完整的数字签名流程：
//  1. 生成 RSA 密钥对（教学用经典参数 p=61, q=53 → N=3233）
//  2. 对消息签名（s = m^D mod N）
//  3. 验证签名（正确消息 → true）
//  4. 篡改消息后验证（→ false）
//  5. 对比 HMAC 对称签名 vs RSA 非对称签名
//
// ctx 预留给未来可能引入的可取消/超时操作（与同包 Demo 签名对齐），当前未使用。
//
// 教学说明：
//   - RSA 签名是非对称的：私钥签名、公钥验签。任何人拿到公钥都能验签，
//     但只有私钥持有者能产生合法签名。这让"谁能签"与"谁能验"分离。
//   - HMAC 是对称的：签名和验签用同一个密钥。能验的人也能签，因此不能
//     向第三方证明"这条消息是对方发的而非我自己伪造的"（不可否认性缺失）。
//   - 真实 RSA 签名应对消息的哈希（如 SHA-256 摘要）签名，而非直接对数值
//     签名；本教学版直接对数值签名以聚焦数学原理。
func SignatureDemo(ctx context.Context) (*SigDemoResult, error) {
	_ = ctx

	// —— 步骤 1：生成 RSA 密钥对 ——
	// 经典教材参数 p=61, q=53 → N=3233, E=17, D=2753，确定性便于复现。
	pub, priv, err := GenerateKey(61, 53)
	if err != nil {
		return nil, fmt.Errorf("SignatureDemo: 生成密钥失败: %w", err)
	}

	// —— 步骤 2：对消息签名 ——
	// 取 m = 123（任意 0..N-1 内的值都可）。
	m := big.NewInt(123)
	sig, err := Sign(m, priv)
	if err != nil {
		return nil, fmt.Errorf("SignatureDemo: 签名失败: %w", err)
	}

	// —— 步骤 3：正确消息验签（应 true）——
	verifiedValid := Verify(m, sig, pub)

	// —— 步骤 4：篡改消息后验签（应 false）——
	// 把 m 改成 124，签名 s 是针对 123 的，验签必然失败 → 检测到篡改。
	tampered := big.NewInt(124)
	verifiedTampered := Verify(tampered, sig, pub)

	// —— 步骤 5：HMAC 对称签名对比 ——
	// 用同一份"消息文本"演示 HMAC：一个共享密钥既签又验。
	hmacKey := []byte("shared-secret-2026")
	hmacMsg := []byte("transfer:100")
	mac := hmac.Compute(hmacKey, hmacMsg)
	hmacVerified := hmac.Verify(hmacKey, hmacMsg, mac)

	// 篡改消息文本（金额 100 → 999），HMAC 验证应失败。
	hmacTamperedMsg := []byte("transfer:999")
	hmacTamperedVerified := hmac.Verify(hmacKey, hmacTamperedMsg, mac)

	// —— 打印教学输出 ——
	fmt.Println("=== 数字签名综合演示 ===")
	fmt.Printf("[1] RSA 密钥对: N=%s, E=%d, D=%s\n", pub.N.String(), pub.E, priv.D.String())
	fmt.Printf("[2] 对 m=%s 签名: s=%s\n", m.String(), sig.String())
	fmt.Printf("[3] 正常验签 (m=%s):        %v\n", m.String(), verifiedValid)
	fmt.Printf("[4] 篡改消息验签 (m=%s):    %v  ← 篡改被检测\n", tampered.String(), verifiedTampered)
	fmt.Println()
	fmt.Println("[5] HMAC 对称签名对比:")
	fmt.Printf("    HMAC-SHA256(%q) = %s\n", string(hmacMsg), core.HexEncode(mac))
	fmt.Printf("    正常验证:   %v\n", hmacVerified)
	fmt.Printf("    篡改验证:   %v  ← 篡改被检测\n", hmacTamperedVerified)
	fmt.Println()
	fmt.Println("对比：")
	fmt.Printf("    RSA（非对称）：私钥 D=%s 签名，公钥 (E=%d,N=%s) 验签 → 签/验分离\n",
		priv.D.String(), pub.E, pub.N.String())
	fmt.Println("    HMAC（对称）：同一密钥既签又验 → 签/验合一，无不可否认性")
	// 附带演示 SHA-256 摘要（真实 RSA 签名应签摘要，此处仅作展示）。
	digest := sha256.Sum256(hmacMsg)
	fmt.Printf("    附：消息 %q 的 SHA-256 摘要 = %s（真实 RSA 签名应签此摘要）\n",
		string(hmacMsg), core.HexEncode(digest[:]))

	return &SigDemoResult{
		Pub:                  pub,
		Priv:                 priv,
		Message:              m,
		Signature:            sig,
		VerifiedValid:        verifiedValid,
		TamperedMessage:      tampered,
		VerifiedTampered:     verifiedTampered,
		HmacMessage:          string(hmacMsg),
		HmacMAC:              core.HexEncode(mac),
		HmacVerified:         hmacVerified,
		HmacTamperedVerified: hmacTamperedVerified,
		Comparison: "RSA 非对称（签/验分离，可公开验签、有不可否认性） vs " +
			"HMAC 对称（签/验同密钥，不可向第三方证明来源）",
	}, nil
}
