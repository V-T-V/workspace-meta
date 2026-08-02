package pipeline

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// StepKind 标识一个步骤是 source/transform/sink。
type StepKind string

const (
	KindSource    StepKind = "source"
	KindTransform StepKind = "transform"
	KindSink      StepKind = "sink"
)

// Step 是管道定义里的一步（对应 YAML 里的一个 steps[] 条目）。
// JSON 标签让 ToJSON 输出 snake_case（与 YAML 风格一致）。
type Step struct {
	ID        string         `json:"id"`                   // 步骤唯一 ID（如 "read" / "clean" / "write"）
	Kind      StepKind       `json:"kind"`                 // source / transform / sink
	Connector string         `json:"connector"`            // 连接器类型名（如 "csv" / "filter" / "sqlite"）
	Config    map[string]any `json:"config,omitempty"`     // 连接器配置
	DependsOn []string       `json:"depends_on,omitempty"` // 前置步骤 ID（定义 DAG 边）

	// Retry 失败时的重试次数（默认 0=不重试）。仅对临时性错误有用（如网络抖动）。
	Retry int `json:"retry,omitempty"`
	// DeadLetter 失败后的死信处理：若配置了，步骤失败时把输入行写入此 sink 连接器，
	// 而非让整个管道失败。config 同 sink 的 config。
	// 形如 {connector: "csv", config: {path: "dead.csv"}}。仅对 transform/sink 有意义。
	DeadLetter *DeadLetterConfig `json:"dead_letter,omitempty"`

	// When 是步骤执行条件（YAML when:）。为空则总是执行；
	// 非空时由 runner 做简单求值（仅支持 {{stepID.rows_out}} OP number 形式，
	// OP ∈ {>、>=、<、<=、==、!=}），条件不满足则该步 Skipped=true 跳过。
	When string `json:"when,omitempty"`
}

// DeadLetterConfig 死信处理配置。
type DeadLetterConfig struct {
	Connector string         `json:"connector"`        // sink 连接器名（如 "csv" / "sqlite"）
	Config    map[string]any `json:"config,omitempty"` // sink 配置
}

// Pipeline 是一个完整的数据管道（一组 Step 组成的 DAG）。
type Pipeline struct {
	Name  string `json:"Name"`
	Steps []Step `json:"Steps"`
}

// TopoSort 返回步骤的拓扑序（依赖在前）。
// 检测循环依赖，有环则报错。
func (p Pipeline) TopoSort() ([]string, error) {
	// 建邻接表 + 入度
	stepByID := map[string]Step{}
	inDegree := map[string]int{}
	dependents := map[string][]string{} // step → 依赖它的步骤列表
	for _, s := range p.Steps {
		stepByID[s.ID] = s
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
	}
	for _, s := range p.Steps {
		for _, dep := range s.DependsOn {
			if _, ok := stepByID[dep]; !ok {
				return nil, fmt.Errorf("步骤 %q 依赖不存在的步骤 %q", s.ID, dep)
			}
			inDegree[s.ID]++
			dependents[dep] = append(dependents[dep], s.ID)
		}
	}

	// Kahn 算法：入度 0 的先入队
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue) // 确定性：同层按 ID 字典序

	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		next := append([]string{}, dependents[cur]...)
		sort.Strings(next)
		for _, n := range next {
			inDegree[n]--
			if inDegree[n] == 0 {
				queue = append(queue, n)
			}
		}
	}
	if len(order) != len(p.Steps) {
		return nil, fmt.Errorf("管道存在循环依赖（已排序 %d/%d）", len(order), len(p.Steps))
	}
	return order, nil
}

// Validate 校验管道结构合法：source 无依赖、sink 至少一个、连接器类型已注册。
func (p Pipeline) Validate() error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("管道 %q 无步骤", p.Name)
	}
	hasSource, hasSink := false, false
	seen := map[string]bool{}
	for _, s := range p.Steps {
		if seen[s.ID] {
			return fmt.Errorf("重复的步骤 ID: %q", s.ID)
		}
		seen[s.ID] = true
		switch s.Kind {
		case KindSource:
			hasSource = true
			if len(s.DependsOn) > 0 {
				return fmt.Errorf("source 步骤 %q 不应有 depends_on", s.ID)
			}
		case KindSink:
			hasSink = true
			if len(s.DependsOn) == 0 {
				return fmt.Errorf("sink 步骤 %q 必须有 depends_on", s.ID)
			}
		case KindTransform:
			if len(s.DependsOn) == 0 {
				return fmt.Errorf("transform 步骤 %q 必须有 depends_on", s.ID)
			}
		default:
			return fmt.Errorf("步骤 %q 未知 kind %q", s.ID, s.Kind)
		}
	}
	if !hasSource {
		return fmt.Errorf("管道 %q 无 source 步骤", p.Name)
	}
	if !hasSink {
		return fmt.Errorf("管道 %q 无 sink 步骤", p.Name)
	}
	if _, err := p.TopoSort(); err != nil {
		return err
	}
	return nil
}

// String 返回管道的可读描述（步骤列表）。
func (p Pipeline) String() string {
	parts := []string{fmt.Sprintf("Pipeline %q (%d steps):", p.Name, len(p.Steps))}
	for _, s := range p.Steps {
		deps := ""
		if len(s.DependsOn) > 0 {
			deps = " ← " + strings.Join(s.DependsOn, ",")
		}
		parts = append(parts, fmt.Sprintf("  [%s] %s/%s%s", s.Kind, s.ID, s.Connector, deps))
	}
	return strings.Join(parts, "\n")
}

// ToJSON 把管道定义序列化为 JSON（便于 API 传输/存储）。
// 用 encoding/json 序列化 Pipeline（Name + Steps 数组），输出带缩进的可读 JSON。
// Steps 各字段的 JSON key 用 snake_case：
//
//	{
//	  "Name": "csv-to-sqlite",
//	  "Steps": [
//	    {
//	      "id": "read",
//	      "kind": "source",
//	      "connector": "csv",
//	      "config": {...},
//	      "depends_on": ["..."],
//	      "retry": 0,
//	      "dead_letter": null
//	    }
//	  ]
//	}
//
// 步骤为空时输出 "Steps": []（而非 null），便于消费端稳定处理。
func (p Pipeline) ToJSON() (string, error) {
	// 兜底：保证 nil Steps 序列化为空数组而非 null（消费端更友好）。
	if p.Steps == nil {
		p.Steps = []Step{}
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化管道 %q 失败: %w", p.Name, err)
	}
	return string(b), nil
}
