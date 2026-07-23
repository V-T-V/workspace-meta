package finance

import (
	"testing"
)

// TestEqualPayment_Standard 验证等额本息标准案例。
// 贷款20万，36期，年利率4.5%：月供约 5949.38（银行计算器标准值）
func TestEqualPayment_Standard(t *testing.T) {
	principal := YuanToMoney(200000)
	rate := PercentToRate(4.5)
	r, err := EqualPayment(principal, rate, 36)
	if err != nil {
		t.Fatal(err)
	}
	// 月供应在 5945~5955 之间（允许 ±5 元误差）
	yuan := r.MonthlyPayment.ToYuan()
	if yuan < 5945 || yuan > 5955 {
		t.Errorf("月供应约 5949.38，实际 %.2f", yuan)
	}
	// 总还款 > 本金
	if r.TotalPayment <= r.Principal {
		t.Error("总还款应大于本金")
	}
	// 总利息 > 0
	if r.TotalInterest <= 0 {
		t.Error("总利息应 > 0")
	}
}

// TestEqualPayment_ZeroRate 验证零利率。
func TestEqualPayment_ZeroRate(t *testing.T) {
	r, err := EqualPayment(YuanToMoney(120000), 0, 12)
	if err != nil {
		t.Fatal(err)
	}
	if r.MonthlyPayment != YuanToMoney(10000) {
		t.Errorf("零利率月供应为 10000 元，实际 %s", r.MonthlyPayment.FormatYuan())
	}
	if r.TotalInterest != 0 {
		t.Errorf("零利率总利息应为 0，实际 %s", r.TotalInterest.FormatYuan())
	}
}

// TestEqualPayment_Invalid 验证非法参数。
func TestEqualPayment_Invalid(t *testing.T) {
	cases := []struct {
		principal Money
		rate      RatePartsPerMillion
		months    int
	}{
		{0, PercentToRate(4.5), 36},
		{YuanToMoney(10000), PercentToRate(4.5), 0},
		{YuanToMoney(10000), PercentToRate(4.5), 500},
		{YuanToMoney(10000), RatePartsPerMillion(-1), 12},
	}
	for _, c := range cases {
		_, err := EqualPayment(c.principal, c.rate, c.months)
		if err == nil {
			t.Errorf("应返回错误: principal=%v rate=%v months=%d", c.principal, c.rate, c.months)
		}
	}
}

// TestEqualPrincipal_Standard 验证等额本金。
func TestEqualPrincipal_Standard(t *testing.T) {
	principal := YuanToMoney(200000)
	rate := PercentToRate(4.5)
	r, err := EqualPrincipal(principal, rate, 36)
	if err != nil {
		t.Fatal(err)
	}
	// 每月还本金 = 200000/36 ≈ 5555.56
	if r.MonthlyPrincipal.ToYuan() < 5550 || r.MonthlyPrincipal.ToYuan() > 5560 {
		t.Errorf("每月本金应约 5555.56，实际 %.2f", r.MonthlyPrincipal.ToYuan())
	}
	// 首月 > 末月（递减）
	if r.FirstPayment <= r.LastPayment {
		t.Error("等额本金首月还款应大于末月")
	}
	// 总利息 < 等额本息总利息
	eq, _ := EqualPayment(principal, rate, 36)
	if r.TotalInterest >= eq.TotalInterest {
		t.Error("等额本金总利息应小于等额本息")
	}
}

// TestDownPayment 验证首付计算。
func TestDownPayment(t *testing.T) {
	r, err := DownPayment(YuanToMoney(200000), 0.2)
	if err != nil {
		t.Fatal(err)
	}
	if r.DownPayment != YuanToMoney(40000) {
		t.Errorf("首付应为 40000，实际 %s", r.DownPayment.FormatYuan())
	}
	if r.LoanPrincipal != YuanToMoney(160000) {
		t.Errorf("贷款本金应为 160000，实际 %s", r.LoanPrincipal.FormatYuan())
	}
}

// TestDownPayment_Invalid 验证非法首付比例。
func TestDownPayment_Invalid(t *testing.T) {
	if _, err := DownPayment(YuanToMoney(100000), 1.5); err == nil {
		t.Error("首付比例 150% 应报错")
	}
	if _, err := DownPayment(YuanToMoney(100000), -0.1); err == nil {
		t.Error("负首付比例应报错")
	}
}

// TestMoney_FormatYuan 验证金额格式化。
func TestMoney_FormatYuan(t *testing.T) {
	cases := map[Money]string{
		0:             "0",
		100:           "1",
		123456:        "1,234.56",
		1000000:       "10,000",
		-123456:       "-1,234.56",
	}
	for m, want := range cases {
		if got := m.FormatYuan(); got != want {
			t.Errorf("FormatYuan(%d) = %q, want %q", m, got, want)
		}
	}
}

// TestRateConversion 验证利率转换。
func TestRateConversion(t *testing.T) {
	r := PercentToRate(4.5)
	if r != 45000 {
		t.Errorf("4.5%% 应为 45000 ppm，实际 %d", r)
	}
	if r.ToPercent() != 4.5 {
		t.Errorf("45000 ppm 应为 4.5%%，实际 %f", r.ToPercent())
	}
}
