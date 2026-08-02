//go:build !windows

package agent

// 非 Windows 平台的让位检测 stub（Phase 1 仅 Windows 受控端）。
//
// Linux/macOS 留作 Phase 扩展点：
//   - Linux idle: 读 /proc/stat 或用 xprintidle / dbus org.gnome.Mutter.IdleMonitor
//   - 外部 GPU: 同样用 nvidia-smi pmon
// 当前返回 0/假值，使非 Windows 环境下 YieldDetector 退化为恒 IDLE（不干扰开发调试）。
