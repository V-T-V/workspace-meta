package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// wsConn 封装一条到 Relay 的 WS 连接，串行化写以避免并发写冲突。
type wsConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (w *wsConn) writeJSON(ctx context.Context, env proto.Envelope) error {
	data, err := jsonMarshal(env)
	if err != nil {
		return err
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.conn.Write(ctx, websocket.MessageText, data)
}

// normalizeRelayURL 把裸地址补全为 ws://host:7780/agent。
//
// 输入示例 → 输出：
//   - 192.168.1.100           → ws://192.168.1.100:7780/agent
//   - 192.168.1.100:9000      → ws://192.168.1.100:9000/agent
//   - ws://h:7780             → ws://h:7780/agent   （补 path）
//   - ws://h:7780/agent       → 原样
//   - wss://h/agent           → 原样
func normalizeRelayURL(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "ws://") || strings.HasPrefix(s, "wss://") {
		// 已有 scheme，只补 path（若无）
		u, err := url.Parse(s)
		if err != nil {
			return s
		}
		if u.Path == "" || u.Path == "/" {
			u.Path = "/agent"
			return u.String()
		}
		return s
	}
	// 裸 host 或 host:port
	if !strings.Contains(s, ":") {
		s = s + ":7780"
	}
	return "ws://" + s + "/agent"
}

// appendToken 把鉴权 token 作为查询参数附加。
func appendToken(rawURL, token string) string {
	if token == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}

// dialRelay 连接 Relay，失败返回错误（由调用方做指数退避重连）。
//
// 关键：必须绕过系统 HTTP 代理（HTTP_PROXY/HTTPS_PROXY）。
// 双重防护：
//  1. EarlyInit() 在进程启动时清空代理环境变量（防 envProxyOnce 缓存）
//  2. 此处显式传入无代理的自定义 HTTPClient（绝对不走 DefaultTransport）
//
// 必须用自建 Transport 且 Proxy 字段设为 nil（非 nil 的 func 返回 nil），
// 才能彻底绕开 net/http 的 envProxyOnce 缓存机制。
var noProxyClient = &http.Client{
	Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return nil, nil }, // 显式无代理
		DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	},
}

func dialRelay(ctx context.Context, relayURL, token string) (*wsConn, error) {
	full := appendToken(normalizeRelayURL(relayURL), token)
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(dialCtx, full, &websocket.DialOptions{
		HTTPClient: noProxyClient,
	})
	if err != nil {
		return nil, err
	}
	c.SetReadLimit(64 << 20) // 64MB（Phase 4 大结果回流）
	return &wsConn{conn: c}, nil
}

// EarlyInit Agent 进程最早期初始化：禁用系统代理。
//
// 必须在 main() 第一行、任何 HTTP 调用前执行。
// 原因：net/http 的 envProxyOnce（sync.Once）在首次 HTTP 请求时缓存代理配置，
// 一旦缓存（含系统代理），之后 os.Unsetenv 也无法清除。
// Agent 到 Relay 是直连公网 VPS，不应走系统代理（否则 WS 握手被代理转发失败）。
//
// 用法：cmd/agent/main.go 的 main() 第一行调用 agent.EarlyInit()。
func EarlyInit() {
	disableProxyEnv()
}

// disableProxyEnv 清空本进程的代理环境变量（进程级，不污染系统）。
func disableProxyEnv() {
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
		"http_proxy", "https_proxy", "all_proxy"} {
		if v := os.Getenv(k); v != "" {
			log.Printf("[agent] 清除代理环境变量 %s=%s", k, v)
			_ = os.Unsetenv(k)
		}
	}
}

// reconnectLoop 反向连接循环：带指数退避，连接成功后跑 session，断了重连。
//
// 这是穿透 NAT 的唯一方式：Agent 主动外连 Relay，Relay 不主动连 Agent。
func (a *Agent) reconnectLoop(parentCtx context.Context) {
	backoff := time.Second
	const maxBackoff = 60 * time.Second
	for {
		select {
		case <-parentCtx.Done():
			return
		default:
		}

		conn, err := dialRelay(parentCtx, a.cfg.RelayURL, a.cfg.Token)
		if err != nil {
			log.Printf("[agent] 连接 Relay 失败: %v (%ds 后重试)", err, int(backoff.Seconds()))
			select {
			case <-parentCtx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		// 连上了，重置退避
		backoff = time.Second
		log.Printf("[agent] 已连接 Relay %s", normalizeRelayURL(a.cfg.RelayURL))

		// 跑一个完整 session（注册 → 心跳 → 读任务），直到连接断开
		a.runSession(parentCtx, conn)

		log.Printf("[agent] 连接断开，准备重连")
		select {
		case <-parentCtx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// errSessionEnded session 因连接断开结束。
var errSessionEnded = errors.New("session ended")

// runSession 跑一个完整的会话周期：发注册 → 启动心跳写循环 → 读循环分发消息。
func (a *Agent) runSession(ctx context.Context, conn *wsConn) {
	// 1. 发注册
	if err := a.sendRegister(ctx, conn); err != nil {
		log.Printf("[agent] 注册失败: %v", err)
		_ = conn.conn.Close(websocket.StatusPolicyViolation, "register failed")
		return
	}

	// 2. 心跳写循环（独立 goroutine）
	hbCtx, hbCancel := context.WithCancel(ctx)
	go a.heartbeatLoop(hbCtx, conn)
	defer hbCancel()

	// 3. 读循环（阻塞，直到出错或 ctx 取消）
	a.readLoop(ctx, conn)
}

// readLoop 持续读 Relay 下发的消息并分发。
func (a *Agent) readLoop(ctx context.Context, conn *wsConn) {
	for {
		// 用 context 控制读超时：长连接读不应无限阻塞，定期醒来检查 ctx
		readCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		_, data, err := conn.conn.Read(readCtx)
		cancel()
		if err != nil {
			if ctx.Err() == nil && !isNormalClose(err) {
				log.Printf("[agent] 读循环错误: %v", err)
			}
			return
		}
		var env proto.Envelope
		if err := jsonUnmarshal(data, &env); err != nil {
			log.Printf("[agent] 解析消息失败: %v", err)
			continue
		}
		a.handleMessage(ctx, conn, env)
	}
}

// handleMessage 分发一条入站消息。
func (a *Agent) handleMessage(ctx context.Context, conn *wsConn, env proto.Envelope) {
	switch env.Type {
	case proto.TypeTaskRequest:
		a.handleTaskRequest(ctx, conn, env)
	case proto.TypeTaskCancel:
		a.handleTaskCancel(ctx, conn, env)
	default:
		log.Printf("[agent] 未知消息类型 %s", env.Type)
	}
}

// isNormalClose 判断 WS 错误是否为正常关闭（重连场景常见）。
func isNormalClose(err error) bool {
	if err == nil {
		return true
	}
	status := websocket.CloseStatus(err)
	return status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway
}

// fmt helper（避免 import 循环 + 集中错误格式化）。
var _ = fmt.Sprintf
