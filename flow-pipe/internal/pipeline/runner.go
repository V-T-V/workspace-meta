package pipeline

import (
	"fmt"
	"time"
)

// StepResult 是单步执行结果。
type StepResult struct {
	StepID       string
	Kind         StepKind
	RowsIn       int // 该步骤读入的行数（source 为 0）
	RowsOut      int // 该步骤输出的行数（sink 为 RowsIn）
	DeadLettered int // 写入死信的行数（0 表示无死信）
	Duration     time.Duration
	Err          error
	// Skipped 表示该步骤被跳过（状态恢复时跳过已成功完成的步骤）。
	// 被跳过的步骤不执行连接器，其输出用空 Rows 代替（partial rerun），
	// 因此下游步骤拿到的输入可能为空——这是中间数据无法从持久化恢复的折中。
	Skipped bool
}

// RunResult 是整个管道的运行结果。
type RunResult struct {
	PipelineName string
	StartedAt    time.Time
	FinishedAt   time.Time
	Steps        []StepResult
	Err          error
}

// Run 按拓扑序执行管道。每个步骤的输入是它所有依赖步骤输出的合并。
// source 步骤从连接器读数据；transform 转换；sink 写出。
//
// 数据流模型（简化）：每个步骤输出 Rows 暂存，下游步骤把所有依赖的输出拼接成输入。
// 这适合典型的 ETL（source → transform → sink 线性链或分支）。
func Run(p Pipeline) *RunResult {
	return RunWithOptions(p)
}

// RunOption 是 Run 的可选参数（函数式选项）。
type RunOption func(*runConfig)

// runConfig 持有 RunWithOptions 的可调参数。
type runConfig struct {
	skipSteps map[string]bool // 跳过这些步骤（状态恢复：已成功完成的步骤）
	// 未来可加：timeout / concurrency 等
}

// WithSkipSteps 返回一个 RunOption，跳过指定步骤（状态恢复用）。
// 被跳过的步骤不执行连接器，输出用空 Rows 代替。
func WithSkipSteps(stepIDs []string) RunOption {
	return func(c *runConfig) {
		for _, id := range stepIDs {
			c.skipSteps[id] = true
		}
	}
}

// RunWithOptions 按选项执行管道（支持状态恢复）。
// 不传任何 opt 时等价于 Run。
func RunWithOptions(p Pipeline, opts ...RunOption) *RunResult {
	cfg := &runConfig{skipSteps: map[string]bool{}}
	for _, opt := range opts {
		opt(cfg)
	}

	result := &RunResult{PipelineName: p.Name, StartedAt: time.Now()}
	defer func() { result.FinishedAt = time.Now() }()

	if err := p.Validate(); err != nil {
		result.Err = err
		return result
	}
	order, err := p.TopoSort()
	if err != nil {
		result.Err = err
		return result
	}

	stepByID := map[string]Step{}
	for _, s := range p.Steps {
		stepByID[s.ID] = s
	}
	// 每步的输出暂存（供下游拼接）
	outputs := map[string]Rows{}

	for _, id := range order {
		s := stepByID[id]
		start := time.Now()
		sr := StepResult{StepID: id, Kind: s.Kind}

		// 状态恢复：跳过已成功完成的步骤。被跳过的步骤不执行连接器，
		// 其输出用空 Rows 代替（partial rerun）。下游拿到的输入相应为空。
		if cfg.skipSteps[id] {
			sr.Skipped = true
			sr.Duration = time.Since(start)
			result.Steps = append(result.Steps, sr)
			continue
		}

		// 收集依赖的输出作为输入（transform/sink 用）。
		// 合并语义：concat（按 DependsOn 声明顺序拼接）。
		// 注意：每个依赖的输出做 copy 而非共享底层数组，避免下游 append
		// 触发扩容时意外改写共享数据。
		var input Rows
		for _, dep := range s.DependsOn {
			src := outputs[dep]
			cp := make(Rows, len(src))
			copy(cp, src)
			input = append(input, cp...)
		}
		sr.RowsIn = len(input)

		// failStep 把当前步骤标记为失败：若有死信配置则写死信（尽力而为，死信失败不阻塞），
		// 然后根据是否继续返回。返回 true 表示已处理（死信写成功或无死信则整体失败）。
		failStep := func(err error, inputForDead Rows) bool {
			sr.Err = err
			if s.DeadLetter != nil {
				// 尝试写死信。死信 sink 不存在或写失败只记录日志，不让管道因此崩。
				if dlConn, ok := GetSink(s.DeadLetter.Connector); ok {
					if derr := dlConn.Write(inputForDead, s.DeadLetter.Config); derr == nil {
						// 死信写成功：本步视为"已兜底处理"，不把 error 上抛，管道继续。
						sr.Err = nil
						sr.DeadLettered = len(inputForDead)
						return false
					}
				}
			}
			// 无死信或死信失败：整体失败。
			result.Steps = append(result.Steps, sr)
			result.Err = err
			return true
		}

		switch s.Kind {
		case KindSource:
			conn, ok := GetSource(s.Connector)
			if !ok {
				if failStep(fmt.Errorf("未知 source 连接器 %q", s.Connector), nil) {
					return result
				}
				break
			}
			out, err := withRetry(s.Retry, func() (Rows, error) { return conn.Read(s.Config) })
			if err != nil {
				if failStep(fmt.Errorf("source %s 失败: %w", id, err), nil) {
					return result
				}
				break
			}
			outputs[id] = out
			sr.RowsOut = len(out)
		case KindTransform:
			conn, ok := GetTransform(s.Connector)
			if !ok {
				if failStep(fmt.Errorf("未知 transform 连接器 %q", s.Connector), input) {
					return result
				}
				break
			}
			out, err := withRetryTransform(s.Retry, conn, input, s.Config)
			if err != nil {
				if failStep(fmt.Errorf("transform %s 失败: %w", id, err), input) {
					return result
				}
				break
			}
			outputs[id] = out
			sr.RowsOut = len(out)
		case KindSink:
			conn, ok := GetSink(s.Connector)
			if !ok {
				if failStep(fmt.Errorf("未知 sink 连接器 %q", s.Connector), input) {
					return result
				}
				break
			}
			if err := withRetrySink(s.Retry, conn, input, s.Config); err != nil {
				if failStep(fmt.Errorf("sink %s 失败: %w", id, err), input) {
					return result
				}
				break
			}
			sr.RowsOut = sr.RowsIn
		}
		sr.Duration = time.Since(start)
		result.Steps = append(result.Steps, sr)
	}
	return result
}

// Summary 返回运行结果的简短摘要。
func (r RunResult) Summary() string {
	if r.Err != nil {
		return fmt.Sprintf("管道 %s 失败: %v", r.PipelineName, r.Err)
	}
	// 统计死信
	dead := 0
	// 统计跳过（状态恢复）
	skipped := 0
	for _, s := range r.Steps {
		dead += s.DeadLettered
		if s.Skipped {
			skipped++
		}
	}
	suffix := ""
	if dead > 0 {
		suffix += fmt.Sprintf(", 死信 %d 行", dead)
	}
	if skipped > 0 {
		suffix += fmt.Sprintf(", 跳过 %d 步（状态恢复）", skipped)
	}
	return fmt.Sprintf("管道 %s 完成: %d 步, 耗时 %s%s", r.PipelineName, len(r.Steps), r.FinishedAt.Sub(r.StartedAt), suffix)
}

// withRetry 对 source.Read 的重试包装。retries=0 表示不重试（只跑一次）。
// 每次重试间无 backoff（教学/轻量场景；生产可加指数退避）。
func withRetry(retries int, fn func() (Rows, error)) (Rows, error) {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		out, err := fn()
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// withRetryTransform 对 transform.Transform 的重试包装。
func withRetryTransform(retries int, conn TransformConnector, rows Rows, cfg map[string]any) (Rows, error) {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		out, err := conn.Transform(rows, cfg)
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// withRetrySink 对 sink.Write 的重试包装。
func withRetrySink(retries int, conn SinkConnector, rows Rows, cfg map[string]any) error {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if err := conn.Write(rows, cfg); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}
