package vigenere

// Kasiski 测试破解维吉尼亚密码。
//
// 原理（Kasiski 1863）：维吉尼亚虽是多表替换，但密钥循环复用——明文里
// 相隔"密钥长度整数倍"的相同片段，加密后会得到相同的密文片段。因此：
//
//  1. 在密文里搜索重复的 3+ 字符子串（"重复块"）。
//  2. 计算它们出现位置的间距（distance）。
//  3. 这些间距的公约数（尤其 GCD）就是密钥长度候选。
//  4. 对每个候选长度，把密文按列分成 N 组（第 i 组 = 所有位置 ≡ i mod N 的
//     密文字母），每组退化为单表凯撒，用频率分析（chi-squared）逐列破解。
//
// 真正的 Kasiski 在步骤 3 用 GCD，但 GCD 对短密文/噪声敏感（一个错误的
// 重复块就能把 GCD 拉偏）。本实现采用更稳健的简化策略：
//
//   - 直接对"候选密钥长度"穷举（2..maxKeyLen，默认 6）。
//   - 对每个长度 L：按列分组，每列用 caesar.FrequencyCrack（复用凯撒的
//     chi-squared 逻辑）找出该列的位移，拼成密钥；再用卡方总和给该长度打分。
//   - 选卡方总和最小（即解密后最像英文）的长度作为最终答案。
//
// 这样既覆盖了 Kasiski 的核心思想（按密钥长度分组 + 频率分析），又对短密文
// 更鲁棒——这正是题目要求的"简化实现"。对长密文，Kasiski 重复块分析仍可
// 作为 L 候选来源加进来（见 guessKeyLensByKasiski）。

import (
	"github.com/QiuShichang/crypto-atlas/internal/caesar"
)

// maxDefaultKeyLen 是穷举密钥长度的默认上限。
// Kasiski 经典例子里密钥常是 3-6 字母（如 LEMON=5），取 6 覆盖典型教学案例，
// 又避免在短密文上因候选过多而把噪声长度选成"最优"。
const maxDefaultKeyLen = 6

// minKeyLen 是密钥长度的下限（维吉尼亚密钥至少 2 个字母才有"多表"意义）。
const minKeyLen = 2

// KasiskiCrack 用 Kasiski 测试破解维吉尼亚密码。
//
// 输入 ciphertext 可以是任意大小写/含非字母的原始密文（内部 normalize 成纯
// 大写字母再分析）。返回破解出的密钥（大写）与对应明文（大写纯字母）。
//
// 流程：
//  1. normalize 密文为纯大写字母序列 ct。
//  2. 若 ct 太短（< minKeyLen*2）无法分组分析，回退：把整段当凯撒单表破解，
//     返回单字母密钥。
//  3. 否则穷举候选密钥长度 L = minKeyLen..maxKeyLen：
//     - 把 ct 按列分成 L 组。
//     - 每组用 caesar.FrequencyCrack 求位移 d，该列密钥字母 = 'A'+d，拼成密钥 kL。
//     - 用 kL 解密整段 ct，对"完整解密文本"算卡方距离 χL（文本总长固定，
//     故不同 L 之间 χ 可比）。
//  4. 取 χL 最小的 L（解密后最像英文的那一个）作为最终答案。
//
// 为什么用"完整文本卡方"而非"每列卡方之和"选长度：每列样本量随 L 变化
// （列长 ≈ len/L），卡方随样本量缩放，导致每列之和跨 L 不可比、且总是
// 偏向"每列样本最少"的大 L（过拟合）。对完整解密文本（长度恒定）算卡方，
// 所有 L 在同一尺度下比较，正确长度的 χ 会远小于错误长度。
//
// 说明：返回的明文是 normalize 过的纯大写文本（不含原始空格/标点）——
// 破解场景下我们只关心"能否还原字母内容"，原样还原格式既不可能（破解者
// 不知道原始非字母字符位置，因为它们在 normalize 时已丢失）也不必要。
func KasiskiCrack(ciphertext string) (key string, plaintext string) {
	ct := normalize(ciphertext)

	// 极短密文：分组后每列只剩 0-1 个字母，频率分析无意义。
	// 回退为单表凯撒破解（密钥长度 1）。
	if len(ct) < minKeyLen*2 {
		shift := caesar.FrequencyCrack(ct)
		k := string(rune('A' + shift%26))
		return k, Decrypt(ct, k)
	}

	bestKey := ""
	bestPlain := ""
	bestChi := -1.0

	for L := minKeyLen; L <= maxDefaultKeyLen && L < len(ct); L++ {
		// 按列分组：第 i 组收集位置 col 满足 col%L == i 的所有字母。
		columns := make([]string, L)
		for i := 0; i < len(ct); i++ {
			columns[i%L] += string(ct[i])
		}
		// 每列独立做频率分析，拼成该长度下的密钥。
		keyBuf := make([]byte, L)
		for i, col := range columns {
			if len(col) == 0 {
				keyBuf[i] = 'A'
				continue
			}
			shift := caesar.FrequencyCrack(col)
			keyBuf[i] = byte('A' + shift%26)
		}
		k := string(keyBuf)
		// 用该密钥解密整段，对完整解密文本算卡方（跨 L 可比）。
		dec := Decrypt(ct, k)
		chi := chiSquared(dec)
		if bestChi < 0 || chi < bestChi {
			bestChi = chi
			bestKey = k
			bestPlain = dec
		}
	}

	return bestKey, bestPlain
}

// englishFreq 是标准英文字母（A-Z）的出现频率（百分比），按字母序排列。
// 与 caesar 包的 englishFreq 取相同的标准频率数据（ETAOIN SHRDLU 降序），
// 这里复制一份以避免 caesar 导出内部表（破解只需这一份常量）。
//
//	E=12.7% T=9.1% A=8.2% O=7.5% I=7.0% N=6.7% S=6.3% H=6.1% R=6.0%
//	D=4.3% L=4.0% C=2.8% U=2.8% M=2.4% W=2.4% F=2.2% G=2.0% Y=2.0%
//	P=1.9% B=1.5% V=1.0% K=0.8% J=0.15% X=0.15% Q=0.10% Z=0.07%
var englishFreq = [26]float64{
	8.2,  // A
	1.5,  // B
	2.8,  // C
	4.3,  // D
	12.7, // E
	2.2,  // F
	2.0,  // G
	6.1,  // H
	7.0,  // I
	0.15, // J
	0.8,  // K
	4.0,  // L
	2.4,  // M
	6.7,  // N
	7.5,  // O
	1.9,  // P
	0.10, // Q
	6.0,  // R
	6.3,  // S
	9.1,  // T
	2.8,  // U
	1.0,  // V
	2.4,  // W
	0.15, // X
	2.0,  // Y
	0.07, // Z
}

// chiSquared 计算给定（解密后）文本的字母频率与标准英文频率的卡方距离。
// 逻辑与 caesar.chiSquared 等价（复制实现以避免 caesar 导出内部表）。
// 卡方统计量 = Σ (observed - expected)² / expected，越小越接近英文分布。
// 文本总长固定时（同一密文的不同候选密钥），该值跨候选可比——这正是
// 我们用它来挑密钥长度的依据。
func chiSquared(text string) float64 {
	total := 0
	var counts [26]int
	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			counts[r-'A']++
			total++
		} else if r >= 'a' && r <= 'z' {
			counts[r-'a']++
			total++
		}
	}
	if total == 0 {
		return 0
	}
	var chi float64
	for i := 0; i < 26; i++ {
		expected := englishFreq[i] / 100.0 * float64(total)
		if expected <= 0 {
			continue
		}
		diff := float64(counts[i]) - expected
		chi += diff * diff / expected
	}
	return chi
}
