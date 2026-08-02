package relay

import (
	"encoding/json"
	"sync"
	"time"
)

// Event 控制台订阅的事件。
type Event struct {
	Kind    string          // 事件类型（agent_online/agent_offline/yield_update/task_result/...）
	AgentID string          // 关联 Agent（可为空）
	TS      int64           // Unix 毫秒
	Payload json.RawMessage // 结构化载荷（可为空）
}

// EventBus SSE 事件广播总线（fan-out 到多个 Web 控制台订阅者）。
//
// 设计为有缓冲订阅：慢消费者周期丢弃旧事件（避免一个慢控制台拖垮整个 Relay）。
type EventBus struct {
	mu          sync.Mutex
	nextSubID   int
	subscribers map[int]chan Event
	dropped     int64
}

// NewEventBus 构造事件总线。
func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[int]chan Event)}
}

// Subscribe 订阅事件。返回订阅 ID 与事件 channel。
// buf 为 channel 缓冲大小（慢消费者满则丢旧事件）。
func (b *EventBus) Subscribe(buf int) (int, <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextSubID++
	id := b.nextSubID
	if buf <= 0 {
		buf = 64
	}
	ch := make(chan Event, buf)
	b.subscribers[id] = ch
	return id, ch
}

// Unsubscribe 取消订阅。
func (b *EventBus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(ch)
	}
}

// Broadcast 向所有订阅者广播事件。慢消费者丢弃最旧事件。
func (b *EventBus) Broadcast(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
			// 缓冲满，丢最旧事件腾位置
			select {
			case <-ch:
				b.dropped++
			default:
			}
			select {
			case ch <- ev:
			default:
				b.dropped++ // 实在塞不进（极端并发）
			}
		}
	}
}

// Dropped 返回累计丢弃事件数。
func (b *EventBus) Dropped() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dropped
}

// stampNow 当前 Unix 毫秒。
func stampNow() int64 { return time.Now().UnixMilli() }
