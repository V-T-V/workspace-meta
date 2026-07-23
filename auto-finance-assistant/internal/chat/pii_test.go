package chat

import (
	"strings"
	"testing"
)

func TestMaskPII_Phone(t *testing.T) {
	cases := map[string]string{
		"我的手机是13812345678": "我的手机是138****5678",
		"电话15900001111联系":  "电话159****1111联系",
		"没有号码的普通文本":       "没有号码的普通文本",
	}
	for in, want := range cases {
		if got := MaskPII(in); got != want {
			t.Errorf("MaskPII(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskPII_PhoneSeparators(t *testing.T) {
	cases := map[string]string{
		"手机138-1234-5678":  "手机138****5678",
		"电话138 1234 5678": "电话138****5678",
	}
	for in, want := range cases {
		if got := MaskPII(in); got != want {
			t.Errorf("MaskPII(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskPII_Email(t *testing.T) {
	got := MaskPII("邮箱zhang.san@company.com联系")
	if got == "邮箱zhang.san@company.com联系" {
		t.Errorf("邮箱应被脱敏，实际 %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Errorf("邮箱脱敏应含 ***，实际 %q", got)
	}
}

func TestMaskPII_IDCard15(t *testing.T) {
	got := MaskPII("旧身份证110101900101001")
	if got == "旧身份证110101900101001" {
		t.Errorf("15位身份证应被脱敏，实际 %q", got)
	}
}

func TestMaskPII_IDCard(t *testing.T) {
	got := MaskPII("身份证号110101199003071234")
	if got == "身份证号110101199003071234" {
		t.Errorf("身份证号应被脱敏，实际 %q", got)
	}
	if len(got) < 8 {
		t.Errorf("脱敏后文本异常短: %q", got)
	}
}

func TestMaskPII_BankCard(t *testing.T) {
	got := MaskPII("银行卡6222020200112345")
	if got == "银行卡6222020200112345" {
		t.Errorf("银行卡号应被脱敏，实际 %q", got)
	}
}

func TestMaskPII_Empty(t *testing.T) {
	if got := MaskPII(""); got != "" {
		t.Errorf("空串应保持空，实际 %q", got)
	}
}
