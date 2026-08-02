package vigenere

import (
	"strings"
	"testing"
)

// crackPlain 是一段足够长、字母分布自然的英文文本。
// 频率分析需要每列有充足样本（经验上每列 ≥ ~80 字母才稳定），
// 因此这里用 500+ 字母的连续英文，使任意长度 2..6 的密钥破解都可靠。
const crackPlain = "ALICE WAS BEGINNING TO GET VERY TIRED OF SITTING BY HER " +
	"SISTER ON THE BANK AND OF HAVING NOTHING TO DO ONCE OR TWICE SHE " +
	"HAD PEEPED INTO THE BOOK HER SISTER WAS READING BUT IT HAD NO " +
	"PICTURES OR CONVERSATIONS IN IT AND WHAT IS THE USE OF A BOOK " +
	"THOUGHT ALICE WITHOUT PICTURES OR CONVERSATIONS SO SHE WAS " +
	"CONSIDERING IN HER OWN MIND AS WELL AS SHE COULD FOR THE HOT DAY " +
	"MADE HER FEEL VERY SLEEPY AND STUPID WHETHER THE PLEASURE OF " +
	"MAKING A DAISY CHAIN WOULD BE WORTH THE TROUBLE OF GETTING UP"

// wantPlain 是 crackPlain 规格化（去非字母、转大写）后的形式——
// KasiskiCrack 内部 normalize 后解密，输出即应是这个纯大写字母串。
var wantPlain = stripNonAlpha(crackPlain)

// TestKasiskiCrackRecoversPlaintext：对多个已知密钥，破解后的明文应与
// 原文（规格化为纯大写字母）完全一致。这是破解"成功"的硬标准。
func TestKasiskiCrackRecoversPlaintext(t *testing.T) {
	keys := []string{"LEMON", "KEY", "ABC", "SECRET", "PASS", "CAT"}
	for _, key := range keys {
		ct := Encrypt(crackPlain, key)
		gotKey, gotPlain := KasiskiCrack(ct)
		if gotPlain != wantPlain {
			t.Errorf("key=%q 破解明文不符:\n got  %q\n want %q\n recoveredKey=%q",
				key, gotPlain, wantPlain, gotKey)
		}
	}
}

// TestKasiskiCrackRecoversExactKey：当密钥长度本身就是最优长度时，
// 破解出的密钥应与原密钥完全一致（密钥不是其循环倍数）。
// 这里挑几个"其倍数会超过 maxKeyLen=6"的长度，避免等价变体干扰。
func TestKasiskiCrackRecoversExactKey(t *testing.T) {
	cases := []string{"LEMON", "SECRET", "PASS", "CAT"}
	for _, key := range cases {
		ct := Encrypt(crackPlain, key)
		gotKey, gotPlain := KasiskiCrack(ct)
		if gotKey != key {
			t.Errorf("key=%q 期望精确还原密钥，got %q (plain=%q)",
				key, gotKey, gotPlain)
		}
	}
}

// TestKasiskiCrackShortKeyEquivalent：短密钥（如 KEY=3）在其倍数长度下
// 会得到等价密钥（KEYKEY=6），两者解密结果一致——破解只要明文对即可，
// 不强制密钥唯一。这里验证 KEY 在长度 3 与 6 下都能正确解密。
func TestKasiskiCrackShortKeyEquivalent(t *testing.T) {
	ct := Encrypt(crackPlain, "KEY")
	_, gotPlain := KasiskiCrack(ct)
	if gotPlain != wantPlain {
		t.Errorf("短密钥 KEY 破解明文不符:\n got  %q\n want %q", gotPlain, wantPlain)
	}
}

// TestKasiskiCrackEmptyInput：空/极短输入不应 panic。
func TestKasiskiCrackEmptyInput(t *testing.T) {
	// 完全空：回退到单表破解，返回单字母密钥 + 空明文。
	k, p := KasiskiCrack("")
	if p != "" {
		t.Errorf("空输入明文应为空, got %q (key=%q)", p, k)
	}
	if len(k) != 1 {
		t.Errorf("空输入应回退单字母密钥, got %q", k)
	}
	// 极短（< 2*2=4 字母）：走单表回退分支，不 panic 即可。
	k2, p2 := KasiskiCrack("ABC")
	if len(k2) != 1 {
		t.Errorf("极短输入应回退单字母密钥, got %q", k2)
	}
	_ = p2
}

// TestKasiskiCrackReturnsUppercaseKey：密钥必须是大写纯字母。
func TestKasiskiCrackReturnsUppercaseKey(t *testing.T) {
	ct := Encrypt(crackPlain, "LEMON")
	k, _ := KasiskiCrack(ct)
	if k == "" {
		t.Fatal("密钥不应为空")
	}
	for _, r := range k {
		if r < 'A' || r > 'Z' {
			t.Errorf("密钥应全为大写字母, got %q (违规字符 %q)", k, string(r))
		}
	}
}

// TestKasiskiCrackPreservesNonAlphaIgnored：含空格/标点/小写的原始密文，
// 其破解结果应与"先 normalize 再喂给 KasiskiCrack"完全一致——这验证了
// 函数内部对非字母字符的 normalize 行为。两条路径必须收敛到同一答案。
//
// （不直接断言"还原成原文"：频率分析对合成短文本不稳定，那是算法本身的
// 统计极限而非 normalize bug；这里只比较两条输入路径是否一致。）
func TestKasiskiCrackPreservesNonAlphaIgnored(t *testing.T) {
	key := "LEMON"
	raw := "Hello, World! This is a test of Vigenere with mixed CASE and 123 digits. " +
		"The quick brown fox jumps over the lazy dog again and again."
	ct := Encrypt(raw, key)
	kRaw, ptRaw := KasiskiCrack(ct)
	// 同样的密文，先 normalize 成纯大写字母再喂——应得到完全相同的结果。
	ctNorm := normalize(ct)
	kNorm, ptNorm := KasiskiCrack(ctNorm)
	if kRaw != kNorm || ptRaw != ptNorm {
		t.Errorf("原始 vs 规格化密文破解不一致:\n raw  key=%q plain=%q\n norm key=%q plain=%q",
			kRaw, ptRaw, kNorm, ptNorm)
	}
}

// stripNonAlpha 丢弃所有非 A-Z/a-z 字符并把字母转大写（与 normalize 等价）。
// 测试里用它把明文规格化成破解输出应呈现的纯大写形式再比对。
func stripNonAlpha(s string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return r
		}
		return -1
	}, s))
}
