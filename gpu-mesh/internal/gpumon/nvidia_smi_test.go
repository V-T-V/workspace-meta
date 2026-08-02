package gpumon

import (
	"testing"
)

func TestParseCSV_Single4060(t *testing.T) {
	// 典型 RTX 4060 8GB 输出（noheader,nounits）
	raw := "0, NVIDIA GeForce RTX 4060, 35, 40, 3456, 8192, 52, 95. 50, 170.00\n"
	snapshots, err := ParseCSV(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("期望 1 张 GPU，得到 %d", len(snapshots))
	}
	s := snapshots[0]
	if s.Index != 0 {
		t.Errorf("Index: 期望 0，得到 %d", s.Index)
	}
	if s.Name != "NVIDIA GeForce RTX 4060" {
		t.Errorf("Name: 得到 %q", s.Name)
	}
	if s.UtilGPU != 35 {
		t.Errorf("UtilGPU: 期望 35，得到 %v", s.UtilGPU)
	}
	if s.MemTotalMB != 8192 {
		t.Errorf("MemTotalMB: 期望 8192，得到 %d", s.MemTotalMB)
	}
	if s.MemUsedMB != 3456 {
		t.Errorf("MemUsedMB: 期望 3456，得到 %d", s.MemUsedMB)
	}
	if s.TempC != 52 {
		t.Errorf("TempC: 期望 52，得到 %d", s.TempC)
	}
	// "95. 50" 这种异常空格应被 atofOrZero 容错（解析失败归 0）
	if s.PowerW != 0 {
		t.Errorf("PowerW: 异常值应归 0，得到 %v", s.PowerW)
	}
	if s.PowerLimitW != 170 {
		t.Errorf("PowerLimitW: 期望 170，得到 %v", s.PowerLimitW)
	}
}

func TestParseCSV_MultiGPU(t *testing.T) {
	// 双卡场景
	raw := "0, NVIDIA GeForce RTX 4060, 10, 5, 512, 8192, 45, 30.5, 170.00\n" +
		"1, NVIDIA GeForce RTX 4060, 80, 60, 6000, 8192, 70, 150.2, 170.00\n"
	snapshots, err := ParseCSV(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("期望 2 张 GPU，得到 %d", len(snapshots))
	}
	if snapshots[1].UtilGPU != 80 {
		t.Errorf("GPU1 UtilGPU: 期望 80，得到 %v", snapshots[1].UtilGPU)
	}
	if snapshots[1].MemUsedMB != 6000 {
		t.Errorf("GPU1 MemUsedMB: 期望 6000，得到 %d", snapshots[1].MemUsedMB)
	}
}

func TestParseCSV_NAValues(t *testing.T) {
	// power.draw 在某些情况下为 "N/A"（如笔记本未接电源）
	raw := "0, NVIDIA GeForce RTX 4060 Laptop GPU, 0, 0, 0, 8192, 30, [N/A], 170.00\n"
	snapshots, err := ParseCSV(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if snapshots[0].PowerW != 0 {
		t.Errorf("[N/A] 应归 0，得到 %v", snapshots[0].PowerW)
	}
}

func TestParseCSV_Empty(t *testing.T) {
	if _, err := ParseCSV(""); err != ErrNoGPU {
		t.Errorf("空输入应返回 ErrNoGPU，得到 %v", err)
	}
	if _, err := ParseCSV("\n\n"); err != ErrNoGPU {
		t.Errorf("纯空行应返回 ErrNoGPU，得到 %v", err)
	}
}

func TestParseCSV_MalformedLine(t *testing.T) {
	// 字段不足的坏行应被跳过，不影响好行
	raw := "0, broken line\n" +
		"1, NVIDIA GeForce RTX 4060, 10, 5, 512, 8192, 45, 30, 170\n"
	snapshots, err := ParseCSV(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("坏行应被跳过，期望 1 张 GPU，得到 %d", len(snapshots))
	}
	if snapshots[0].Index != 1 {
		t.Errorf("应保留好行 GPU1，得到 Index %d", snapshots[0].Index)
	}
}

func TestAtofOrZero(t *testing.T) {
	cases := map[string]float64{
		"35":     35,
		"35.5":   35.5,
		"":       0,
		"N/A":    0,
		"[N/A]":  0,
		"  12  ": 12,
	}
	for in, want := range cases {
		if got := atofOrZero(in); got != want {
			t.Errorf("atofOrZero(%q) = %v, want %v", in, got, want)
		}
	}
}
