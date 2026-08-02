// Package metrics 的聚合查询层。
//
// 本文件在"已收集的 MetricPoint 切片"上做聚合查询，不触碰 Registry 内部状态：
// 按 label 过滤、按 name 分组求和、取 TopN。这样查询逻辑可复用于任意来源的
// 点切片（来自 Registry.AllPoints / 远端拉取 / 离线导出文件），保持 Registry
// 只负责"采集"这一职责。

package metrics

import (
	"sort"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// MetricSummary 是按 name 聚合后的摘要（跨所有标签组合）。
type MetricSummary struct {
	Name  string
	Sum   float64
	Count int
}

// FilterByLabel 过滤出含指定标签键值对的 metric 点。
//
// key 为空或 point.Labels 为 nil 时该点不匹配。返回新切片，不修改入参。
// 典型用途：从 Registry.AllPoints() 结果里挑出某个 service / endpoint 的点。
func FilterByLabel(points []types.MetricPoint, key, value string) []types.MetricPoint {
	if key == "" {
		return nil
	}
	out := make([]types.MetricPoint, 0, len(points))
	for _, p := range points {
		if p.Labels == nil {
			continue
		}
		if v, ok := p.Labels[key]; ok && v == value {
			out = append(out, p)
		}
	}
	return out
}

// SumByName 按名称求和（跨所有标签组合）。
//
// 把所有 Name == name 的点的 Value 累加。无匹配返回 0。
// 适用 counter 类指标做全局汇总（如某接口的请求总数，不分实例）。
func SumByName(points []types.MetricPoint, name string) float64 {
	var sum float64
	for _, p := range points {
		if p.Name == name {
			sum += p.Value
		}
	}
	return sum
}

// GroupSum 按名称分组求和，返回 map[name]sum。
//
// 一次遍历完成全部分组（O(n)）。空入参返回空非 nil map，便于调用方安全遍历。
func GroupSum(points []types.MetricPoint) map[string]float64 {
	out := make(map[string]float64, len(points))
	for _, p := range points {
		out[p.Name] += p.Value
	}
	return out
}

// TopN 返回值最大的 N 个 metric（按 name 聚合后）。
//
// 先用 GroupSum 按 name 聚合 Sum + Count，再按 Sum 降序排序；
// Sum 相同时按 Name 升序（保证输出确定性、可复现测试）。
// n <= 0 返回 nil；n > 去重后的 name 数时返回全部。
func TopN(points []types.MetricPoint, n int) []MetricSummary {
	if n <= 0 {
		return nil
	}
	// 一次遍历同时累加 Sum 和 Count。
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, p := range points {
		sums[p.Name] += p.Value
		counts[p.Name]++
	}
	summaries := make([]MetricSummary, 0, len(sums))
	for name, sum := range sums {
		summaries = append(summaries, MetricSummary{Name: name, Sum: sum, Count: counts[name]})
	}
	// 排序：Sum 降序为主，Sum 相同按 Name 升序（稳定、可复现）。
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Sum != summaries[j].Sum {
			return summaries[i].Sum > summaries[j].Sum
		}
		return summaries[i].Name < summaries[j].Name
	})
	if n > len(summaries) {
		n = len(summaries)
	}
	return summaries[:n]
}

// Rate 计算同一名称相邻两个 MetricPoint 的变化率（每秒变化量）。
// points 应按时间排序。返回最后一个点与前一个点的斜率。
func Rate(points []types.MetricPoint, name string) float64 {
	var filtered []types.MetricPoint
	for _, p := range points {
		if p.Name == name {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) < 2 {
		return 0
	}
	last := filtered[len(filtered)-1]
	prev := filtered[len(filtered)-2]
	dt := last.Timestamp.Sub(prev.Timestamp).Seconds()
	if dt <= 0 {
		return 0
	}
	return (last.Value - prev.Value) / dt
}
