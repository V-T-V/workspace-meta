package pipeline

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// pipelineFile 是 YAML 文件的结构（用 pipeline_yaml 标签）。
type pipelineFile struct {
	Name  string     `yaml:"name"`
	Steps []stepYAML `yaml:"steps"`
}

type stepYAML struct {
	ID         string          `yaml:"id"`
	Type       StepKind        `yaml:"type"`      // source / transform / sink
	Connector  string          `yaml:"connector"` // csv / filter / sqlite ...
	Config     map[string]any  `yaml:"config"`
	DependsOn  []string        `yaml:"depends_on"`
	Retry      int             `yaml:"retry"`       // 失败重试次数（默认 0）
	DeadLetter *deadLetterYAML `yaml:"dead_letter"` // 死信处理
}

type deadLetterYAML struct {
	Connector string         `yaml:"connector"`
	Config    map[string]any `yaml:"config"`
}

// LoadFromFile 从 YAML 文件加载管道定义。
//
// YAML 示例：
//
//	name: demo
//	steps:
//	  - id: read
//	    type: source
//	    connector: csv
//	    config: { path: "in.csv" }
//	  - id: clean
//	    type: transform
//	    connector: filter
//	    config: { where: "amount > 0" }
//	    depends_on: [read]
//	  - id: write
//	    type: sink
//	    connector: sqlite
//	    config: { path: "out.db", table: "records" }
//	    depends_on: [clean]
func LoadFromFile(path string) (*Pipeline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取管道文件失败 %s: %w", path, err)
	}
	return Parse(raw)
}

// Parse 从 YAML 字节解析管道定义。
func Parse(raw []byte) (*Pipeline, error) {
	raw = expandEnv(raw)
	var pf pipelineFile
	if err := yaml.Unmarshal(raw, &pf); err != nil {
		return nil, fmt.Errorf("解析 YAML 失败: %w", err)
	}
	if pf.Name == "" {
		return nil, fmt.Errorf("管道缺少 name 字段")
	}
	steps := make([]Step, 0, len(pf.Steps))
	for _, sy := range pf.Steps {
		if sy.ID == "" {
			return nil, fmt.Errorf("步骤缺少 id 字段")
		}
		if sy.Type == "" {
			return nil, fmt.Errorf("步骤 %q 缺少 type 字段", sy.ID)
		}
		if sy.Connector == "" {
			return nil, fmt.Errorf("步骤 %q 缺少 connector 字段", sy.ID)
		}
		s := Step{
			ID: sy.ID, Kind: sy.Type, Connector: sy.Connector,
			Config: sy.Config, DependsOn: sy.DependsOn,
			Retry: sy.Retry,
		}
		if sy.DeadLetter != nil {
			s.DeadLetter = &DeadLetterConfig{
				Connector: sy.DeadLetter.Connector,
				Config:    sy.DeadLetter.Config,
			}
		}
		steps = append(steps, s)
	}
	return &Pipeline{Name: pf.Name, Steps: steps}, nil
}

// expandEnv 把 ${VAR} 替换为 os.Getenv("VAR") 的值。
// 未设置的环境变量替换为空字符串。
func expandEnv(raw []byte) []byte {
	s := string(raw)
	var out []byte
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			end := -1
			for j := i + 2; j < len(s); j++ {
				if s[j] == '}' {
					end = j
					break
				}
			}
			if end > 0 {
				name := s[i+2 : end]
				out = append(out, []byte(os.Getenv(name))...)
				i = end + 1
				continue
			}
		}
		out = append(out, s[i])
		i++
	}
	return out
}
