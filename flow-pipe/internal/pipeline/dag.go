package pipeline

import (
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
type Step struct {
	ID        string         // 步骤唯一 ID（如 "read" / "clean" / "write"）
	Kind      StepKind       // source / transform / sink
	Connector string         // 连接器类型名（如 "csv" / "filter" / "sqlite"）
	Config    map[string]any // 连接器配置
	DependsOn []string       // 前置步骤 ID（定义 DAG 边）

	// Retry 失败时的重试次数（默认 0=不重试）。仅对临时性错误有用（如网络抖动）。
	Retry int
	// DeadLetter 失败后的死信处理：若配置了，步骤失败时把输入行写入此 sink 连接器，
	// 而非让整个管道失败。config 同 sink 的 config。
	// 形如 {connector: "csv", config: {path: "dead.csv"}}。仅对 transform/sink 有意义。
	DeadLetter *DeadLetterConfig
}

// DeadLetterConfig 死信处理配置。
type DeadLetterConfig struct {
	Connector string         // sink 连接器名（如 "csv" / "sqlite"）
	Config    map[string]any // sink 配置
}

// Pipeline 是一个完整的数据管道（一组 Step 组成的 DAG）。
type Pipeline struct {
	Name  string
	Steps []Step
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
