package metrics

import (
	"sort"
	"time"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// Downsample 把高频 MetricPoint 按时间窗口降采样。
//
// 每个 window 内的聚合规则：
//   - counter：取窗口内最后一个点（按时间戳升序）的值——counter 单调递增，
//     窗口末值即代表该窗口结束时的累计量，符合 Prometheus rate() 的取数逻辑。
//   - gauge：取窗口内所有点的平均值——gauge 反映瞬时状态，平均值给出该窗口的代表值。
//   - histogram：不参与降采样（MetricPoint 不携带桶数据），原样保留窗口内首个点。
//
// 分组：降采样按 (name, labelsKey) 分别进行——不同序列互不混淆。
// 输出点的 Timestamp 取窗口起点（对齐到 window 整数倍），便于后续按时间聚合。
// window <= 0 时视为不降采样，原样返回（拷贝一份，避免调用方误改原切片）。
//
// 输入无需预先排序：函数内部按时间戳稳定排序后再分组聚合。
func Downsample(points []types.MetricPoint, window time.Duration) []types.MetricPoint {
	if window <= 0 {
		out := make([]types.MetricPoint, len(points))
		copy(out, points)
		return out
	}
	if len(points) == 0 {
		return nil
	}

	// 1. 稳定排序：按时间戳升序（保持同时间戳点的相对顺序，避免序列聚合漂移）。
	sorted := make([]types.MetricPoint, len(points))
	copy(sorted, points)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	// 2. 按 (name, labelsKey) 分组，每组内再按窗口起点二次分组。
	type seriesKey struct {
		name   string
		labels string
	}
	// windowAggs[seriesKey] = 按窗口起点索引的聚合桶（map 保序交给最后排序处理）。
	type aggBucket struct {
		windowStart time.Time
		points      []types.MetricPoint
	}
	series := map[seriesKey][]aggBucket{}
	windowIdx := map[seriesKey]map[int64]int{} // series → windowStart.UnixNano → 在 series 里的下标

	for _, p := range sorted {
		key := seriesKey{name: p.Name, labels: labelsKey(p.Labels)}
		ws := p.Timestamp.Truncate(window).UnixNano()
		if series[key] == nil {
			series[key] = []aggBucket{}
			windowIdx[key] = map[int64]int{}
		}
		idx, ok := windowIdx[key][ws]
		if !ok {
			idx = len(series[key])
			series[key] = append(series[key], aggBucket{windowStart: time.Unix(0, ws)})
			windowIdx[key][ws] = idx
		}
		series[key][idx].points = append(series[key][idx].points, p)
	}

	// 3. 聚合每个桶 → 单个输出点。
	out := make([]types.MetricPoint, 0, len(points))
	for key, buckets := range series {
		for _, b := range buckets {
			out = append(out, aggregateBucket(key.name, b.windowStart, b.points))
		}
	}

	// 4. 输出按时间戳升序、再按 name 稳定排序，便于消费端处理。
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Timestamp.Equal(out[j].Timestamp) {
			return out[i].Name < out[j].Name
		}
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

// aggregateBucket 把同一序列同一窗口内的多个点聚合成一个点。
// counter 取末值、gauge 取均值、histogram 取首点（保留桶信息占位）。
func aggregateBucket(name string, windowStart time.Time, pts []types.MetricPoint) types.MetricPoint {
	if len(pts) == 0 {
		return types.MetricPoint{}
	}
	// 用窗口内第一个点作为模板（保留 Kind/Labels）。
	base := pts[0]
	base.Timestamp = windowStart
	switch base.Kind {
	case types.MetricCounter:
		// counter 单调递增：窗口内最末点（时间戳最大）即窗口末值。
		last := pts[len(pts)-1]
		base.Value = last.Value
	case types.MetricGauge:
		// gauge 取均值。
		var sum float64
		for _, p := range pts {
			sum += p.Value
		}
		base.Value = sum / float64(len(pts))
	case types.MetricHistogram:
		// MetricPoint 不含桶分布，histogram 无法真正降采样；保留窗口首点占位。
		base.Value = pts[0].Value
	}
	// labels 已是序列 key 的来源（parseLabelsKey 还原 map），但 base.Labels 即原 map，直接用。
	return base
}
