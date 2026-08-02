package metrics

import "testing"

// TestValidateMetricNameValid 校验一批符合 Prometheus 命名规范的合法名都通过。
func TestValidateMetricNameValid(t *testing.T) {
	valid := []string{
		"http_requests_total",
		"go_memstats_alloc_bytes",
		"my:special:metric", // 冒号用于某些分命名空间
		"_internal_counter", // 下划线开头合法
		":leading_colon",    // 冒号开头合法
		"a",                 // 单字符
		"A1_B2:C3",
	}
	for _, name := range valid {
		if err := ValidateMetricName(name); err != nil {
			t.Errorf("合法名 %q 不应报错，实际 %v", name, err)
		}
	}
}

// TestValidateMetricNameEmpty 校验空名报错。
func TestValidateMetricNameEmpty(t *testing.T) {
	if err := ValidateMetricName(""); err == nil {
		t.Error("空名应报错")
	}
}

// TestValidateMetricNameTooLong 校验超长名报错（>128）。
func TestValidateMetricNameTooLong(t *testing.T) {
	// 129 个字符：超限 1。
	long := makeName(129)
	if err := ValidateMetricName(long); err == nil {
		t.Error("129 字符名应报错（超 128 上限）")
	}
}

// TestValidateMetricNameMaxLengthOK 校验恰好在 128 字符上限的名通过。
func TestValidateMetricNameMaxLengthOK(t *testing.T) {
	exact := makeName(maxMetricNameLen)
	if err := ValidateMetricName(exact); err != nil {
		t.Errorf("恰好 %d 字符的名应通过，实际 %v", maxMetricNameLen, err)
	}
}

// TestValidateMetricNameStartsWithDigit 校验数字开头的名报错。
func TestValidateMetricNameStartsWithDigit(t *testing.T) {
	bad := []string{"1xxx", "9", "0_counter", "123"}
	for _, name := range bad {
		if err := ValidateMetricName(name); err == nil {
			t.Errorf("数字开头名 %q 应报错", name)
		}
	}
}

// TestValidateMetricNameIllegalChars 校验含非法字符的名报错。
func TestValidateMetricNameIllegalChars(t *testing.T) {
	bad := []string{
		"a-b",       // 连字符
		"a.b",       // 点
		"a b",       // 空格
		"中文",        // 非 ASCII
		"a@b",       // 特殊符号
		"a/b",       // 斜杠
		"http://x",  // 含冒号但含斜杠
		"name$",     // 美元符
		"tab\there", // 制表符
		"new\nline", // 换行
		"带中文的名字",    // 中文
	}
	for _, name := range bad {
		if err := ValidateMetricName(name); err == nil {
			t.Errorf("含非法字符的名 %q 应报错", name)
		}
	}
}

// TestValidateMetricNameDoesNotMutate 校验校验不修改入参。
func TestValidateMetricNameDoesNotMutate(t *testing.T) {
	name := "http_requests_total"
	_ = ValidateMetricName(name)
	if name != "http_requests_total" {
		t.Errorf("ValidateMetricName 不应修改入参，实际 %q", name)
	}
}

// makeName 生成长度为 n 的合法指标名（首字符为字母，其余为字母/数字）。
func makeName(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
