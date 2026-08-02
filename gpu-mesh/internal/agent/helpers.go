package agent

import (
	"encoding/json"
	"os"
	"runtime"
	"sync/atomic"
)

// Version Agent 二进制版本（编译时可 -ldflags 注入）。
const Version = "0.1.0"

// 序列化辅助（集中管理，便于将来切换 sonic 等高性能 JSON）。
func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// hostname 返回本机主机名。
func hostname() string {
	hn, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hn
}

// osName 返回 GOOS/GOARCH。
func osName() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// seq 原子递增序号，用于心跳防丢包检测。
var seqCounter int64

func (a *Agent) nextSeq() int64 { return atomic.AddInt64(&seqCounter, 1) }
