// 本文件实现等额本息还款计划生成与提前还款试算。
// 对应原计划第十五节补充。提前还款两种常见模式：
//   - 缩短期数（月供不变，期数减少，省息最多）
//   - 减少月供（期数不变，月供降低）
package finance

import "fmt"

// BuildEqualPaymentSchedule 生成等额本息逐期还款明细。
// 每期：利息 = 剩余本金 × 月利率；本金 = 月供 − 利息；剩余本金递减。
// 最后一期补齐因四舍五入累积的尾差（保证剩余归零）。
func BuildEqualPaymentSchedule(principal Money, annualRate RatePartsPerMillion, months int) ([]ScheduleEntry, error) {
	if principal <= 0 {
		return nil, fmt.Errorf("本金必须 > 0")
	}
	if months <= 0 || months > 360 {
		return nil, fmt.Errorf("期数必须在 1~360 之间")
	}
	if annualRate < 0 {
		return nil, fmt.Errorf("利率不能为负")
	}

	// 复用 EqualPayment 得到固定月供（保证与汇总数一致）
	base, err := EqualPayment(principal, annualRate, months)
	if err != nil {
		return nil, err
	}
	monthly := base.MonthlyPayment
	monthlyRatePPM := float64(annualRate) / 12.0 / 1000000.0

	schedule := make([]ScheduleEntry, 0, months)
	remaining := principal
	for period := 1; period <= months; period++ {
		interest := Money(float64(remaining)*monthlyRatePPM + 0.5)
		// 本期还本金 = 月供 − 利息（最后一期补尾差）
		thisPrincipal := monthly - interest
		if period == months {
			thisPrincipal = remaining // 末期还清全部剩余
			// 末期还款 = 本金 + 利息（可能与月供差几分）
		}
		if thisPrincipal > remaining {
			thisPrincipal = remaining
		}
		payment := thisPrincipal + interest
		remaining -= thisPrincipal
		if remaining < 0 {
			remaining = 0
		}
		schedule = append(schedule, ScheduleEntry{
			Period:    period,
			Payment:   payment,
			Principal: thisPrincipal,
			Interest:  interest,
			Remaining: remaining,
		})
	}
	return schedule, nil
}

// PrepayMode 提前还款后的处理方式。
type PrepayMode string

const (
	PrepayShortenTerm  PrepayMode = "shorten_term"  // 缩短期数（月供不变）
	PrepayReducePayment PrepayMode = "reduce_payment" // 减少月供（期数不变）
)

// PrepayResult 提前还款试算结果。
type PrepayResult struct {
	Mode             PrepayMode
	PaidPeriods      int    // 已还期数
	PrepayAmount     Money  // 提前还款金额（一次性偿还本金）
	RemainingBefore  Money  // 还款前剩余本金
	RemainingAfter   Money  // 还款后剩余本金
	NewMonthlyPayment Money  // 缩短期数：原月供；减少月供：新月供
	NewMonths        int    // 缩短期数：新剩余期数；减少月供：原剩余期数
	SavedInterest    Money  // 节省的利息（相对原计划）
	NewTotalInterest Money  // 提前还款后剩余利息合计
}

// Prepay 计算在第 paidPeriods 期末提前偿还 prepayAmount 本金后的新方案。
//
// 流程：
//  1. 用原合同参数生成完整还款计划；
//  2. 取第 paidPeriods 期末的剩余本金 remaining；
//  3. 提前还款 prepayAmount（不超过 remaining）后的新剩余本金 newRemaining；
//  4. shorten_term：月供不变，重算剩余期数；reduce_payment：期数不变，重算月供；
//  5. 节省利息 = 原计划剩余总利息 − 新方案剩余总利息。
func Prepay(principal Money, annualRate RatePartsPerMillion, months, paidPeriods int, prepayAmount Money, mode PrepayMode) (*PrepayResult, error) {
	if principal <= 0 {
		return nil, fmt.Errorf("本金必须 > 0")
	}
	if months <= 0 || months > 360 {
		return nil, fmt.Errorf("期数必须在 1~360 之间")
	}
	if annualRate < 0 {
		return nil, fmt.Errorf("利率不能为负")
	}
	if paidPeriods < 0 || paidPeriods >= months {
		return nil, fmt.Errorf("已还期数必须在 0 ~ %d 之间", months-1)
	}
	if prepayAmount < 0 {
		return nil, fmt.Errorf("提前还款金额不能为负")
	}

	schedule, err := BuildEqualPaymentSchedule(principal, annualRate, months)
	if err != nil {
		return nil, err
	}

	// 已还期数的剩余本金：paidPeriods=N 表示已还 N 期，剩余 = 第 N 期末（schedule[N-1].Remaining）
	// paidPeriods=0 表示尚未还款，剩余 = 原本金。
	remaining := principal
	originalRemainingInterest := Money(0)
	for i := 0; i < months; i++ {
		// i+1 == paidPeriods 表示第 paidPeriods 期末的剩余
		if paidPeriods > 0 && i == paidPeriods-1 {
			remaining = schedule[i].Remaining
		}
		// 累计从第 paidPeriods+1 期开始的剩余利息（即尚未支付的利息）
		if i >= paidPeriods {
			originalRemainingInterest += schedule[i].Interest
		}
	}

	// 提前还款额不超过剩余本金
	actualPrepay := prepayAmount
	if actualPrepay > remaining {
		actualPrepay = remaining
	}
	newRemaining := remaining - actualPrepay

	result := &PrepayResult{
		Mode:            mode,
		PaidPeriods:     paidPeriods,
		PrepayAmount:    actualPrepay,
		RemainingBefore: remaining,
		RemainingAfter:  newRemaining,
	}

	// 原月供：取第 paidPeriods 期的还款额作为基准（已还末期的月供）
	monthly := schedule[0].Payment
	if paidPeriods > 0 {
		monthly = schedule[paidPeriods-1].Payment
	}

	switch mode {
	case PrepayShortenTerm:
		// 月供不变，重算剩余期数
		result.NewMonthlyPayment = monthly
		if newRemaining <= 0 {
			result.NewMonths = 0
			result.NewTotalInterest = 0
		} else {
			newMonths, newInterest := calcRemainingShortenTerm(newRemaining, annualRate, monthly)
			result.NewMonths = newMonths
			result.NewTotalInterest = newInterest
		}
	case PrepayReducePayment:
		// 期数不变，重算月供
		remainingPeriods := months - paidPeriods
		result.NewMonths = remainingPeriods
		if newRemaining <= 0 {
			result.NewMonthlyPayment = 0
			result.NewTotalInterest = 0
		} else {
			newMonthly, newInterest := calcRemainingReducePayment(newRemaining, annualRate, remainingPeriods)
			result.NewMonthlyPayment = newMonthly
			result.NewTotalInterest = newInterest
		}
	default:
		return nil, fmt.Errorf("未知提前还款模式: %s", mode)
	}

	result.SavedInterest = originalRemainingInterest - result.NewTotalInterest
	if result.SavedInterest < 0 {
		result.SavedInterest = 0
	}
	return result, nil
}

// calcRemainingShortenTerm 在固定月供下，还清 newRemaining 需要多少期 + 总利息。
func calcRemainingShortenTerm(newRemaining Money, annualRate RatePartsPerMillion, monthly Money) (int, Money) {
	monthlyRatePPM := float64(annualRate) / 12.0 / 1000000.0
	remaining := newRemaining
	var totalInterest Money
	months := 0
	// 最多迭代原期数次（防止月供 < 利息导致无限循环）
	for i := 0; i < 360 && remaining > 0; i++ {
		interest := Money(float64(remaining)*monthlyRatePPM + 0.5)
		thisPrincipal := monthly - interest
		if thisPrincipal <= 0 {
			// 月供不足以覆盖利息，无法缩期
			break
		}
		if thisPrincipal > remaining {
			thisPrincipal = remaining
		}
		remaining -= thisPrincipal
		totalInterest += interest
		months++
	}
	return months, totalInterest
}

// calcRemainingReducePayment 在固定期数下，还清 newRemaining 的月供 + 总利息。
func calcRemainingReducePayment(newRemaining Money, annualRate RatePartsPerMillion, months int) (Money, Money) {
	// 零利率：月供 = 本金 / 期数
	if annualRate == 0 || newRemaining <= 0 {
		monthly := newRemaining / Money(months)
		return monthly, 0
	}
	// 用等额本息公式算新月供
	r := NewMonthlyPayment(newRemaining, annualRate, months)
	// 逐期累加利息（末期补尾差）
	monthlyRatePPM := float64(annualRate) / 12.0 / 1000000.0
	remaining := newRemaining
	var totalInterest Money
	for period := 1; period <= months; period++ {
		interest := Money(float64(remaining)*monthlyRatePPM + 0.5)
		thisPrincipal := r - interest
		if period == months {
			thisPrincipal = remaining
		}
		if thisPrincipal > remaining {
			thisPrincipal = remaining
		}
		remaining -= thisPrincipal
		totalInterest += interest
	}
	return r, totalInterest
}

// NewMonthlyPayment 按等额本息公式计算月供（独立函数，供提前还款复用）。
func NewMonthlyPayment(principal Money, annualRate RatePartsPerMillion, months int) Money {
	if annualRate == 0 || principal <= 0 || months <= 0 {
		if months > 0 {
			return principal / Money(months)
		}
		return principal
	}
	monthlyRatePPM := float64(annualRate) / 12.0 / 1000000.0
	p := float64(principal)
	pow := 1.0
	for i := 0; i < months; i++ {
		pow *= (1 + monthlyRatePPM)
	}
	monthlyFloat := p * monthlyRatePPM * pow / (pow - 1)
	return Money(monthlyFloat + 0.5)
}
