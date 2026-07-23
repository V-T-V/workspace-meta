package api

import (
	"net/http"

	"github.com/QiuShichang/auto-finance-assistant/internal/finance"
)

type equalPaymentRequest struct {
	Principal  float64 `json:"principal"`  // 贷款本金（元）
	AnnualRate float64 `json:"annualRate"` // 年利率（%）
	Months     int     `json:"months"`     // 期数
}

type equalPaymentResponse struct {
	Type            string  `json:"type"`
	Principal       string  `json:"principal"`
	AnnualRate      float64 `json:"annualRate"`
	Months          int     `json:"months"`
	MonthlyPayment  string  `json:"monthlyPayment"`
	TotalPayment    string  `json:"totalPayment"`
	TotalInterest   string  `json:"totalInterest"`
	Disclaimer      string  `json:"disclaimer"`
}

func (s *Server) handleEqualPayment(w http.ResponseWriter, r *http.Request) {
	var body equalPaymentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	result, err := finance.EqualPayment(
		finance.YuanToMoney(body.Principal),
		finance.PercentToRate(body.AnnualRate),
		body.Months,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "calc_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, equalPaymentResponse{
		Type: "equal_payment", Principal: result.Principal.FormatYuan(),
		AnnualRate: body.AnnualRate, Months: result.Months,
		MonthlyPayment: result.MonthlyPayment.FormatYuan(),
		TotalPayment:   result.TotalPayment.FormatYuan(),
		TotalInterest:  result.TotalInterest.FormatYuan(),
		Disclaimer:     finance.Disclaimer,
	})
}

type equalPrincipalResponse struct {
	Type             string  `json:"type"`
	Principal        string  `json:"principal"`
	AnnualRate       float64 `json:"annualRate"`
	Months           int     `json:"months"`
	MonthlyPrincipal string  `json:"monthlyPrincipal"`
	FirstPayment     string  `json:"firstPayment"`
	LastPayment      string  `json:"lastPayment"`
	TotalPayment     string  `json:"totalPayment"`
	TotalInterest    string  `json:"totalInterest"`
	Disclaimer       string  `json:"disclaimer"`
}

func (s *Server) handleEqualPrincipal(w http.ResponseWriter, r *http.Request) {
	var body equalPaymentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	result, err := finance.EqualPrincipal(
		finance.YuanToMoney(body.Principal),
		finance.PercentToRate(body.AnnualRate),
		body.Months,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "calc_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, equalPrincipalResponse{
		Type: "equal_principal", Principal: result.Principal.FormatYuan(),
		AnnualRate: body.AnnualRate, Months: result.Months,
		MonthlyPrincipal: result.MonthlyPrincipal.FormatYuan(),
		FirstPayment:     result.FirstPayment.FormatYuan(),
		LastPayment:      result.LastPayment.FormatYuan(),
		TotalPayment:     result.TotalPayment.FormatYuan(),
		TotalInterest:    result.TotalInterest.FormatYuan(),
		Disclaimer:       finance.Disclaimer,
	})
}

type downPaymentRequest struct {
	VehiclePrice  float64 `json:"vehiclePrice"`  // 车价（元）
	DownPaymentPct float64 `json:"downPaymentPct"` // 首付比例（0.2 = 20%）
}

type downPaymentResponse struct {
	Type         string  `json:"type"`
	VehiclePrice string  `json:"vehiclePrice"`
	DownPaymentPct float64 `json:"downPaymentPct"`
	DownPayment  string  `json:"downPayment"`
	LoanPrincipal string `json:"loanPrincipal"`
}

func (s *Server) handleDownPayment(w http.ResponseWriter, r *http.Request) {
	var body downPaymentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	result, err := finance.DownPayment(
		finance.YuanToMoney(body.VehiclePrice),
		body.DownPaymentPct,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "calc_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, downPaymentResponse{
		Type: "down_payment", VehiclePrice: result.VehiclePrice.FormatYuan(),
		DownPaymentPct: body.DownPaymentPct,
		DownPayment:    result.DownPayment.FormatYuan(),
		LoanPrincipal:  result.LoanPrincipal.FormatYuan(),
	})
}
