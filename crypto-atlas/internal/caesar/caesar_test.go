package caesar

import (
	"context"
	"testing"
)

func TestEncrypt(t *testing.T) {
	cases := []struct {
		plain, want string
		key         int
	}{
		{"ABC", "DEF", 3},
		{"XYZ", "ABC", 3},     // 环绕
		{"abc", "def", 3},     // 小写
		{"Hello", "Khoor", 3}, // 混合大小写
		{"ABC", "ABC", 0},     // key=0 不变
		{"ABC", "ABC", 26},    // key=26 等于 0
		{"DEF", "ABC", -3},    // 负 key
		{"Hi!", "Kl!", 3},     // 非字母不变
	}
	for _, c := range cases {
		if got := Encrypt(c.plain, c.key); got != c.want {
			t.Errorf("Encrypt(%q,%d) = %q, want %q", c.plain, c.key, got, c.want)
		}
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// 加密后解密应还原
	for _, key := range []int{0, 1, 3, 13, 25, 26, 100, -5} {
		plain := "The Quick Brown Fox!"
		ct := Encrypt(plain, key)
		pt := Decrypt(ct, key)
		if pt != plain {
			t.Errorf("key=%d 往返失败: %q → %q → %q", key, plain, ct, pt)
		}
	}
}

func TestNonAlphaUnchanged(t *testing.T) {
	// 数字、标点、空格不变
	s := "123 !@# 中"
	if got := Encrypt(s, 5); got != s {
		t.Errorf("非英文字符应不变: %q → %q", s, got)
	}
}

func TestDemoRuns(t *testing.T) {
	r, err := Demo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Plaintext == r.Ciphertext {
		t.Error("密文不应等于明文")
	}
	if r.Decrypted != r.Plaintext {
		t.Error("解密应还原明文")
	}
}

func TestNegativeKeyNormalization(t *testing.T) {
	// -3 和 23 等价
	if Encrypt("A", -3) != Encrypt("A", 23) {
		t.Error("-3 应等价于 23")
	}
}

func TestCrack(t *testing.T) {
	plaintext := "Hello World"
	ciphertext := Encrypt(plaintext, 7)
	results := Crack(ciphertext)
	if len(results) != 25 {
		t.Errorf("应返回 25 个结果，实际 %d", len(results))
	}
	// 正确的 key=7 应在结果中，解密出原文
	found := false
	for _, r := range results {
		if r.Key == 7 && r.Plaintext == plaintext {
			found = true
		}
	}
	if !found {
		t.Error("key=7 应解密出原文")
	}
}

// TestFrequencyCrack 用足够长的英文密文验证频率分析能找回正确的 key。
// 文本取一段自然英文，足够长以让字母频率稳定接近标准分布。
func TestFrequencyCrack(t *testing.T) {
	plaintext := "the quick brown fox jumps over the lazy dog and then " +
		"it runs away into the forest while the dog keeps chasing it " +
		"through the trees and across the river until the sun goes down"
	// 用多个 key 加密，验证每个都能被破解回来
	for _, key := range []int{1, 3, 7, 13, 25} {
		ciphertext := Encrypt(plaintext, key)
		got := FrequencyCrack(ciphertext)
		if got != key {
			t.Errorf("key=%d: FrequencyCrack 返回 %d，期望 %d（解密: %q）",
				key, got, key, Decrypt(ciphertext, got))
		}
		// 双重确认：用破解出的 key 解密应得回原文
		if Decrypt(ciphertext, got) != plaintext {
			t.Errorf("key=%d: 用破解 key=%d 解密未还原原文", key, got)
		}
	}
}

// TestFrequencyCrackNoLetters 无字母的密文不应 panic，返回 0。
func TestFrequencyCrackNoLetters(t *testing.T) {
	if got := FrequencyCrack("123 !@# 中"); got != 0 {
		t.Errorf("无字母应返回 0，实际 %d", got)
	}
}
