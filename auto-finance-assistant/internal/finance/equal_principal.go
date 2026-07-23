package finance

import "fmt"

// EqualPrincipalResult 等额本金试算结果。
type EqualPrincipalResult struct {
	Principal       Money
	AnnualRate      RatePartsPerMillion
	Months          int
	MonthlyPrincipal Money   // 每月还本金（固定）
	FirstPayment    Money    // 首月还款（最高）
	LastPayment     Money    // 末月还款（最低）
	TotalPayment    Money
	TotalInterest   Money
}

// EqualPrincipal 等额本金计算。
// 每月还本金 = 本金 / 期数（固定）
// 每月利息 = 剩余本金 × 月利率
// 每月还款 = 每月还本金 + 每月利息（逐月递减）
func EqualPrincipal(principal Money, annualRate RatePartsPerMillion, months int) (*EqualPrincipalResult, error) {
	if principal <= 0 {
		return nil, fmt.Errorf("本金必须 > 0")
	}
	if months <= 0 || months > 360 {
		return nil, fmt.Errorf("期数必须在 1~360 之间")
	}
	if annualRate < 0 {
		return nil, fmt.Errorf("利率不能为负")
	}

	monthlyPrincipal := principal / Money(months)
	// 处理除不尽
	remainder := principal - monthlyPrincipal*Money(months)

	monthlyRatePPM := float64(annualRate) / 12.0 / 1000000.0

	var totalInterest Money
	remaining := principal
	var firstPay, lastPay Money

	for period := 1; period <= months; period++ {
		// 本期还本金（最后一期补差）
		thisPrincipal := monthlyPrincipal
		if period == months {
			thisPrincipal = monthlyPrincipal + remainder
		}
		// 本期利息 = 剩余本金 × 月利率
		interest := Money(float64(remaining)*monthlyRatePPM + 0.5)
		payment := thisPrincipal + interest
		totalInterest += interest
		remaining -= thisPrincipal

		if period == 1 {
			firstPay = payment
		}
		lastPay = payment
	}

	return &EqualPrincipalResult{
		Principal:        principal,
		AnnualRate:       annualRate,
		Months:           months,
		MonthlyPrincipal: monthlyPrincipal,
		FirstPayment:     firstPay,
		LastPayment:      lastPay,
		TotalPayment:     principal + totalInterest,
		TotalInterest:    totalInterest,
	}, nil
}
