// Package metrics 的聚合查询层。
//
// 本文件在"已收集的 MetricPoint 切片"上做聚合查询，不触碰 Registry 内部状态：
// 按 label 过滤、按 name 分组求和、取 TopN。这样查询逻辑可复用于任意来源的
// 点切片（来自 Registry.AllPoints / 远端拉取 / 离线导出文件），保持 Registry
// 只负责"采集"这一职责。

package metrics

import (
	"math"
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

// ExtractLabels 收集所有 metric 点在某标签键下的不同值（去重）。
//
// 遍历 points，把每个点 Labels[key] 的值收集起来，去重后按升序返回。
// 典型用途：拿一批点先列出"当前有哪些 service / endpoint"，再做下钻分析。
//
// 行为约定（与 FilterByLabel 对齐，便于组合使用）：
//   - key 为空 → 返回 nil（避免"空键命中所有点"的歧义）。
//   - 点的 Labels 为 nil 或不含该 key → 跳过该点（不报错）。
//   - 去重 + 字母序升序：输出确定性，可直接用于断言/表头渲染。
//   - 不修改入参切片与各点的 Labels map。
func ExtractLabels(points []types.MetricPoint, key string) []string {
	if key == "" {
		return nil
	}
	seen := make(map[string]struct{}, len(points))
	for _, p := range points {
		if p.Labels == nil {
			continue
		}
		if v, ok := p.Labels[key]; ok {
			seen[v] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
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

// Percentile 从 histogram 数据估算指定百分位的值。
// 用线性插值法在桶之间估算。
// p 是 0-100 的百分位（如 50=中位数, 90=P90, 99=P99）。
//
// 实现思路（与 Prometheus quantile 同款算法）：
//   - Buckets[i].Count 是"上界 <= Buckets[i].UpperBound"的累计观测数，
//     最后一个桶的 UpperBound 为 +Inf、Count == hist.Count。
//   - 目标排名 target = p/100 * total，在累计计数序列里找到第一个
//     Count >= target 的桶 bi（即第 p 百分位落在该桶内）。
//   - 用 bi 的下界（即上一个桶的上界 loBound）与上界（hiBound）线性插值：
//     fraction = (target - prevCount) / (bi.Count - prevCount)
//     result = loBound + fraction * (hiBound - loBound)
//
// 边界处理：p <= 0 返回最低桶下界；p >= 100 或 target 落到 +Inf 桶时
// 返回最后一个有限桶的上界（避免返回 +Inf）。空 histogram 返回 0。
func Percentile(hist types.HistogramData, p float64) float64 {
	// 异常输入归一化。
	if len(hist.Buckets) == 0 || hist.Count == 0 {
		return 0
	}
	if p <= 0 {
		return 0
	}
	if p > 100 {
		p = 100
	}

	// 目标观测排名（1-based 语义：target=1 表示第一个观测）。
	target := p / 100 * float64(hist.Count)

	// 找到第一个累计计数 >= target 的桶。
	bi := -1
	for i, b := range hist.Buckets {
		if float64(b.Count) >= target {
			bi = i
			break
		}
	}
	if bi < 0 {
		// 全部桶都没覆盖到（理论上 +Inf 桶必覆盖），返回最大有限上界。
		return finiteUpper(hist.Buckets)
	}

	hiBound := hist.Buckets[bi].UpperBound
	// 落到 +Inf 桶：拿不到有限上界，返回最大有限桶上界。
	if math.IsInf(hiBound, 1) {
		return finiteUpper(hist.Buckets)
	}

	// bi 桶的下界 = 上一个桶的上界（第一个桶下界视为 0）。
	var loBound float64
	var prevCount float64
	if bi > 0 {
		loBound = hist.Buckets[bi-1].UpperBound
		prevCount = float64(hist.Buckets[bi-1].Count)
	}

	// 该桶内的观测数（累计差）。
	bucketCount := float64(hist.Buckets[bi].Count) - prevCount
	if bucketCount <= 0 {
		// 桶内无观测（理论不该发生），直接返回桶上界。
		return hiBound
	}

	// 在 [loBound, hiBound] 内线性插值。
	fraction := (target - prevCount) / bucketCount
	return loBound + fraction*(hiBound-loBound)
}

// finiteUpper 返回 buckets 里最后一个有限上界（即 +Inf 桶之前的那个桶的上界）。
// buckets 至少含一个 +Inf 桶；调用方保证 len(buckets) > 0。
func finiteUpper(buckets []types.HistogramBucket) float64 {
	for i := len(buckets) - 1; i >= 0; i-- {
		if !math.IsInf(buckets[i].UpperBound, 1) {
			return buckets[i].UpperBound
		}
	}
	return 0
}

// Dedup 按 (name+labels) 去重，保留最后出现的点。
func Dedup(points []types.MetricPoint) []types.MetricPoint {
	seen := make(map[string]int) // key → index in result
	var result []types.MetricPoint
	for _, p := range points {
		key := p.Name + labelsKeyStr(p.Labels)
		if idx, ok := seen[key]; ok {
			result[idx] = p // 覆盖为最新
		} else {
			seen[key] = len(result)
			result = append(result, p)
		}
	}
	return result
}

func labelsKeyStr(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for _, k := range keys {
		out += k + "=" + labels[k] + ","
	}
	return out
}
