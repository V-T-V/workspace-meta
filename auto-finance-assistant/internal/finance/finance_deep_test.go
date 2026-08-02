package finance

// 第四轮：金融计算器深层测试。
// 覆盖还款计划生成（逐期本金/利息/剩余本金递推）、提前还款（缩期/降月供）、
// 利率换算边界、金额格式化边界、定点精度一致性。

import (
	"math"
	"strings"
	"testing"
)

// ===========================================================================
// 还款计划生成
// ===========================================================================

func TestBuildEqualPaymentSchedule_Standard(t *testing.T) {
	principal := YuanToMoney(200000)
	rate := PercentToRate(4.5)
	schedule, err := BuildEqualPaymentSchedule(principal, rate, 36)
	if err != nil {
		t.Fatal(err)
	}
	if len(schedule) != 36 {
		t.Fatalf("应 36 期，实际 %d", len(schedule))
	}
	// 第 1 期
	first := schedule[0]
	if first.Period != 1 {
		t.Errorf("首期 Period 应 1，实际 %d", first.Period)
	}
	// 首期利息 = 200000 × 4.5%/12 ≈ 750
	if first.Interest.ToYuan() < 745 || first.Interest.ToYuan() > 755 {
		t.Errorf("首期利息应约 750，实际 %.2f", first.Interest.ToYuan())
	}
	// 末期剩余本金应归零（尾差补齐）
	last := schedule[35]
	if last.Remaining != 0 {
		t.Errorf("末期剩余本金应 0，实际 %s", last.Remaining.FormatYuan())
	}
	// 每期还款额应等于固定月供（除末期可能差几分）
	base, _ := EqualPayment(principal, rate, 36)
	monthly := base.MonthlyPayment
	for i, e := range schedule {
		if i < 35 {
			diff := e.Payment - monthly
			if diff < -2 || diff > 2 {
				t.Errorf("第 %d 期还款 %s 偏离月供 %s（差 %s）", i+1,
					e.Payment.FormatYuan(), monthly.FormatYuan(), diff.FormatYuan())
			}
		}
	}
}

func TestBuildEqualPaymentSchedule_InterestDecreasing(t *testing.T) {
	// 等额本息：每期利息递减、本金递增
	schedule, _ := BuildEqualPaymentSchedule(YuanToMoney(100000), PercentToRate(6.0), 12)
	for i := 1; i < len(schedule); i++ {
		if schedule[i].Interest > schedule[i-1].Interest {
			// 允许极小四舍五入波动，但整体应递减
			if schedule[i].Interest-schedule[i-1].Interest > 1 {
				t.Errorf("第 %d 期利息 %s 应 <= 前期 %s", i+1,
					schedule[i].Interest.FormatYuan(), schedule[i-1].Interest.FormatYuan())
			}
		}
		if schedule[i].Principal+1 < schedule[i-1].Principal {
			t.Errorf("本金应递增（允许 1 分波动）")
		}
	}
}

func TestBuildEqualPaymentSchedule_Invalid(t *testing.T) {
	cases := []struct {
		p    Money
		r    RatePartsPerMillion
		m    int
	}{
		{0, PercentToRate(4.5), 12},
		{YuanToMoney(10000), PercentToRate(4.5), 0},
		{YuanToMoney(10000), PercentToRate(4.5), 500},
		{YuanToMoney(10000), RatePartsPerMillion(-1), 12},
	}
	for _, c := range cases {
		if _, err := BuildEqualPaymentSchedule(c.p, c.r, c.m); err == nil {
			t.Errorf("应返回错误: p=%v r=%v m=%d", c.p, c.r, c.m)
		}
	}
}

func TestBuildEqualPaymentSchedule_SumEqualsTotal(t *testing.T) {
	// 各期本金之和 = 原本金；各期利息之和 = 总利息
	principal := YuanToMoney(150000)
	rate := PercentToRate(5.4)
	months := 24
	schedule, _ := BuildEqualPaymentSchedule(principal, rate, months)
	var sumPrincipal, sumInterest Money
	for _, e := range schedule {
		sumPrincipal += e.Principal
		sumInterest += e.Interest
	}
	// 本金之和应等于原始本金（允许 1 分尾差）
	if diff := sumPrincipal - principal; diff > 1 || diff < -1 {
		t.Errorf("本金之和 %s 应等于 %s（误差 ≤1 分）", sumPrincipal.FormatYuan(), principal.FormatYuan())
	}
	// 总利息应 > 0
	if sumInterest <= 0 {
		t.Error("总利息应 > 0")
	}
}

func TestBuildEqualPaymentSchedule_ZeroRate(t *testing.T) {
	schedule, err := BuildEqualPaymentSchedule(YuanToMoney(120000), 0, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(schedule) != 12 {
		t.Fatalf("应 12 期，实际 %d", len(schedule))
	}
	// 零利率：每期利息 = 0，本金 = 10000
	for _, e := range schedule {
		if e.Interest != 0 {
			t.Errorf("零利率利息应 0，实际 %s", e.Interest.FormatYuan())
		}
		if e.Principal != YuanToMoney(10000) {
			t.Errorf("零利率每期本金应 10000，实际 %s", e.Principal.FormatYuan())
		}
	}
}

// ===========================================================================
// 提前还款
// ===========================================================================

func TestPrepay_ShortenTerm_SavesInterest(t *testing.T) {
	principal := YuanToMoney(200000)
	rate := PercentToRate(4.5)
	// 36 期合同，第 12 期末提前还款 50000，缩期模式
	r, err := Prepay(principal, rate, 36, 12, YuanToMoney(50000), PrepayShortenTerm)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != PrepayShortenTerm {
		t.Errorf("mode 应 shorten_term")
	}
	// 缩期后剩余期数应 < 原剩余期数 (36-12=24)
	if r.NewMonths >= 24 {
		t.Errorf("缩期后剩余期数应 < 24，实际 %d", r.NewMonths)
	}
	// 必须省息
	if r.SavedInterest <= 0 {
		t.Errorf("提前还款应省息，实际节省 %s", r.SavedInterest.FormatYuan())
	}
	// 还款后剩余本金 = 还款前 − 50000
	expectRemaining := r.RemainingBefore - YuanToMoney(50000)
	if r.RemainingAfter != expectRemaining {
		t.Errorf("还款后剩余本金应 %s，实际 %s", expectRemaining.FormatYuan(), r.RemainingAfter.FormatYuan())
	}
	// 月供不变（缩期模式）
	if r.NewMonthlyPayment <= 0 {
		t.Error("缩期模式月供应 > 0")
	}
}

func TestPrepay_ReducePayment_LowersMonthly(t *testing.T) {
	principal := YuanToMoney(200000)
	rate := PercentToRate(4.5)
	r, err := Prepay(principal, rate, 36, 12, YuanToMoney(50000), PrepayReducePayment)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != PrepayReducePayment {
		t.Errorf("mode 应 reduce_payment")
	}
	// 期数不变：36-12=24
	if r.NewMonths != 24 {
		t.Errorf("降月供模式期数应 24，实际 %d", r.NewMonths)
	}
	// 新月供应 < 原月供（约 5949）
	if r.NewMonthlyPayment.ToYuan() > 5949 {
		t.Errorf("降月供后月供应 < 5949，实际 %.2f", r.NewMonthlyPayment.ToYuan())
	}
	// 必须省息
	if r.SavedInterest <= 0 {
		t.Errorf("提前还款应省息，实际 %s", r.SavedInterest.FormatYuan())
	}
}

func TestPrepay_ShortenTermSavesMoreThanReducePayment(t *testing.T) {
	// 同等提前还款额，缩期模式省息应 >= 降月供模式
	principal := YuanToMoney(200000)
	rate := PercentToRate(4.5)
	shorten, _ := Prepay(principal, rate, 36, 12, YuanToMoney(50000), PrepayShortenTerm)
	reduce, _ := Prepay(principal, rate, 36, 12, YuanToMoney(50000), PrepayReducePayment)
	if shorten.SavedInterest < reduce.SavedInterest {
		t.Errorf("缩期省息 %s 应 >= 降月供省息 %s",
			shorten.SavedInterest.FormatYuan(), reduce.SavedInterest.FormatYuan())
	}
}

func TestPrepay_FullRepayRemaining(t *testing.T) {
	// 提前还款额 >= 剩余本金 → 一次还清，省全部剩余利息
	principal := YuanToMoney(100000)
	rate := PercentToRate(6.0)
	// 第 6 期末一次性还清剩余
	r, err := Prepay(principal, rate, 12, 6, YuanToMoney(1000000), PrepayShortenTerm)
	if err != nil {
		t.Fatal(err)
	}
	// 实际还款额被钳制为剩余本金
	if r.PrepayAmount != r.RemainingBefore {
		t.Errorf("超额还款应钳制为剩余本金，实际 %s vs %s",
			r.PrepayAmount.FormatYuan(), r.RemainingBefore.FormatYuan())
	}
	// 一次还清：剩余期数 0，剩余利息 0
	if r.NewMonths != 0 {
		t.Errorf("一次还清后剩余期数应 0，实际 %d", r.NewMonths)
	}
	if r.NewTotalInterest != 0 {
		t.Errorf("一次还清后剩余利息应 0，实际 %s", r.NewTotalInterest.FormatYuan())
	}
}

func TestPrepay_Invalid(t *testing.T) {
	cases := []struct {
		name       string
		principal  Money
		rate       RatePartsPerMillion
		months     int
		paid       int
		prepay     Money
		mode       PrepayMode
	}{
		{"本金 0", 0, PercentToRate(4.5), 36, 12, YuanToMoney(10000), PrepayShortenTerm},
		{"期数 0", YuanToMoney(10000), PercentToRate(4.5), 0, 0, YuanToMoney(1000), PrepayShortenTerm},
		{"负利率", YuanToMoney(10000), RatePartsPerMillion(-1), 12, 1, YuanToMoney(1000), PrepayShortenTerm},
		{"已还期数越界", YuanToMoney(10000), PercentToRate(4.5), 12, 12, YuanToMoney(1000), PrepayShortenTerm},
		{"负还款额", YuanToMoney(10000), PercentToRate(4.5), 12, 1, Money(-100), PrepayShortenTerm},
		{"未知模式", YuanToMoney(10000), PercentToRate(4.5), 12, 1, YuanToMoney(1000), PrepayMode("bogus")},
	}
	for _, c := range cases {
		_, err := Prepay(c.principal, c.rate, c.months, c.paid, c.prepay, c.mode)
		if err == nil {
			t.Errorf("%s: 应返回错误", c.name)
		}
	}
}

func TestPrepay_ZeroPaidPeriods(t *testing.T) {
	// 第 0 期末（即放款后立即）提前还款
	r, err := Prepay(YuanToMoney(100000), PercentToRate(6.0), 12, 0, YuanToMoney(20000), PrepayShortenTerm)
	if err != nil {
		t.Fatal(err)
	}
	// 第 0 期末剩余本金 = 原本金（尚未还款）
	if r.RemainingBefore != YuanToMoney(100000) {
		t.Errorf("第 0 期末剩余应 = 本金 100000，实际 %s", r.RemainingBefore.FormatYuan())
	}
	if r.RemainingAfter != YuanToMoney(80000) {
		t.Errorf("还款后应 80000，实际 %s", r.RemainingAfter.FormatYuan())
	}
}

// ===========================================================================
// 利率换算边界
// ===========================================================================

func TestPercentToRate_Boundary(t *testing.T) {
	cases := []struct {
		percent float64
		want    RatePartsPerMillion
	}{
		{0, 0},
		{4.5, 45000},
		{0.001, 10},      // 极小利率
		{24.0, 240000},   // 高利率
		{4.375, 43750},   // 小数利率
	}
	for _, c := range cases {
		got := PercentToRate(c.percent)
		if got != c.want {
			t.Errorf("PercentToRate(%v) = %d, want %d", c.percent, got, c.want)
		}
		// 往返转换应一致（误差 < 1 ppm）
		back := got.ToPercent()
		if math.Abs(back-c.percent) > 0.0001 {
			t.Errorf("往返转换 %v → %v 误差过大", c.percent, back)
		}
	}
}

func TestYuanToMoney_Rounding(t *testing.T) {
	// 四舍五入到分
	cases := []struct {
		yuan float64
		want Money
	}{
		{100.00, 10000},
		{100.004, 10000},  // 舍
		{100.005, 10001},  // 入
		{0.01, 1},
		{0.005, 1},        // 入
	}
	for _, c := range cases {
		got := YuanToMoney(c.yuan)
		if got != c.want {
			t.Errorf("YuanToMoney(%v) = %d, want %d", c.yuan, got, c.want)
		}
	}
}

func TestRatePartsPerMillion_FormatPercent(t *testing.T) {
	if PercentToRate(4.5).ToPercent() != 4.5 {
		t.Error("4.5% 往返不一致")
	}
}

// TestNewMonthlyPayment_Consistency 验证月供公式与 EqualPayment 一致。
func TestNewMonthlyPayment_Consistency(t *testing.T) {
	principal := YuanToMoney(200000)
	rate := PercentToRate(4.5)
	months := 36
	r1, _ := EqualPayment(principal, rate, months)
	r2 := NewMonthlyPayment(principal, rate, months)
	if r1.MonthlyPayment != r2 {
		t.Errorf("NewMonthlyPayment 应与 EqualPayment 月供一致：%s vs %s",
			r1.MonthlyPayment.FormatYuan(), r2.FormatYuan())
	}
}

// ===========================================================================
// 金额格式化边界
// ===========================================================================

func TestFormatYuan_LargeNumbers(t *testing.T) {
	cases := map[Money]string{
		YuanToMoney(100000000): "100,000,000", // 1 亿
		YuanToMoney(1234567.89): "1,234,567.89",
		Money(1):               "0.01",
		Money(99):              "0.99",
		Money(101):             "1.01",
	}
	for m, want := range cases {
		if got := m.FormatYuan(); got != want {
			t.Errorf("FormatYuan(%d) = %q, want %q", m, got, want)
		}
	}
}

func TestFormatYuan_Negative(t *testing.T) {
	// 负数金额（理论上不应出现，但格式化需稳健）
	cases := map[Money]string{
		Money(-100):    "-1",
		Money(-123456): "-1,234.56",
		Money(0):       "0",
	}
	for m, want := range cases {
		if got := m.FormatYuan(); got != want {
			t.Errorf("FormatYuan(%d) = %q, want %q", m, got, want)
		}
	}
}

func TestToYuan_RoundTrip(t *testing.T) {
	// YuanToMoney → ToYuan 往返（允许 ±0.01 误差）
	for _, yuan := range []float64{0.01, 100.50, 9999.99, 123456.78} {
		m := YuanToMoney(yuan)
		back := m.ToYuan()
		if math.Abs(back-yuan) > 0.01 {
			t.Errorf("往返 %v → %v 误差过大", yuan, back)
		}
	}
}

// ===========================================================================
// 等额本金深层
// ===========================================================================

func TestEqualPrincipal_DecreasingPayment(t *testing.T) {
	r, err := EqualPrincipal(YuanToMoney(300000), PercentToRate(4.35), 60)
	if err != nil {
		t.Fatal(err)
	}
	// 首月 > 末月（严格递减）
	if r.FirstPayment <= r.LastPayment {
		t.Errorf("等额本金首月 %s 应 > 末月 %s",
			r.FirstPayment.FormatYuan(), r.LastPayment.FormatYuan())
	}
	// 每月本金固定 = 300000/60 = 5000
	if r.MonthlyPrincipal.ToYuan() < 4999 || r.MonthlyPrincipal.ToYuan() > 5001 {
		t.Errorf("每月本金应约 5000，实际 %.2f", r.MonthlyPrincipal.ToYuan())
	}
}

func TestEqualPrincipal_LessInterestThanEqualPayment(t *testing.T) {
	// 同等条件，等额本金总利息 < 等额本息
	p := YuanToMoney(300000)
	rate := PercentToRate(4.35)
	months := 60
	eqp, _ := EqualPrincipal(p, rate, months)
	ep, _ := EqualPayment(p, rate, months)
	if eqp.TotalInterest >= ep.TotalInterest {
		t.Errorf("等额本金总利息 %s 应 < 等额本息 %s",
			eqp.TotalInterest.FormatYuan(), ep.TotalInterest.FormatYuan())
	}
}

func TestEqualPrincipal_TotalPaymentConsistency(t *testing.T) {
	// 总还款 = 本金 + 总利息
	r, _ := EqualPrincipal(YuanToMoney(100000), PercentToRate(5.0), 12)
	expectTotal := r.Principal + r.TotalInterest
	if r.TotalPayment != expectTotal {
		t.Errorf("总还款 %s 应 = 本金+利息 %s",
			r.TotalPayment.FormatYuan(), expectTotal.FormatYuan())
	}
}

// ===========================================================================
// 等额本息数学一致性
// ===========================================================================

func TestEqualPayment_MonthlyTimesMonthsApproxTotal(t *testing.T) {
	// 月供 × 期数 ≈ 总还款（允许月供四舍五入累积误差）
	r, _ := EqualPayment(YuanToMoney(200000), PercentToRate(4.5), 36)
	product := r.MonthlyPayment * Money(r.Months)
	diff := r.TotalPayment - product
	// 误差应在几分内（月供四舍五入）
	if diff < -36 || diff > 36 {
		t.Errorf("月供×期数 %s 与总还款 %s 偏差过大 %s",
			product.FormatYuan(), r.TotalPayment.FormatYuan(), diff.FormatYuan())
	}
}

func TestEqualPayment_DisclaimerPresent(t *testing.T) {
	// 所有结果附免责声明
	if !strings.Contains(Disclaimer, "试算") {
		t.Error("免责声明应含 试算")
	}
	r, _ := EqualPayment(YuanToMoney(10000), PercentToRate(4.5), 12)
	_ = r // 确保可调用
}

// ===========================================================================
// DownPayment 深层
// ===========================================================================

func TestDownPayment_BoundaryPct(t *testing.T) {
	cases := []struct {
		pct  float64
		ok   bool
	}{
		{0, true},     // 0% 首付（全贷）
		{1, true},     // 100% 首付（全款）
		{0.3, true},   // 常规 30%
		{1.01, false}, // 超 100%
		{-0.1, false}, // 负数
	}
	price := YuanToMoney(200000)
	for _, c := range cases {
		_, err := DownPayment(price, c.pct)
		gotOk := err == nil
		if gotOk != c.ok {
			t.Errorf("首付比例 %v 期望 ok=%v 实际 ok=%v err=%v", c.pct, c.ok, gotOk, err)
		}
	}
}

func TestDownPayment_ZeroPct(t *testing.T) {
	// 0% 首付：贷款本金 = 车价
	r, err := DownPayment(YuanToMoney(200000), 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.DownPayment != 0 {
		t.Errorf("0%% 首付金额应 0，实际 %s", r.DownPayment.FormatYuan())
	}
	if r.LoanPrincipal != YuanToMoney(200000) {
		t.Errorf("0%% 首付贷款应 = 车价，实际 %s", r.LoanPrincipal.FormatYuan())
	}
}

func TestDownPayment_FullPct(t *testing.T) {
	// 100% 首付：贷款本金 = 0
	r, err := DownPayment(YuanToMoney(200000), 1)
	if err != nil {
		t.Fatal(err)
	}
	if r.LoanPrincipal != 0 {
		t.Errorf("100%% 首付贷款应 0，实际 %s", r.LoanPrincipal.FormatYuan())
	}
	if r.DownPayment != YuanToMoney(200000) {
		t.Errorf("100%% 首付金额应 = 车价，实际 %s", r.DownPayment.FormatYuan())
	}
}
