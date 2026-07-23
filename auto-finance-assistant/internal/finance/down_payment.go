package finance

import "fmt"

// DownPaymentResult 首付试算结果。
type DownPaymentResult struct {
	VehiclePrice   Money // 车价
	DownPaymentPct float64 // 首付比例（如 0.2 = 20%）
	DownPayment    Money // 首付金额
	LoanPrincipal  Money // 贷款本金
}

// DownPayment 根据车价与首付比例计算首付金额与贷款本金。
func DownPayment(vehiclePrice Money, downPaymentPct float64) (*DownPaymentResult, error) {
	if vehiclePrice <= 0 {
		return nil, fmt.Errorf("车价必须 > 0")
	}
	if downPaymentPct < 0 || downPaymentPct > 1 {
		return nil, fmt.Errorf("首付比例必须在 0~1 之间（如 0.2 表示 20%%）")
	}
	downPay := Money(float64(vehiclePrice)*downPaymentPct + 0.5)
	loan := vehiclePrice - downPay
	return &DownPaymentResult{
		VehiclePrice:   vehiclePrice,
		DownPaymentPct: downPaymentPct,
		DownPayment:    downPay,
		LoanPrincipal:  loan,
	}, nil
}

// EqualPaymentRequest 试算请求（统一入口）。
type EqualPaymentRequest struct {
	PrincipalYuan float64 `json:"principal"`  // 贷款本金（元）
	AnnualRatePct float64 `json:"annualRate"` // 年利率（%，如 4.5）
	Months        int     `json:"months"`     // 期数
	IncludeSchedule bool  `json:"includeSchedule"` // 是否返回还款计划
}
