package finance

import (
	"fmt"
	"math"
)

// EqualPaymentResult 等额本息试算结果。
type EqualPaymentResult struct {
	Principal        Money // 贷款本金
	AnnualRate       RatePartsPerMillion
	Months           int
	MonthlyPayment   Money // 每月还款（固定）
	TotalPayment     Money // 总还款额
	TotalInterest    Money // 总利息
	Schedule         []ScheduleEntry // 还款计划（可选，条数多时不填）
}

// ScheduleEntry 单期还款明细。
type ScheduleEntry struct {
	Period         int
	Payment        Money // 本期还款总额
	Principal      Money // 本期还本金
	Interest       Money // 本期还利息
	Remaining      Money // 剩余本金
}

// EqualPayment 等额本息计算。
// 公式：月还款 = 本金 × 月利率 × (1+月利率)^期数 / ((1+月利率)^期数 - 1)
// 定点实现：用 Money（分）与 RatePPM（ppm）避免 float 累计误差，
// 但因指数运算需 float，中间用 float64 计算后四舍五入回 Money。
func EqualPayment(principal Money, annualRate RatePartsPerMillion, months int) (*EqualPaymentResult, error) {
	if principal <= 0 {
		return nil, fmt.Errorf("本金必须 > 0")
	}
	if months <= 0 || months > 360 {
		return nil, fmt.Errorf("期数必须在 1~360 之间")
	}
	if annualRate < 0 {
		return nil, fmt.Errorf("利率不能为负")
	}

	// 零利率：直接本金/期数
	if annualRate == 0 {
		monthly := principal / Money(months)
		// 处理除不尽：总还款按实际累加（最后一期可能有几分舍入差）
		totalPaid := monthly * Money(months)
		return &EqualPaymentResult{
			Principal:      principal,
			AnnualRate:     annualRate,
			Months:         months,
			MonthlyPayment: monthly,
			TotalPayment:   totalPaid, // 实际还款总额（可能与本金差几分）
			TotalInterest:  0,
		}, nil
	}

	// 月利率（ppm）= 年利率 / 12
	monthlyRatePPM := float64(annualRate) / 12.0 / 1000000.0
	p := float64(principal)

	// (1+r)^n —— 用 math.Pow 保证精度
	pow := math.Pow(1+monthlyRatePPM, float64(months))
	// 月还款 = P × r × pow / (pow - 1)
	monthlyFloat := p * monthlyRatePPM * pow / (pow - 1)
	monthly := Money(monthlyFloat + 0.5)

	totalPayment := monthly * Money(months)
	totalInterest := totalPayment - principal

	return &EqualPaymentResult{
		Principal:      principal,
		AnnualRate:     annualRate,
		Months:         months,
		MonthlyPayment: monthly,
		TotalPayment:   totalPayment,
		TotalInterest:  totalInterest,
	}, nil
}

// Disclaimer 试算免责声明（所有计算结果必须附带）。
const Disclaimer = "\n\n以上结果仅为试算，不构成正式金融报价。实际金额以金融机构审批结果及正式合同为准。"
