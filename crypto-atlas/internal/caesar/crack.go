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
