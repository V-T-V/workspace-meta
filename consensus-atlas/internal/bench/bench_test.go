package bench

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestBenchmarkBasic 验证 Benchmark 基本统计（耗时/平均/throughput）。
func TestBenchmarkBasic(t *testing.T) {
	// 一个快函数（sleep 1ms 模拟工作）
	fn := func(ctx context.Context) error {
		time.Sleep(time.Millisecond)
		return nil
	}
	r := Benchmark("fake", fn, 5)
	if r.Err != nil {
		t.Fatalf("不应报错: %v", r.Err)
	}
	if r.Runs != 5 {
		t.Errorf("Runs 应为 5，实际 %d", r.Runs)
	}
	if r.AvgTime < time.Microsecond {
		t.Errorf("AvgTime 应 >= 1ms（sleep 1ms），实际 %s", r.AvgTime)
	}
	if r.Throughput <= 0 {
		t.Error("Throughput 应 > 0")
	}
}

// TestBenchmarkError 验证函数报错时 Result.Err 被记录。
func TestBenchmarkError(t *testing.T) {
	fn := func(ctx context.Context) error {
		return fmt.Errorf("模拟失败")
	}
	r := Benchmark("bad", fn, 3)
	if r.Err == nil {
		t.Error("应记录 error")
	}
}

// TestBenchmarkZeroRuns 验证 runs<=0 时默认为 10。
func TestBenchmarkZeroRuns(t *testing.T) {
	fn := func(ctx context.Context) error { return nil }
	r := Benchmark("x", fn, 0)
	if r.Runs != 10 {
		t.Errorf("runs=0 应默认 10，实际 %d", r.Runs)
	}
}

// TestFormatResult 验证格式化输出。
func TestFormatResult(t *testing.T) {
	r := Result{Name: "test", Runs: 10, AvgTime: 5 * time.Microsecond, Throughput: 200000}
	s := FormatResult(r)
	if s == "" {
		t.Error("FormatResult 不应为空")
	}
	// 错误的也要能格式化
	re := Result{Name: "bad", Err: fmt.Errorf("x")}
	if FormatResult(re) == "" {
		t.Error("错误结果也应能格式化")
	}
}

// TestFormatResultSubMicrosecond 验证 <1µs 的极快 demo 不被截断成 "0s"
// （之前的精度 bug：Round(µs) 把 320ns 显示成 0s，throughput 显示 1e9 次/秒失真）。
// 修复后：纳秒整数原样显示，并标注 "≈/*" 提醒 throughput 仅量级参考。
func TestFormatResultSubMicrosecond(t *testing.T) {
	r := Result{Name: "clock", Runs: 1000, AvgTime: 320 * time.Nanosecond, Throughput: 3.125e6}
	s := FormatResult(r)
	if !strings.Contains(s, "320ns") {
		t.Errorf("应保留纳秒精度显示 320ns，实际 %q", s)
	}
	if !strings.Contains(s, "≈") || !strings.Contains(s, "*") {
		t.Errorf("亚微秒结果应标注 ≈/* 提醒精度受限，实际 %q", s)
	}
}

// TestFormatDuration 验证自适应时长格式不被 µs 截断。
func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{1 * time.Nanosecond, "1ns"},
		{500 * time.Nanosecond, "500ns"},
		{999 * time.Nanosecond, "999ns"},
		{5 * time.Microsecond, "5.000µs"},
		{12345 * time.Nanosecond, "12.345µs"},
	}
	for _, c := range cases {
		got := formatDuration(c.d)
		if got != c.want {
			t.Errorf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

// TestFormatSummary 验证对比表 + 最快/最慢。
func TestFormatSummary(t *testing.T) {
	s := Summary{Results: []Result{
		{Name: "a", Runs: 10, AvgTime: 10 * time.Microsecond, Throughput: 100000},
		{Name: "b", Runs: 10, AvgTime: 50 * time.Microsecond, Throughput: 20000},
	}}
	out := FormatSummary(s)
	if out == "" {
		t.Error("FormatSummary 不应为空")
	}
	// 应含最快/最慢对比
	if !contains(out, "最快") {
		t.Error("应含最快算法")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
