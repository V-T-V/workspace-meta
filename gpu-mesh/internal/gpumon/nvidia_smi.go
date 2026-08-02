// Package gpumon 负责采集 GPU 状态快照。
//
// 设计原则：零 CGO。通过解析 nvidia-smi 命令输出获取 GPU 指标，
// 适用于所有装了 NVIDIA 驱动的机器（Windows/Linux 通用），无需任何 Go 绑定库。
//
// 采集命令（CSV 格式，便于稳定解析）：
//
//	nvidia-smi --query-gpu=index,name,utilization.gpu,utilization.memory,
//	            memory.used,memory.total,temperature.gpu,power.draw,power.limit
//	          --format=csv,noheader,nounits
package gpumon

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// ErrNoGPU 机器上没有可用的 NVIDIA GPU（nvidia-smi 不存在或失败）。
var ErrNoGPU = errors.New("no nvidia gpu available")

// 默认 nvidia-smi 路径。Linux 一般在 PATH；Windows 在固定安装目录。
var nvidiaSMICmd = "nvidia-smi"

// QueryFields 与 nvidia-smi --query-gpu 的字段顺序一一对应。
const queryFields = "index,name,utilization.gpu,utilization.memory," +
	"memory.used,memory.total,temperature.gpu,power.draw,power.limit"

func init() {
	// Windows 上 nvidia-smi 通常不在 PATH，补默认安装路径。
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			`C:\Windows\System32\nvidia-smi.exe`,
			`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`,
		} {
			if _, err := exec.LookPath(p); err == nil {
				nvidiaSMICmd = p
				return
			}
		}
	}
}

// SnapshotOnce 执行一次 nvidia-smi，返回所有 GPU 的快照。
// 非 NVIDIA 机器返回 ErrNoGPU，调用方应降级为空切片。
func SnapshotOnce(ctx context.Context) ([]proto.GPUSnapshot, error) {
	cmd := exec.CommandContext(ctx, nvidiaSMICmd,
		"--query-gpu", queryFields,
		"--format=csv,noheader,nounits",
	)
	out, err := cmd.Output()
	if err != nil {
		// nvidia-smi 不存在或驱动未装
		if execErr, ok := err.(*exec.Error); ok && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, ErrNoGPU
		}
		// 退出码非 0（如 [NVML Not Initialized]）也视为无可用 GPU
		if _, ok := err.(*exec.ExitError); ok {
			return nil, ErrNoGPU
		}
		return nil, err
	}
	return ParseCSV(string(out))
}

// ParseCSV 解析 nvidia-smi 的 CSV 输出为 GPU 快照切片。
//
// 行格式（noheader,nounits）：每行一张 GPU，字段按 queryFields 顺序：
//
//	0=index, 1=name, 2=utilization.gpu, 3=utilization.memory,
//	4=memory.used(MiB), 5=memory.total(MiB), 6=temperature.gpu(℃),
//	7=power.draw(W), 8=power.limit(W)
//
// 注意：数值字段已是纯数字（nounits），但可能含空格或 "N/A"（如未接外接电源时 power.draw）。
func ParseCSV(raw string) ([]proto.GPUSnapshot, error) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	snapshots := make([]proto.GPUSnapshot, 0, len(lines))
	now := time.Now().UnixMilli()
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 9 {
			continue // 字段不全，跳过坏行
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		s := proto.GPUSnapshot{
			Index:       atoiOrZero(fields[0]),
			Name:        fields[1],
			UtilGPU:     atofOrZero(fields[2]),
			UtilMem:     atofOrZero(fields[3]),
			MemUsedMB:   atoiOrZero(fields[4]),
			MemTotalMB:  atoiOrZero(fields[5]),
			TempC:       atoiOrZero(fields[6]),
			PowerW:      atofOrZero(fields[7]),
			PowerLimitW: atofOrZero(fields[8]),
			TS:          now,
		}
		snapshots = append(snapshots, s)
	}
	if len(snapshots) == 0 {
		return nil, ErrNoGPU
	}
	return snapshots, nil
}

// atoiOrZero 解析整数，失败/N/A 返回 0。
func atoiOrZero(s string) int {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// atofOrZero 解析浮点，失败/N/A 返回 0。
func atofOrZero(s string) float64 {
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
