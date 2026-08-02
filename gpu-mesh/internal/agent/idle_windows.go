//go:build windows

package agent

import (
	"syscall"
	"unsafe"
)

// Windows idle 检测实现（零 CGO，纯 syscall）。
//
// GetLastInputInfo：返回最后一次键盘/鼠标输入的系统滴答数，与 GetTickCount 差值即空闲秒数。
// GetForegroundWindow：返回当前前台窗口句柄，周期采样变化 → 人在主动切窗口。

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetLastInputInfo    = user32.NewProc("GetLastInputInfo")
	procGetTickCount64      = kernel32.NewProc("GetTickCount64") // 注意：在 kernel32，不在 user32
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
)

type lastInputInfo struct {
	CbSize uint32
	DwTime uint32 // 最后输入的系统滴答数
}

var lastForeground uintptr // 上次采样的前台窗口句柄

func init() {
	getIdleSeconds = idleSecondsWindows
	foregroundChangedSinceLast = foregroundChangedWindows
}

// idleSecondsWindows 通过 GetLastInputInfo 计算键鼠空闲秒数。
// 任何 API 失败都降级返回 0（保守视为 BUSY，避免误判机器空闲而抢占用户资源）。
func idleSecondsWindows() int {
	defer func() { _ = recover() }() // 防 syscall 异常导致整个 Agent 崩溃

	li := lastInputInfo{CbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&li)))
	if r == 0 {
		return 0 // GetLastInputInfo 失败
	}
	now, _, _ := procGetTickCount64.Call()
	if now == 0 {
		return 0 // GetTickCount64 失败
	}
	// GetTickCount64 是 64 位，GetLastInputInfo 是 32 位滴答（系统启动后毫秒）
	diff := int64(now) - int64(li.DwTime)
	if diff < 0 {
		return 0
	}
	return int(diff / 1000)
}

// foregroundChangedWindows 检测前台窗口自上次调用是否变化。
func foregroundChangedWindows() bool {
	defer func() { _ = recover() }()
	hwnd, _, _ := procGetForegroundWindow.Call()
	changed := hwnd != 0 && hwnd != lastForeground
	lastForeground = hwnd
	return changed
}
