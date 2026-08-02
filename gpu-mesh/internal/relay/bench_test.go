package relay

import (
	"encoding/json"
	"testing"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// === 性能基准测试（摸底，优化前后对比）===

// makeAgents 构造 N 个测试 Agent 视图。
func makeAgents(n int) []AgentView {
	out := make([]AgentView, n)
	for i := 0; i < n; i++ {
		out[i] = AgentView{
			AgentID:  "agent-" + itoa(i),
			Hostname: "host-" + itoa(i),
			OS:       "windows/amd64",
			Engines:  []string{"ollama"},
			Models:   []string{"qwen2.5:7b", "nomic-embed-text"},
			Tags:     map[string]string{"gpu": "4060", "region": "bj"},
			GPUs: []proto.GPUSnapshot{{
				Name: "NVIDIA GeForce RTX 4060", UtilGPU: float64(i % 100),
				MemUsedMB: 2048 + i, MemTotalMB: 8192, TempC: 50, PowerW: 100, PowerLimitW: 170,
			}},
			Yield:  proto.YieldState{Level: proto.YieldIDLE, Budget: 100},
			Online: true,
		}
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := [12]byte{}
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// BenchmarkScheduler_Schedule_100 测 100 节点调度的决策延迟（每次推理请求都走）。
func BenchmarkScheduler_Schedule_100(b *testing.B) {
	s := NewScheduler()
	agents := makeAgents(100)
	req := ScheduleRequest{Model: "qwen2.5:7b", MinBudget: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, _ := s.Schedule(req, agents)
		s.Release(id) // 释放槽位，保持基准干净
	}
}

// BenchmarkScheduler_Schedule_1000 测 1000 节点（未来规模）。
func BenchmarkScheduler_Schedule_1000(b *testing.B) {
	s := NewScheduler()
	agents := makeAgents(1000)
	req := ScheduleRequest{Model: "qwen2.5:7b", MinBudget: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, _ := s.Schedule(req, agents)
		s.Release(id)
	}
}

// BenchmarkRegistry_Snapshot_100 测百节点 Snapshot 拷贝（每个 API 请求都走）。
func BenchmarkRegistry_Snapshot_100(b *testing.B) {
	r := NewRegistry()
	for i := 0; i < 100; i++ {
		reg := proto.AgentRegister{
			AgentID: "agent-" + itoa(i), Hostname: "h", OS: "w", Version: "v",
			Engines: []string{"ollama"}, Models: []string{"m"},
			GPUs: []proto.GPUSnapshot{{Name: "4060", MemTotalMB: 8192}},
			Yield: proto.YieldState{Level: proto.YieldIDLE, Budget: 100},
		}
		r.Register(reg, nil)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Snapshot()
	}
}

// BenchmarkHeartbeatJSON 测心跳 JSON 编解码（百台 × 5s 心跳高频）。
func BenchmarkHeartbeatJSON(b *testing.B) {
	hb := proto.AgentHeartbeat{
		AgentID: "agent-001",
		GPUs: []proto.GPUSnapshot{{
			Name: "NVIDIA GeForce RTX 4060", UtilGPU: 45.5,
			MemUsedMB: 3456, MemTotalMB: 8192, TempC: 52, PowerW: 95.5, PowerLimitW: 170,
		}},
		Yield: proto.YieldState{Level: proto.YieldIDLE, Budget: 100, IdleSeconds: 600},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(hb)
		var out proto.AgentHeartbeat
		json.Unmarshal(data, &out)
	}
}

// BenchmarkTaskResultJSON 测任务结果 JSON 编解码（每请求回流）。
func BenchmarkTaskResultJSON(b *testing.B) {
	result := proto.TaskResult{
		TaskID: "gw-abc12345", Success: true, ExitCode: 0,
		Data: proto.MarshalData(proto.InferenceResult{
			Content: "这是一段示例推理结果文本", Model: "qwen2.5:7b",
			DoneReason: "stop", PromptTokens: 100, CompletionTokens: 50,
		}),
		Duration: 1234,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(result)
		var out proto.TaskResult
		json.Unmarshal(data, &out)
	}
}
