package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// === 集成测试基础设施 ===
//
// 用真实 Relay（httptest）+ 模拟 Agent（WS 客户端）验证多组件协作链路。
// 这些测试覆盖单元测试无法触达的：WS 通信、调度选择、让位重调度、批量并行、故障转移。

// testCluster 测试集群：一个 Relay + 多个 mock Agent。
type testCluster struct {
	t      *testing.T
	relay  *Relay
	server *httptest.Server
	agents []*mockAgent
}

// newTestCluster 启动一个带真实路由的 Relay（httptest）。
func newTestCluster(t *testing.T) *testCluster {
	t.Helper()
	dir := t.TempDir()
	r, err := New(Config{
		Store:     true,
		StorePath: dir + "/test.db",
	})
	if err != nil {
		t.Fatalf("New Relay 失败: %v", err)
	}
	mux := setupTestMux(r)
	server := httptest.NewServer(mux)
	r.StartSweeper(context.Background())
	return &testCluster{t: t, relay: r, server: server}
}

// setupTestMux 挂载 Relay 所有路由（复用生产路由配置）。
func setupTestMux(r *Relay) *http.ServeMux {
	mux := http.NewServeMux()
	r.Routes(mux)
	return mux
}

// teardown 关闭集群。
func (c *testCluster) teardown() {
	for _, a := range c.agents {
		a.close()
	}
	c.server.Close()
	c.relay.Close()
}

// wsURL 返回 Agent 接入的 WS URL。
func (c *testCluster) wsURL() string {
	u, _ := url.Parse(c.server.URL)
	return "ws://" + u.Host + "/agent"
}

// httpURL 返回 HTTP API 基址。
func (c *testCluster) httpURL() string { return c.server.URL }

// === mock Agent ===

// mockAgent 模拟一个真实 Agent：WS 连接 + 注册 + 心跳 + 任务处理。
type mockAgent struct {
	id      string
	conn    *websocket.Conn
	yield   proto.YieldState
	models  []string
	engines []string
	gpus    []proto.GPUSnapshot

	// taskHandler 自定义任务处理函数（返回结果）。默认返回成功。
	taskHandler func(task proto.TaskRequest) proto.TaskResult
	// 收到的任务记录（供测试断言）
	receivedTasks []proto.TaskRequest
	mu            sync.Mutex
	closeOnce     sync.Once
}

// newMockAgent 创建并连接一个 mock Agent 到 Relay。
func (c *testCluster) newMockAgent(id string, yield proto.YieldState) *mockAgent {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, c.wsURL(), nil)
	if err != nil {
		c.t.Fatalf("mockAgent %s 连接 Relay 失败: %v", id, err)
	}
	conn.SetReadLimit(64 << 20)
	a := &mockAgent{
		id:      id,
		conn:    conn,
		yield:   yield,
		engines: []string{"ollama"},
		models:  []string{"test-model"},
		taskHandler: func(task proto.TaskRequest) proto.TaskResult {
			return proto.TaskResult{TaskID: task.TaskID, Success: true, Data: proto.MarshalData(proto.InferenceResult{Content: "ok", Model: "test-model"})}
		},
	}
	// 发注册
	a.sendRegister()
	// 启动读循环（处理 Relay 下发的任务）
	go a.readLoop()
	c.agents = append(c.agents, a)
	// 等待 Relay 注册生效
	time.Sleep(100 * time.Millisecond)
	return a
}

// sendRegister 发送注册消息。
func (a *mockAgent) sendRegister() {
	reg := proto.AgentRegister{
		AgentID: a.id, Hostname: a.id, OS: "test/amd64", Version: "test",
		GPUs: a.gpus, Engines: a.engines, Models: a.models, Yield: a.yield,
	}
	env, _ := proto.NewEnvelope(proto.TypeRegister, a.id, "relay", reg)
	a.write(env)
}

// sendHeartbeat 发送心跳（带最新 yield + gpu）。
func (a *mockAgent) sendHeartbeat() {
	hb := proto.AgentHeartbeat{AgentID: a.id, GPUs: a.gpus, Yield: a.yield}
	env, _ := proto.NewEnvelope(proto.TypeHeartbeat, a.id, "relay", hb)
	a.write(env)
}

// setYield 更新让位状态并立即心跳（模拟用户开始/停止使用机器）。
func (a *mockAgent) setYield(level string, budget int) {
	a.mu.Lock()
	a.yield = proto.YieldState{Level: level, Budget: budget}
	a.mu.Unlock()
	a.sendHeartbeat()
	time.Sleep(50 * time.Millisecond) // 等 Relay 处理
}

// write 串行化写 WS。
func (a *mockAgent) write(env proto.Envelope) {
	data, _ := json.Marshal(env)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = a.conn.Write(ctx, websocket.MessageText, data)
}

// readLoop 读 Relay 下发的消息，处理任务。
func (a *mockAgent) readLoop() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, data, err := a.conn.Read(ctx)
		cancel()
		if err != nil {
			return
		}
		var env proto.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.Type == proto.TypeTaskRequest {
			var task proto.TaskRequest
			env.Decode(&task)
			a.mu.Lock()
			a.receivedTasks = append(a.receivedTasks, task)
			handler := a.taskHandler
			a.mu.Unlock()
			// 执行并回流
			result := handler(task)
			result.TaskID = task.TaskID
			respEnv, _ := proto.NewEnvelope(proto.TypeTaskResult, a.id, "relay", result)
			a.write(respEnv)
		}
	}
}

// close 关闭连接。
func (a *mockAgent) close() {
	a.closeOnce.Do(func() {
		_ = a.conn.Close(websocket.StatusNormalClosure, "test done")
	})
}

// setTaskHandler 设置自定义任务处理函数。
func (a *mockAgent) setTaskHandler(fn func(task proto.TaskRequest) proto.TaskResult) {
	a.mu.Lock()
	a.taskHandler = fn
	a.mu.Unlock()
}

// taskCount 返回收到的任务数。
func (a *mockAgent) taskCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.receivedTasks)
}

// === HTTP 客户端辅助 ===

// apiGet 发 GET 请求返回 body。
func (c *testCluster) apiGet(path string) ([]byte, error) {
	resp, err := c.server.Client().Get(c.httpURL() + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

// apiPostJSON 发 POST JSON 请求。
func (c *testCluster) apiPostJSON(path string, body any) ([]byte, error) {
	payload, _ := json.Marshal(body)
	return c.apiPost(path, string(payload))
}

// apiPost 发 POST 请求。
func (c *testCluster) apiPost(path, body string) ([]byte, error) {
	resp, err := c.server.Client().Post(c.httpURL()+path, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

// listAgents 解析 /api/agents 响应。
func (c *testCluster) listAgents() []AgentView {
	data, err := c.apiGet("/api/agents")
	if err != nil {
		c.t.Fatalf("listAgents 失败: %v", err)
	}
	var resp struct {
		Agents []AgentView `json:"agents"`
		Count  int         `json:"count"`
	}
	json.Unmarshal(data, &resp)
	return resp.Agents
}

// chat 发推理请求。
func (c *testCluster) chat(model, msg string) (proto.OpenAIChatResponse, error) {
	data, err := c.apiPostJSON("/v1/chat/completions", map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": msg}},
	})
	var resp proto.OpenAIChatResponse
	if err != nil {
		return resp, err
	}
	err = json.Unmarshal(data, &resp)
	return resp, err
}

// waitFor 等待条件满足（轮询，超时失败）。
func waitFor(t *testing.T, desc string, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("等待超时（%v）: %s", timeout, desc)
}

// failMsg 格式化失败消息。
func failMsg(format string, args ...any) string { return fmt.Sprintf(format, args...) }
