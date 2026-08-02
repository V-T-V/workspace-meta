package caesar

// CrackResult 是暴力破解的一个候选结果。
type CrackResult struct {
	Key       int
	Plaintext string
}

// Crack 暴力破解凯撒密文：尝试所有 25 个非零 key，返回每个 key 的解密结果。
// 破解者可以用频率分析或人工审查选出正确的明文。
func Crack(ciphertext string) []CrackResult {
	results := make([]CrackResult, 0, 25)
	for key := 1; key <= 25; key++ {
		results = append(results, CrackResult{
			Key:       key,
			Plaintext: Decrypt(ciphertext, key),
		})
	}
	return results
}

// englishFreq 是标准英文字母（A-Z）的出现频率，按字母序排列。
// 数据来自经典英文频率统计（与 ETAOIN SHRDLU 降序一致）：
//
//	E=12.7% T=9.1% A=8.2% O=7.5% I=7.0% N=6.7% S=6.3% H=6.1% R=6.0%
//	D=4.3% L=4.0% C=2.8% U=2.8% M=2.4% W=2.4% F=2.2% G=2.0% Y=2.0%
//	P=1.9% B=1.5% V=1.0% K=0.8% J=0.15% X=0.15% Q=0.10% Z=0.07%
//
// 单位是百分比（0-100）。
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

// FrequencyCrack 用英文字母频率分析破解凯撒密文。
// 统计密文各字母频率，与标准英文频率对比，选偏差最小的 key。
// 标准频率：ETAOIN SHRDLU（降序）。
//
// 算法：对每个 key(0-25)，先解密再统计解密文本的字母频率，
// 计算与标准英文频率的卡方（chi-squared）距离，选最小者。
// 返回的是使偏差最小的 key（0 表示"无需位移"，即原文本身）。
//
// 注意：频率分析对密文长度敏感，文本越接近自然英文分布越准确；
// 极短或非自然文本（如全是同一个字母）可能误判。
func FrequencyCrack(ciphertext string) int {
	total := countLetters(ciphertext)
	if total == 0 {
		return 0 // 无字母无法分析
	}

	bestKey := 0
	bestChi := -1.0
	for key := 0; key < 26; key++ {
		candidate := Decrypt(ciphertext, key)
		chi := chiSquared(candidate, total)
		if bestChi < 0 || chi < bestChi {
			bestChi = chi
			bestKey = key
		}
	}
	return bestKey
}

// countLetters 统计文本中 A-Z 字母（不区分大小写）的总数。
func countLetters(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			n++
		case r >= 'a' && r <= 'z':
			n++
		}
	}
	return n
}

// chiSquared 计算给定文本的字母频率与标准英文频率的卡方距离。
// 文本必须已含字母（total>0），调用方负责保证。
// 卡方统计量 = Σ (observed - expected)² / expected，越小越接近英文分布。
func chiSquared(text string, total int) float64 {
	var counts [26]int
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z':
			counts[r-'A']++
		case r >= 'a' && r <= 'z':
			counts[r-'a']++
		}
	}
	var chi float64
	for i := 0; i < 26; i++ {
		// 期望频次 = 该字母标准占比 × 总字母数
		expected := englishFreq[i] / 100.0 * float64(total)
		if expected <= 0 {
			continue
		}
		diff := float64(counts[i]) - expected
		chi += diff * diff / expected
	}
	return chi
}
