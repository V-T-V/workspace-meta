// Package metrics 的指标名校验。
//
// 本文件实现 Prometheus 命名规范的指标名合法性校验。Registry / Counter /
// Gauge / Histogram 创建时若能顺手 ValidateMetricName 一下，可把非法名挡在
// 采集之前（避免导出文本时才发现名字带非法字符、被远端 Prometheus 拒收）。

package metrics

import (
	"errors"
	"fmt"
)

// 指标名长度上限（与 Prometheus client_golang 的约定一致，留出足够余量）。
const maxMetricNameLen = 128

// ValidateMetricName 校验指标名是否合法（对齐 Prometheus 命名规范）。
//
// 规则：
//   - 非空
//   - 长度不超过 128 字符
//   - 仅允许字母 / 数字 / 下划线 / 冒号（[a-zA-Z0-9_:]）
//   - 首字符不能是数字（Prometheus 要求名字以字母/下划线/冒号开头，
//     避免与裸数字字面量冲突）
//
// 合法示例：http_requests_total、go_memstats_alloc_bytes、
// my:special:metric、_internal_counter。
// 非法示例：""（空）、"1xxx"（数字开头）、"a-b"（含连字符）、
// "a.b"（含点）、含空格 / 中文等。
//
// 合法返回 nil，非法返回描述具体原因的 error。
func ValidateMetricName(name string) error {
	if name == "" {
		return errors.New("指标名不能为空")
	}
	if len(name) > maxMetricNameLen {
		return fmt.Errorf("指标名长度 %d 超过上限 %d", len(name), maxMetricNameLen)
	}
	// 首字符不能是数字（Prometheus 规范：避免与数值字面量歧义）。
	if name[0] >= '0' && name[0] <= '9' {
		return fmt.Errorf("指标名 %q 不能以数字开头", name)
	}
	// 逐字符校验合法字符集。
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !isMetricNameChar(c) {
			return fmt.Errorf("指标名 %q 含非法字符 %q（只允许字母/数字/下划线/冒号）", name, rune(c))
		}
	}
	return nil
}

// isMetricNameChar 报告字节 c 是否为 Prometheus 指标名的合法字符。
// 注意指标名都是 ASCII，按字节判断即可（无需 rune 解码）。
func isMetricNameChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_' || c == ':':
		return true
	}
	return false
}
