package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// === Phase 6：审计日志 + 多租户配额 + 告警 ===

// --- 审计日志（append-only JSONL）---

// AuditEntry 审计日志条目。
type AuditEntry struct {
	TS      int64  `json:"ts"`       // Unix 毫秒
	Event   string `json:"event"`    // 事件类型
	AgentID string `json:"agent_id"` // 关联 Agent
	TaskID  string `json:"task_id"`  // 关联任务
	APIKey  string `json:"api_key"`  // 关联 API Key（截断）
	Detail  string `json:"detail"`   // 详情
}

// AuditLogger append-only 审计日志（JSONL 格式）。
type AuditLogger struct {
	mu   sync.Mutex
	file *os.File
}

// NewAuditLogger 打开审计日志文件（append 模式）。
func NewAuditLogger(path string) (*AuditLogger, error) {
	if path == "" {
		path = "audit.jsonl"
	}
	if dir := filepath.Dir(path); dir != "." {
		os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &AuditLogger{file: f}, nil
}

// Log 写一条审计记录。
func (a *AuditLogger) Log(e AuditEntry) {
	if a == nil {
		return
	}
	e.TS = time.Now().UnixMilli()
	a.mu.Lock()
	defer a.mu.Unlock()
	data, _ := json.Marshal(e)
	a.file.Write(append(data, '\n'))
}

// Close 关闭。
func (a *AuditLogger) Close() {
	if a != nil && a.file != nil {
		a.file.Close()
	}
}

// --- 多租户 API Key 配额 ---

// Tenant 一个租户（API Key 对应）。
type Tenant struct {
	APIKey    string `json:"api_key"`
	Name      string `json:"name"`
	// 配额：每分钟最大请求数（RPM），0=不限
	RPM int `json:"rpm"`
	// 配额：每天最大 Token 数，0=不限
	DailyTokens int `json:"daily_tokens"`

	// 运行态（不持久化）
	mu             sync.Mutex
	requestsMinute int        // 当前分钟内请求数
	minuteStart    time.Time  // 当前分钟起点
	tokensToday    int        // 今天已用 token
	dayStart       time.Time  // 今天起点
}

// TenantManager 多租户管理器。
type TenantManager struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant // apiKey → tenant
}

// NewTenantManager 构造。
func NewTenantManager() *TenantManager {
	return &TenantManager{tenants: make(map[string]*Tenant)}
}

// AddTenant 添加租户。
func (m *TenantManager) AddTenant(t *Tenant) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.minuteStart = time.Now()
	t.dayStart = time.Now()
	m.tenants[t.APIKey] = t
}

// Get 查找租户。
func (m *TenantManager) Get(apiKey string) *Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tenants[apiKey]
}

// CheckQuota 检查配额是否允许请求。返回错误表示超限。
func (t *Tenant) CheckQuota() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	// 重置分钟计数
	if now.Sub(t.minuteStart) >= time.Minute {
		t.requestsMinute = 0
		t.minuteStart = now
	}
	// 重置日计数
	if now.Sub(t.dayStart) >= 24*time.Hour {
		t.tokensToday = 0
		t.dayStart = now
	}
	// RPM 检查
	if t.RPM > 0 && t.requestsMinute >= t.RPM {
		return fmt.Errorf("超过每分钟配额 %d", t.RPM)
	}
	t.requestsMinute++
	return nil
}

// ConsumeTokens 扣减 token 配额。
func (t *Tenant) ConsumeTokens(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tokensToday += n
}

// --- 告警 ---

// AlertRule 告警规则。
type AlertRule struct {
	Name    string // 规则名
	Webhook string // 钉钉/飞书/企业微信 webhook URL
}

// AlertManager 告警管理器。
type AlertManager struct {
	mu     sync.Mutex
	rules  []AlertRule
	client *http.Client
	// 去重：同一规则同一消息 5 分钟内只发一次
	lastSent map[string]time.Time
}

// NewAlertManager 构造。
func NewAlertManager() *AlertManager {
	return &AlertManager{
		client:   &http.Client{Timeout: 5 * time.Second},
		lastSent: make(map[string]time.Time),
	}
}

// AddWebhook 添加告警 webhook。
func (a *AlertManager) AddWebhook(name, url string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rules = append(a.rules, AlertRule{Name: name, Webhook: url})
}

// Send 发送告警（去重：同名告警 5 分钟内只发一次）。
func (a *AlertManager) Send(name, message string) {
	a.mu.Lock()
	// 去重
	key := name + ":" + message
	if last, ok := a.lastSent[key]; ok && time.Since(last) < 5*time.Minute {
		a.mu.Unlock()
		return
	}
	a.lastSent[key] = time.Now()
	rules := make([]AlertRule, len(a.rules))
	copy(rules, a.rules)
	a.mu.Unlock()

	for _, r := range rules {
		go a.postWebhook(r, name, message)
	}
}

// postWebhook 发送到一个 webhook（钉钉/飞书通用 JSON 格式）。
func (a *AlertManager) postWebhook(r AlertRule, name, message string) {
	body := map[string]any{
		"text": fmt.Sprintf("[gpu-mesh 告警] %s\n%s\n%s", name, message, time.Now().Format("2006-01-02 15:04:05")),
	}
	payload, _ := json.Marshal(body)
	resp, err := a.client.Post(r.Webhook, "application/json", bytes.NewReader(payload))
	if err != nil {
		return
	}
	resp.Body.Close()
}

// --- 计数器（供 metrics 用）---

var (
	auditEventsTotal  int64
	alertsSentTotal   int64
	quotaBlockedTotal int64
)

// incAudit, incAlert, incQuotaBlocked 原子计数（metrics 暴露用）。
func incAudit()        { atomic.AddInt64(&auditEventsTotal, 1) }
func incAlert()        { atomic.AddInt64(&alertsSentTotal, 1) }
func incQuotaBlocked() { atomic.AddInt64(&quotaBlockedTotal, 1) }
