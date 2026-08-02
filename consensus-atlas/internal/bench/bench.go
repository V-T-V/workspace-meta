// Package bench 实现 consensus-atlas 各算法的横向性能基准对比。
//
// 每个 benchmark 把对应算法的 Demo() 跑 N 次，统计：
//   - 总耗时 / 平均耗时
//   - 每秒可完成的共识/收敛次数（throughput）
//
// 这让学生直观看到不同算法在相同规模下的性能差异（如 PBFT 的 O(n²) 消息
// 比 Raft 的 O(n) 慢多少）。
//
// 用法：go run ./cmd/atlas -bench   或   make bench
package bench

import (
	"context"
	"fmt"
	"time"
)

// Result 是单个算法的基准结果。
type Result struct {
	Name       string        // 算法名（raft/paxos/...）
	Runs       int           // 实际跑的次数
	TotalTime  time.Duration // 总耗时
	AvgTime    time.Duration // 平均耗时（每次）
	Throughput float64       // 每秒完成次数（1/AvgTime 秒）
	Err        error         // 若算法 demo 报错
}

// Summary 是全部算法的基准汇总。
type Summary struct {
	Results []Result
}

// Benchmark 跑单个算法的 DemoFunc N 次，返回统计。
// demoFunc 是算法的入口（如 raft.Demo 的包装）。
func Benchmark(name string, demoFunc func(context.Context) error, runs int) Result {
	r := Result{Name: name, Runs: runs}
	if runs <= 0 {
		runs = 10
	}
	r.Runs = runs

	start := time.Now()
	for i := 0; i < runs; i++ {
		if err := demoFunc(context.Background()); err != nil {
			r.Err = err
			return r
		}
	}
	r.TotalTime = time.Since(start)
	// 用 float64 算平均，避免整数除法对极快 demo（<1µs）截断成 0。
	r.AvgTime = time.Duration(float64(r.TotalTime) / float64(runs))
	if r.AvgTime <= 0 {
		// 理论上不会进这里（float64 除法只要 TotalTime>0 就 >0），留作数值兜底。
		r.AvgTime = 1 * time.Nanosecond
	}
	// Throughput = 1s / AvgTime。注意：当 AvgTime <1µs 时，单次耗时与
	// time.Now() 的测量粒度同量级，Throughput 会被严重放大（如 1ns 下限
	// 算出 1e9 次/秒）。FormatResult 会对这类结果标 "≈/*" 提醒读者。
	r.Throughput = float64(time.Second) / float64(r.AvgTime)
	return r
}

// FormatResult 把单个 Result 格式化成一行。
func FormatResult(r Result) string {
	if r.Err != nil {
		return fmt.Sprintf("%-12s ERROR: %v", r.Name, r.Err)
	}
	// 亚微秒（<1µs）的 demo 极快，单次耗时与 time.Now() 的测量粒度同量级，
	// 吞吐量因此失真（会出现"1000000000 次/秒"这种被 1ns 下限放大的数字）。
	// 对这类结果标注 "≈"，提醒读者 throughput 仅为量级参考而非精确值。
	if r.AvgTime < time.Microsecond {
		return fmt.Sprintf("%-12s %4d 次, 平均 ≈%8s, %8.1f 次/秒*",
			r.Name, r.Runs, formatDuration(r.AvgTime), r.Throughput)
	}
	return fmt.Sprintf("%-12s %4d 次, 平均 %8s, %8.1f 次/秒",
		r.Name, r.Runs, formatDuration(r.AvgTime), r.Throughput)
}

// formatDuration 把 time.Duration 格式化成自适应精度的可读字符串。
// 不做 Round(µs) 截断——否则 <500ns 的 demo 会被四舍五入成 0s（之前的精度 bug）：
//   - <1µs：保留纳秒整数（如 "320ns"），此时测量本身受 time.Now() 粒度影响。
//   - <1ms：保留 3 位小数微秒（如 "12.345µs"）。
//   - 否则：交给 Duration.String()（"1.234ms" / "1.5s"）。
func formatDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.3fµs", float64(d.Nanoseconds())/1000.0)
	default:
		return d.String()
	}
}

// FormatSummary 把全部结果格式化成对比表。
func FormatSummary(s Summary) string {
	out := fmt.Sprintf("算法性能基准（各跑 %d 次取平均）\n", s.Results[0].Runs)
	out += "--------------------------------------------\n"
	for _, r := range s.Results {
		out += FormatResult(r) + "\n"
	}
	// 找最快和最慢
	if len(s.Results) > 1 {
		var fastest, slowest *Result
		for i := range s.Results {
			if s.Results[i].Err != nil {
				continue
			}
			if fastest == nil || s.Results[i].AvgTime < fastest.AvgTime {
				fastest = &s.Results[i]
			}
			if slowest == nil || s.Results[i].AvgTime > slowest.AvgTime {
				slowest = &s.Results[i]
			}
		}
		if fastest != nil && slowest != nil && fastest.Name != slowest.Name && fastest.AvgTime > 0 {
			ratio := float64(slowest.AvgTime) / float64(fastest.AvgTime)
			if ratio > 0 && ratio < 1e9 { // 防 Inf/NaN
				// 最快若 <1µs，比值受测量噪声主导，标注近似。
				approx := ""
				if fastest.AvgTime < time.Microsecond {
					approx = "（最快 <1µs，比值仅供参考）"
				}
				out += fmt.Sprintf("\n最快: %s (%s), 最慢: %s (%s), 差 %.1fx%s\n",
					fastest.Name, formatDuration(fastest.AvgTime),
					slowest.Name, formatDuration(slowest.AvgTime), ratio, approx)
			}
		}
	}
	return out
}
