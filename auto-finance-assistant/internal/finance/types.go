// Package finance 实现金融试算（定点，避免浮点误差）。
// 对应原计划第十五节。所有金额用 int64「分」表示。
package finance

// Money 以「分」为单位的人民币金额。禁止用 float64 累计金额。
type Money int64

// RatePartsPerMillion 利率用 ppm（百万分之一）表示。
// 例：4.5% 年利率 = 45000 ppm。
// 这样可以精确表示 4.5%、4.375% 等小数利率，避免 float 误差。
type RatePartsPerMillion int64

// PercentToRate 把百分数转为 ppm。
// 例：PercentToRate(4.5) → 45000
func PercentToRate(percent float64) RatePartsPerMillion {
	return RatePartsPerMillion(percent * 10000)
}

// ToPercent 转回百分数（仅用于展示）。
func (r RatePartsPerMillion) ToPercent() float64 {
	return float64(r) / 10000
}

// YuanToMoney 把元转为分。
func YuanToMoney(yuan float64) Money {
	return Money(yuan*100 + 0.5) // 四舍五入
}

// ToYuan 转回元（仅用于展示）。
func (m Money) ToYuan() float64 {
	return float64(m) / 100
}

// FormatYuan 格式化为人民币字符串，如 "1234.56"。
func (m Money) FormatYuan() string {
	negate := false
	v := int64(m)
	if v < 0 {
		negate = true
		v = -v
	}
	yuan := v / 100
	cents := v % 100
	s := ""
	if cents > 0 {
		s = formatWithCommas(yuan) + "." + twoDigit(cents)
	} else {
		s = formatWithCommas(yuan)
	}
	if negate {
		return "-" + s
	}
	return s
}

// formatWithCommas 千分位逗号。
func formatWithCommas(n int64) string {
	if n == 0 {
		return "0"
	}
	digits := []rune{}
	for n > 0 {
		digits = append([]rune{rune('0' + n%10)}, digits...)
		n /= 10
	}
	var out []rune
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, d)
	}
	return string(out)
}

func twoDigit(n int64) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
