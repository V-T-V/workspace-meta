package core

import (
	"reflect"
	"testing"
)

// -----------------------------------------------------------------------------
// log.go
// -----------------------------------------------------------------------------

// TestLogAppend 验证 Append 返回的 Index 从 1 递增。
func TestLogAppend(t *testing.T) {
	var l Log
	cases := []struct {
		command any
		wantIdx uint64
	}{
		{"a", 1},
		{"b", 2},
		{"c", 3},
	}
	for i, tc := range cases {
		idx := l.Append(1, tc.command)
		if idx != tc.wantIdx {
			t.Errorf("第 %d 次 Append 返回 Index = %d, want %d", i+1, idx, tc.wantIdx)
		}
	}
	if len(l.Entries) != 3 {
		t.Errorf("3 次 Append 后 Entries 长度应为 3，实际 %d", len(l.Entries))
	}
	// 校验写入的 Term/Index/Command 字段。
	for i, e := range l.Entries {
		if e.Index != uint64(i+1) {
			t.Errorf("Entries[%d].Index = %d, want %d", i, e.Index, i+1)
		}
		if e.Term != 1 {
			t.Errorf("Entries[%d].Term = %d, want 1", i, e.Term)
		}
	}
}

// TestLogAppendTermPreserved 验证不同 Term 写入后 LastTerm 反映最新 Term。
func TestLogAppendTermPreserved(t *testing.T) {
	var l Log
	l.Append(1, "x")
	l.Append(2, "y")
	l.Append(2, "z")
	if l.LastTerm() != 2 {
		t.Errorf("LastTerm = %d, want 2", l.LastTerm())
	}
}

// TestLogLastIndexAndTerm 验证空日志返回 0，非空返回最后一条。
func TestLogLastIndexAndTerm(t *testing.T) {
	// 空日志。
	var empty Log
	if empty.LastIndex() != 0 {
		t.Errorf("空日志 LastIndex = %d, want 0", empty.LastIndex())
	}
	if empty.LastTerm() != 0 {
		t.Errorf("空日志 LastTerm = %d, want 0", empty.LastTerm())
	}

	// 非空。
	var l Log
	l.Append(1, "a")
	l.Append(3, "b")
	if l.LastIndex() != 2 {
		t.Errorf("LastIndex = %d, want 2", l.LastIndex())
	}
	if l.LastTerm() != 3 {
		t.Errorf("LastTerm = %d, want 3", l.LastTerm())
	}
}

// TestLogAt 验证按 Index 取条目，越界返回 false。
func TestLogAt(t *testing.T) {
	var l Log
	l.Append(1, "a")
	l.Append(2, "b")

	cases := []struct {
		name    string
		index   uint64
		wantOk  bool
		wantCmd any
	}{
		{"index 0（哨兵）", 0, false, nil},
		{"index 1", 1, true, "a"},
		{"index 2", 2, true, "b"},
		{"越界：index 3", 3, false, nil},
		{"远越界：index 100", 100, false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, ok := l.At(tc.index)
			if ok != tc.wantOk {
				t.Errorf("At(%d) ok = %v, want %v", tc.index, ok, tc.wantOk)
			}
			if ok && e.Command != tc.wantCmd {
				t.Errorf("At(%d).Command = %v, want %v", tc.index, e.Command, tc.wantCmd)
			}
		})
	}
}

// TestLogTruncate 验证截断后的日志长度与剩余条目。
func TestLogTruncate(t *testing.T) {
	var l Log
	for i := 0; i < 5; i++ {
		l.Append(uint64(i+1), string(rune('a'+i))) // a,b,c,d,e
	}
	// 从 index 3（含）截断，保留 1..2。
	l.Truncate(3)
	if len(l.Entries) != 2 {
		t.Errorf("Truncate(3) 后长度应为 2，实际 %d", len(l.Entries))
	}
	if l.LastIndex() != 2 {
		t.Errorf("Truncate(3) 后 LastIndex 应为 2，实际 %d", l.LastIndex())
	}
	// 截断后再追加应从 index 3 继续。
	idx := l.Append(3, "c2")
	if idx != 3 {
		t.Errorf("截断后 Append 应返回 3，实际 %d", idx)
	}
}

// TestLogTruncateEdgeCases 验证 Truncate 对边界 index 的处理。
func TestLogTruncateEdgeCases(t *testing.T) {
	// index 0：哨兵，应无操作。
	t.Run("index 0 不操作", func(t *testing.T) {
		var l Log
		l.Append(1, "x")
		l.Truncate(0)
		if len(l.Entries) != 1 {
			t.Errorf("Truncate(0) 后长度应仍为 1，实际 %d", len(l.Entries))
		}
	})
	// 越界 index：无操作。
	t.Run("越界 index 不操作", func(t *testing.T) {
		var l Log
		l.Append(1, "x")
		l.Truncate(100)
		if len(l.Entries) != 1 {
			t.Errorf("Truncate(100) 后长度应仍为 1，实际 %d", len(l.Entries))
		}
	})
	// 截断到 1：清空所有。
	t.Run("Truncate(1) 清空", func(t *testing.T) {
		var l Log
		l.Append(1, "x")
		l.Append(1, "y")
		l.Truncate(1)
		if len(l.Entries) != 0 {
			t.Errorf("Truncate(1) 后长度应为 0，实际 %d", len(l.Entries))
		}
	})
}

// TestLogCompact 验证日志压缩（snapshot 简化版）的核心语义。
func TestLogCompact(t *testing.T) {
	var l Log
	// 写 5 条：term=i, command=字母 a..e，Index=1..5。
	for i := 0; i < 5; i++ {
		l.Append(uint64(i+1), string(rune('a'+i)))
	}
	// Compact(2)：保留最后 2 条（d@4, e@5），丢弃前 3 条。
	dropped := l.Compact(2)
	if dropped != 3 {
		t.Fatalf("应丢弃 3 条，实际 %d", dropped)
	}
	if l.Length() != 2 {
		t.Errorf("压缩后 Length 应为 2，实际 %d", l.Length())
	}
	if l.BaseIndex != 3 {
		t.Errorf("压缩后 BaseIndex 应为 3，实际 %d", l.BaseIndex)
	}
	if l.BaseTerm != 3 { // 被丢弃的最后一条是 c@3，其 Term=3
		t.Errorf("压缩后 BaseTerm 应为 3（c 的 Term），实际 %d", l.BaseTerm)
	}
	// 逻辑 LastIndex 不变（仍为 5）。
	if l.LastIndex() != 5 {
		t.Errorf("压缩后 LastIndex 应仍为 5，实际 %d", l.LastIndex())
	}
	// 被压缩条目取不到，保留条目仍可取且 Index 不变。
	if _, ok := l.At(1); ok {
		t.Error("At(1) 应返回 false（已压缩）")
	}
	if e, ok := l.At(4); !ok || e.Command != "d" || e.Index != 4 {
		t.Errorf("At(4) 应为 d/Index=4，实际 ok=%v %+v", ok, e)
	}
	if e, ok := l.At(5); !ok || e.Command != "e" || e.Index != 5 {
		t.Errorf("At(5) 应为 e/Index=5，实际 ok=%v %+v", ok, e)
	}
}

// TestLogCompactEdgeCases 验证 Compact 的边界。
func TestLogCompactEdgeCases(t *testing.T) {
	t.Run("keepLast >= len 无操作", func(t *testing.T) {
		var l Log
		l.Append(1, "a")
		l.Append(1, "b")
		if d := l.Compact(5); d != 0 {
			t.Errorf("keepLast(5)>=len(2) 应丢弃 0，实际 %d", d)
		}
		if l.Length() != 2 || l.BaseIndex != 0 {
			t.Errorf("无操作压缩后应保持 Length=2 BaseIndex=0")
		}
	})
	t.Run("keepLast=0 丢弃全部", func(t *testing.T) {
		var l Log
		l.Append(1, "a")
		l.Append(2, "b")
		if d := l.Compact(0); d != 2 {
			t.Errorf("keepLast(0) 应丢弃 2，实际 %d", d)
		}
		if l.Length() != 0 {
			t.Errorf("keepLast(0) 后 Length 应为 0，实际 %d", l.Length())
		}
		if l.BaseIndex != 2 {
			t.Errorf("keepLast(0) 后 BaseIndex 应为 2，实际 %d", l.BaseIndex)
		}
		if l.BaseTerm != 2 { // b 的 Term
			t.Errorf("keepLast(0) 后 BaseTerm 应为 2，实际 %d", l.BaseTerm)
		}
		// 逻辑 LastIndex 跟随 BaseIndex（空日志返回 BaseIndex）。
		if l.LastIndex() != 2 {
			t.Errorf("全压缩后 LastIndex 应为 BaseIndex=2，实际 %d", l.LastIndex())
		}
	})
	t.Run("keepLast 负数按 0 处理", func(t *testing.T) {
		var l Log
		l.Append(1, "a")
		if d := l.Compact(-1); d != 1 {
			t.Errorf("keepLast(-1) 应按 0 处理丢弃 1，实际 %d", d)
		}
	})
	t.Run("多次压缩累加 BaseIndex", func(t *testing.T) {
		var l Log
		for i := 0; i < 6; i++ {
			l.Append(uint64(i+1), string(rune('a'+i)))
		}
		l.Compact(4) // 丢弃 a,b → BaseIndex=2, BaseTerm=2
		if l.BaseIndex != 2 {
			t.Fatalf("第一次压缩后 BaseIndex 应为 2，实际 %d", l.BaseIndex)
		}
		l.Compact(1) // 保留最后 1 条，再丢弃 3 条 → BaseIndex=5, BaseTerm=5
		if l.BaseIndex != 5 {
			t.Errorf("第二次压缩后 BaseIndex 应为 5，实际 %d", l.BaseIndex)
		}
		if l.BaseTerm != 5 {
			t.Errorf("第二次压缩后 BaseTerm 应为 5（e 的 Term），实际 %d", l.BaseTerm)
		}
		if l.Length() != 1 || l.LastIndex() != 6 {
			t.Errorf("两次压缩后应 Length=1 LastIndex=6，实际 Length=%d LastIndex=%d",
				l.Length(), l.LastIndex())
		}
	})
}

// TestLogCompactThenAppend 继续追加的 Index 连续性。
func TestLogCompactThenAppend(t *testing.T) {
	var l Log
	for i := 0; i < 5; i++ {
		l.Append(uint64(i+1), string(rune('a'+i)))
	}
	l.Compact(2) // 保留 d@4,e@5
	// 再 Append 应得 Index=6。
	idx := l.Append(6, "f")
	if idx != 6 {
		t.Errorf("压缩后 Append 应返回 6，实际 %d", idx)
	}
	e, ok := l.At(6)
	if !ok || e.Command != "f" || e.Term != 6 || e.Index != 6 {
		t.Errorf("At(6) 应为 f/Term=6/Index=6，实际 ok=%v %+v", ok, e)
	}
}

// TestLogTermAt 验证 TermAt 在压缩边界与非边界的返回值。
func TestLogTermAt(t *testing.T) {
	var l Log
	// term 序列：Index 1..5 → Term 1,1,2,2,2
	l.Append(1, "a")
	l.Append(1, "b")
	l.Append(2, "c")
	l.Append(2, "d")
	l.Append(2, "e")

	// 未压缩时。
	if got := l.TermAt(0); got != 0 {
		t.Errorf("TermAt(0) 应为 0，实际 %d", got)
	}
	if got := l.TermAt(3); got != 2 {
		t.Errorf("TermAt(3) 应为 2，实际 %d", got)
	}
	if got := l.TermAt(5); got != 2 {
		t.Errorf("TermAt(5) 应为 2，实际 %d", got)
	}
	if got := l.TermAt(6); got != 0 { // 越界
		t.Errorf("TermAt(6) 越界应为 0，实际 %d", got)
	}

	// 压缩：保留最后 2 条（d@4,e@5），丢弃 a,b,c。BaseIndex=3, BaseTerm=2（c 的 Term）。
	l.Compact(2)
	if l.BaseIndex != 3 || l.BaseTerm != 2 {
		t.Fatalf("压缩后 BaseIndex/BaseTerm 应为 3/2，实际 %d/%d", l.BaseIndex, l.BaseTerm)
	}
	// TermAt(BaseIndex) 应返回压缩时记录的 BaseTerm。
	if got := l.TermAt(3); got != 2 {
		t.Errorf("TermAt(BaseIndex=3) 应返回 BaseTerm=2，实际 %d", got)
	}
	// 保留区间正常。
	if got := l.TermAt(4); got != 2 {
		t.Errorf("TermAt(4) 应为 2，实际 %d", got)
	}
	// 已压缩区间内部（1,2）无法提供 Term → 0。
	if got := l.TermAt(1); got != 0 {
		t.Errorf("TermAt(1) 已压缩内部应为 0，实际 %d", got)
	}
	if got := l.TermAt(2); got != 0 {
		t.Errorf("TermAt(2) 已压缩内部应为 0，实际 %d", got)
	}
}

// TestLogLength 验证 Length 与压缩的关系。
func TestLogLength(t *testing.T) {
	var l Log
	if l.Length() != 0 {
		t.Errorf("空日志 Length 应为 0，实际 %d", l.Length())
	}
	for i := 0; i < 5; i++ {
		l.Append(1, "x")
	}
	if l.Length() != 5 {
		t.Errorf("5 条后 Length 应为 5，实际 %d", l.Length())
	}
	l.Compact(2)
	if l.Length() != 2 {
		t.Errorf("压缩保留 2 后 Length 应为 2，实际 %d", l.Length())
	}
}

// TestLogEmptyAfterCompactLastIndex 验证全压缩后空日志的 LastIndex 返回 BaseIndex。
func TestLogEmptyAfterCompactLastIndex(t *testing.T) {
	var l Log
	l.Append(1, "a")
	l.Append(1, "b")
	l.Compact(0) // 全压缩，BaseIndex=2
	// 空日志 LastIndex 返回 BaseIndex（表达"最后一条已压缩条目的 Index"）。
	if l.LastIndex() != 2 {
		t.Errorf("全压缩后空日志 LastIndex 应为 BaseIndex=2，实际 %d", l.LastIndex())
	}
	if l.Length() != 0 {
		t.Errorf("全压缩后 Length 应为 0，实际 %d", l.Length())
	}
}

// TestLogSlice 验证范围查询，含 start>end 返回 nil。
func TestLogSlice(t *testing.T) {
	var l Log
	for i := 0; i < 5; i++ {
		l.Append(uint64(i+1), string(rune('a'+i))) // 1..5: a b c d e
	}

	cases := []struct {
		name     string
		start    uint64
		end      uint64
		wantCmds []any
		wantNil  bool
	}{
		{"全量 1..5", 1, 5, []any{"a", "b", "c", "d", "e"}, false},
		{"子区间 2..4", 2, 4, []any{"b", "c", "d"}, false},
		{"单元素 3..3", 3, 3, []any{"c"}, false},
		{"start>end 返回 nil", 4, 2, nil, true},
		{"start=0（哨兵）返回 nil", 0, 3, nil, true},
		{"end 超过 lastIndex 应截断到末尾", 4, 100, []any{"d", "e"}, false},
		{"start 超过 lastIndex 返回 nil", 100, 100, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := l.Slice(tc.start, tc.end)
			if tc.wantNil {
				if got != nil {
					t.Errorf("Slice(%d,%d) 应返回 nil，实际 %v", tc.start, tc.end, got)
				}
				return
			}
			if len(got) != len(tc.wantCmds) {
				t.Errorf("Slice(%d,%d) 长度 = %d, want %d", tc.start, tc.end, len(got), len(tc.wantCmds))
				return
			}
			for i, e := range got {
				if e.Command != tc.wantCmds[i] {
					t.Errorf("Slice(%d,%d)[%d].Command = %v, want %v", tc.start, tc.end, i, e.Command, tc.wantCmds[i])
				}
			}
		})
	}
}

// TestLogSliceReturnsCopy 验证 Slice 返回副本，修改不影响原日志。
func TestLogSliceReturnsCopy(t *testing.T) {
	var l Log
	l.Append(1, "a")
	l.Append(1, "b")
	s := l.Slice(1, 2)
	if len(s) != 2 {
		t.Fatalf("Slice 长度应为 2，实际 %d", len(s))
	}
	// 修改切片元素不应改变原日志（LogEntry 是值类型，append 复制了值）。
	s[0].Command = "HACKED"
	if e, _ := l.At(1); e.Command != "a" {
		t.Errorf("修改 Slice 副本不应影响原日志：原 Entries[0].Command = %v, want a", e.Command)
	}
}

// -----------------------------------------------------------------------------
// transport.go
// -----------------------------------------------------------------------------

// TestMemTransportSendAndDrain 验证投递 + Handler 回复 + 回复再投递的完整往返。
func TestMemTransportSendAndDrain(t *testing.T) {
	tr := NewMemTransport()
	// b 的 handler：收到任何消息都回一个 echo，handled=true。
	echoHandler := func(msg Message) (Message, bool) {
		return Message{From: msg.To, To: msg.From, Kind: "echo", Payload: "echo:" + msg.Kind}, true
	}
	tr.Install("a", func(msg Message) (Message, bool) {
		// a 不主动回复，仅用于接收 b 的 echo。
		return Message{}, false
	})
	tr.Install("b", echoHandler)

	// a 向 b 发一条消息。
	tr.Send(Message{From: "a", To: "b", Kind: "ping", Payload: "hello"})

	// 第一轮 Drain：b 收到 ping，回 echo（echo 入队）。
	handled1 := tr.Drain()
	if len(handled1) != 1 {
		t.Fatalf("第一轮 Drain 应处理 1 条，实际 %d", len(handled1))
	}
	if handled1[0].Kind != "ping" {
		t.Errorf("第一轮处理的消息 Kind = %s, want ping", handled1[0].Kind)
	}

	// 第二轮 Drain：a 收到 b 的 echo（a 不回复，handled=false，不计入）。
	handled2 := tr.Drain()
	if len(handled2) != 0 {
		t.Errorf("第二轮 Drain 处理的消息数应为 0（a 不回复），实际 %d", len(handled2))
	}

	// 第三轮 Drain：队列应已空，返回 nil。
	if got := tr.Drain(); got != nil {
		t.Errorf("空队列 Drain 应返回 nil，实际 %v", got)
	}
}

// TestMemTransportInstallOverwrite 验证 Install 可覆盖已注册的 Handler。
func TestMemTransportInstallOverwrite(t *testing.T) {
	tr := NewMemTransport()
	var callCount int
	tr.Install("x", func(msg Message) (Message, bool) {
		callCount++
		return Message{}, false
	})
	tr.Install("x", func(msg Message) (Message, bool) {
		// 覆盖：不计数。
		return Message{}, false
	})
	tr.Send(Message{From: "y", To: "x", Kind: "k"})
	tr.Drain()
	if callCount != 0 {
		t.Errorf("覆盖后的旧 Handler 不应被调用，实际 callCount=%d", callCount)
	}
}

// TestMemTransportUnregisteredDropped 验证未注册目标的消息被丢弃。
func TestMemTransportUnregisteredDropped(t *testing.T) {
	tr := NewMemTransport()
	tr.Install("a", func(msg Message) (Message, bool) {
		// a 处理消息并标记 handled=true（但不回 reply），以便 Drain 把它计入返回切片。
		return Message{}, true
	})
	// 发给未注册的 "ghost"（无 handler，应被丢弃，不 panic）。
	tr.Send(Message{From: "a", To: "ghost", Kind: "k"})
	// 发给已注册的 "a"。
	tr.Send(Message{From: "ghost", To: "a", Kind: "k"})

	handled := tr.Drain()
	// 只有发给 "a" 的那条被处理（注册了 handler），"ghost" 那条被丢弃。
	if len(handled) != 1 {
		t.Errorf("应只处理 1 条（给已注册 a 的），实际 %d", len(handled))
	}
	if len(handled) > 0 && handled[0].To != "a" {
		t.Errorf("被处理的消息 To 应为 a，实际 %s", handled[0].To)
	}
}

// TestMemTransportReplyRienqueued 验证 reply 被重新入队，下一轮 Drain 投递。
func TestMemTransportReplyRienqueued(t *testing.T) {
	tr := NewMemTransport()
	gotReply := make(chan Message, 4)
	tr.Install("server", func(msg Message) (Message, bool) {
		// 收到请求立即回复。
		return Message{From: "server", To: msg.From, Kind: "reply", Payload: msg.Payload}, true
	})
	tr.Install("client", func(msg Message) (Message, bool) {
		gotReply <- msg
		return Message{}, false
	})

	tr.Send(Message{From: "client", To: "server", Kind: "req", Payload: "p1"})
	// 推进两轮：第一轮 server 回 reply 入队；第二轮 client 收到。
	tr.Drain()
	tr.Drain()

	select {
	case r := <-gotReply:
		if r.Kind != "reply" || r.Payload != "p1" {
			t.Errorf("client 收到 reply = %+v, want Kind=reply Payload=p1", r)
		}
	default:
		t.Error("client 应收到 server 的 reply，但未收到")
	}
}

// TestMemTransportEmptyReplyNotEnqueued 验证 handled=true 但 reply 全空时不入队。
func TestMemTransportEmptyReplyNotEnqueued(t *testing.T) {
	tr := NewMemTransport()
	tr.Install("a", func(msg Message) (Message, bool) {
		// handled=true 但返回零值 Message（From/To 都空）。
		return Message{}, true
	})
	tr.Send(Message{From: "b", To: "a", Kind: "k"})
	handled := tr.Drain()
	// a 处理了消息（handled 计入），但 reply 全空不入队。
	if len(handled) != 1 {
		t.Errorf("应处理 1 条，实际 %d", len(handled))
	}
	if got := tr.Drain(); got != nil {
		t.Errorf("零值 reply 不应入队，第二轮 Drain 应返回 nil，实际 %v", got)
	}
}

// -----------------------------------------------------------------------------
// clock.go
// -----------------------------------------------------------------------------

// TestLamportClockTick 验证 Tick 每次递增 1，Now 跟踪当前值。
func TestLamportClockTick(t *testing.T) {
	var lc LamportClock
	if lc.Now() != 0 {
		t.Errorf("初始 Now 应为 0，实际 %d", lc.Now())
	}
	for i := uint64(1); i <= 5; i++ {
		got := lc.Tick()
		if got != i {
			t.Errorf("第 %d 次 Tick = %d, want %d", i, got, i)
		}
		if lc.Now() != i {
			t.Errorf("第 %d 次 Tick 后 Now = %d, want %d", i, lc.Now(), i)
		}
	}
}

// TestLamportClockObserve 验证 Observe 的 max+1 规则。
func TestLamportClockObserve(t *testing.T) {
	cases := []struct {
		name  string
		local uint64 // 预设的本地时钟（用 Tick 推到）
		other uint64
		want  uint64
	}{
		{"本地 5 收 8 → 9（other 更大）", 5, 8, 9},
		{"本地 8 收 5 → 9（local 更大）", 8, 5, 9},
		{"本地 5 收 5 → 6（相等）", 5, 5, 6},
		{"本地 0 收 0 → 1（都为 0）", 0, 0, 1},
		{"本地 3 收 10 → 11（other 远大）", 3, 10, 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var lc LamportClock
			for i := uint64(0); i < tc.local; i++ {
				lc.Tick()
			}
			got := lc.Observe(tc.other)
			if got != tc.want {
				t.Errorf("Observe(%d) [local=%d] = %d, want %d", tc.other, tc.local, got, tc.want)
			}
			if lc.Now() != tc.want {
				t.Errorf("Observe 后 Now = %d, want %d", lc.Now(), tc.want)
			}
		})
	}
}

// TestLamportClockImplementsInterface 验证 LamportClock 实现 Clock 接口。
func TestLamportClockImplementsInterface(t *testing.T) {
	var _ Clock = (*LamportClock)(nil)
}

// TestLamportClockTickThenObserve 验证混合调用序列的单调性。
func TestLamportClockTickThenObserve(t *testing.T) {
	var lc LamportClock
	seq := []struct {
		op   string // "tick" or "observe"
		arg  uint64
		want uint64
	}{
		{"tick", 0, 1},
		{"tick", 0, 2},
		{"observe", 5, 6},
		{"tick", 0, 7},
		{"observe", 7, 8},
		{"observe", 100, 101},
	}
	for i, s := range seq {
		var got uint64
		switch s.op {
		case "tick":
			got = lc.Tick()
		case "observe":
			got = lc.Observe(s.arg)
		}
		if got != s.want {
			t.Errorf("步骤 %d (%s) = %d, want %d", i, s.op, got, s.want)
		}
	}
}

// -----------------------------------------------------------------------------
// node.go
// -----------------------------------------------------------------------------

// TestNodeStateString 验证各状态返回正确名称，表驱动。
func TestNodeStateString(t *testing.T) {
	cases := []struct {
		state NodeState
		want  string
	}{
		{StateFollower, "follower"},
		{StateCandidate, "candidate"},
		{StateLeader, "leader"},
		{StatePrimary, "primary"},
		{StateReplica, "replica"},
		{NodeState(999), "unknown"},
		{NodeState(-1), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("NodeState(%d).String() = %q, want %q", tc.state, got, tc.want)
		}
	}
}

// TestNodeIDType 验证 NodeID 是 string 别名，可比较、可赋值。
func TestNodeIDType(t *testing.T) {
	var id NodeID = "node-1"
	if id != "node-1" {
		t.Errorf("NodeID 赋值/比较失败：got %q", id)
	}
	if string(id) != "node-1" {
		t.Errorf("NodeID 转 string 失败：got %q", string(id))
	}
	// 用作 map key。
	m := map[NodeID]int{id: 42}
	if m["node-1"] != 42 {
		t.Error("NodeID 作为 map key 失败")
	}
}

// TestMessageFields 验证 Message 结构体字段可正确构造与读取。
func TestMessageFields(t *testing.T) {
	m := Message{
		From:    "a",
		To:      "b",
		Kind:    "test",
		Term:    7,
		Payload: struct{ X int }{X: 9},
	}
	if m.From != "a" || m.To != "b" || m.Kind != "test" || m.Term != 7 {
		t.Errorf("Message 字段未正确设置: %+v", m)
	}
	p, ok := m.Payload.(struct{ X int })
	if !ok || p.X != 9 {
		t.Errorf("Message.Payload 类型断言失败: got %+v ok=%v", p, ok)
	}
}

// TestLogEntryZeroValue 验证 LogEntry 零值。
func TestLogEntryZeroValue(t *testing.T) {
	var e LogEntry
	if e.Term != 0 || e.Index != 0 || e.Command != nil {
		t.Errorf("LogEntry 零值不符：got %+v", e)
	}
	if !reflect.DeepEqual(e, LogEntry{}) {
		t.Error("LogEntry 零值不等于 LogEntry{}")
	}
}
