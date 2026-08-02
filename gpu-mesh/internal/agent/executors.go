package agent

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/engine"
	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// === Phase 2 推理执行器 ===

// execInference 执行 LLM 对话推理（支持流式与非流式）。
func (a *Agent) execInference(ctx context.Context, task proto.TaskRequest) proto.TaskResult {
	var t proto.InferenceTask
	if err := jsonUnmarshal(task.Payload, &t); err != nil {
		return proto.TaskResult{Success: false, Error: "解析 inference 载荷失败: " + err.Error()}
	}
	eng := a.FindEngine(t.Engine)
	if eng == nil {
		return proto.TaskResult{Success: false, Error: "无可用引擎 (请求 engine=" + t.Engine + ")"}
	}

	// 流式推理：逐 token 经 TaskProgress 回传（Step="delta"）
	if t.Stream {
		// 需要从 ctx 拿到 conn 才能发 progress——这里通过 inferenceCtx 关联
		// 由于 execInference 签名无 conn，流式走 execInferenceStream（在 executeTask 里识别 Stream 字段分流）
		// 此处兜底用非流式
	}

	resp, err := eng.Chat(ctx, engine.ChatRequest{
		Model:     t.Model,
		Messages:  t.Messages,
		Options:   t.Options,
		MaxTokens: t.MaxTokens,
	})
	if err != nil {
		return proto.TaskResult{Success: false, Error: "推理失败: " + err.Error()}
	}
	return proto.TaskResult{
		Success:  true,
		ExitCode: 0,
		Data: proto.MarshalData(proto.InferenceResult{
			Content:          resp.Content,
			Model:            resp.Model,
			DoneReason:       resp.DoneReason,
			PromptTokens:     resp.PromptTokens,
			CompletionTokens: resp.CompletionTokens,
		}),
	}
}

// execInferenceStream 流式推理：逐 token 通过 TaskProgress 推回 Relay。
//
// 约定：TaskProgress.Step="delta" 时，Message 字段为本次增量文本。
// 完成时正常回 TaskResult（含完整内容 + usage）。
func (a *Agent) execInferenceStream(ctx context.Context, task proto.TaskRequest, conn *wsConn) proto.TaskResult {
	var t proto.InferenceTask
	if err := jsonUnmarshal(task.Payload, &t); err != nil {
		return proto.TaskResult{Success: false, Error: "解析 inference 载荷失败: " + err.Error()}
	}
	eng := a.FindEngine(t.Engine)
	if eng == nil {
		return proto.TaskResult{Success: false, Error: "无可用引擎"}
	}
	resp, err := eng.ChatStream(ctx, engine.ChatRequest{
		Model: t.Model, Messages: t.Messages, Options: t.Options, MaxTokens: t.MaxTokens,
	}, func(delta string) {
		a.reportProgress(ctx, conn, task.TaskID, "delta", delta, 0)
	})
	if err != nil {
		return proto.TaskResult{Success: false, Error: "流式推理失败: " + err.Error()}
	}
	return proto.TaskResult{
		Success: true,
		Data: proto.MarshalData(proto.InferenceResult{
			Content: resp.Content, Model: resp.Model, DoneReason: resp.DoneReason,
			PromptTokens: resp.PromptTokens, CompletionTokens: resp.CompletionTokens,
		}),
	}
}

// execEmbed 批量文本嵌入。
func (a *Agent) execEmbed(ctx context.Context, task proto.TaskRequest) proto.TaskResult {
	var t proto.EmbedTask
	if err := jsonUnmarshal(task.Payload, &t); err != nil {
		return proto.TaskResult{Success: false, Error: "解析 embed 载荷失败: " + err.Error()}
	}
	eng := a.FindEngine(t.Engine)
	if eng == nil {
		return proto.TaskResult{Success: false, Error: "无可用引擎"}
	}
	resp, err := eng.Embed(ctx, engine.EmbedRequest{Model: t.Model, Input: t.Input})
	if err != nil {
		return proto.TaskResult{Success: false, Error: "嵌入失败: " + err.Error()}
	}
	return proto.TaskResult{
		Success:  true,
		ExitCode: 0,
		Data: proto.MarshalData(proto.EmbedResult{
			Embeddings: resp.Embeddings,
			Model:      resp.Model,
		}),
	}
}

// execPull 拉取/加载模型。
func (a *Agent) execPull(ctx context.Context, task proto.TaskRequest) proto.TaskResult {
	var t proto.PullTask
	if err := jsonUnmarshal(task.Payload, &t); err != nil {
		return proto.TaskResult{Success: false, Error: "解析 pull 载荷失败: " + err.Error()}
	}
	eng := a.FindEngine(t.Engine)
	if eng == nil {
		return proto.TaskResult{Success: false, Error: "无可用引擎 " + t.Engine}
	}
	if err := eng.Pull(ctx, t.Model); err != nil {
		if err == engine.ErrUnsupported {
			return proto.TaskResult{Success: false, Error: t.Engine + " 不支持远程拉取模型，请预设 GGUF 文件"}
		}
		return proto.TaskResult{Success: false, Error: "拉取失败: " + err.Error()}
	}
	log.Printf("[agent] 模型 %s 拉取/加载完成 (engine=%s)", t.Model, t.Engine)
	return proto.TaskResult{Success: true, ExitCode: 0}
}

// === Phase 1 诊断执行器 ===

// execDiag 执行诊断命令（Phase 1 验证链路用）。
func (a *Agent) execDiag(ctx context.Context, task proto.TaskRequest) proto.TaskResult {
	var diag proto.DiagTask
	if err := jsonUnmarshal(task.Payload, &diag); err != nil {
		return proto.TaskResult{Success: false, Error: "解析 diag 载荷失败: " + err.Error()}
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", diag.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", diag.Command)
	}
	start := time.Now()
	out, err := cmd.CombinedOutput()
	result := proto.TaskResult{
		Stdout:   string(out),
		Duration: int(time.Since(start).Milliseconds()),
	}
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}
	return result
}

// === Phase 4 批量离线推理执行器 ===

// execBatch 执行一个分片：逐条调 engine.Chat/Embed，进度上报，容错。
//
// 让位协作：本执行器在 tasks.go 已经过 MinBudget 预检（Phase 3），
// 若执行中途 Agent 进 BUSY，下一批会被外层调度重投到其他节点。
func (a *Agent) execBatch(ctx context.Context, task proto.TaskRequest, conn *wsConn) proto.TaskResult {
	var t proto.BatchTask
	if err := jsonUnmarshal(task.Payload, &t); err != nil {
		return proto.TaskResult{Success: false, Error: "解析 batch 载荷失败: " + err.Error()}
	}
	eng := a.FindEngine(t.Engine)
	if eng == nil {
		return proto.TaskResult{Success: false, Error: "无可用引擎"}
	}

	result := proto.BatchShardResult{
		BatchID: t.BatchID,
		ShardID: t.ShardID,
	}
	total := len(t.Items)

	switch t.TaskType {
	case "chat":
		result.Results = make([]string, 0, total)
		for i, item := range t.Items {
			// 检查 ctx（超时/取消）
			if ctx.Err() != nil {
				result.Failed = total - i
				result.Errors = append(result.Errors, "上下文取消: "+ctx.Err().Error())
				break
			}
			resp, err := eng.Chat(ctx, engine.ChatRequest{
				Model:     t.Model,
				Messages:  []proto.ChatMessage{{Role: "user", Content: item}},
				Options:   t.Options,
				MaxTokens: t.MaxTokens,
			})
			if err != nil {
				result.Results = append(result.Results, "")
				result.Failed++
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.Results = append(result.Results, resp.Content)
				result.Succeeded++
			}
			// 进度上报（每 5 条或最后一条）
			if i%5 == 0 || i == total-1 {
				a.reportProgress(ctx, conn, task.TaskID, fmt.Sprintf("shard %s", t.ShardID),
					fmt.Sprintf("已处理 %d/%d", i+1, total), int(float64(i+1)/float64(total)*100))
			}
		}

	case "embed":
		// 嵌入一次性批量调（engine 支持数组输入）
		model := t.EmbedModel
		if model == "" {
			model = t.Model
		}
		resp, err := eng.Embed(ctx, engine.EmbedRequest{Model: model, Input: t.Items})
		if err != nil {
			return proto.TaskResult{Success: false, Error: "批量嵌入失败: " + err.Error()}
		}
		result.Embeddings = resp.Embeddings
		result.Succeeded = len(resp.Embeddings)

	default:
		return proto.TaskResult{Success: false, Error: "未知 batch task_type: " + t.TaskType}
	}

	log.Printf("[agent] 批量分片 %s 完成: 成功 %d 失败 %d", t.ShardID, result.Succeeded, result.Failed)
	return proto.TaskResult{
		Success:  result.Failed == 0 || result.Succeeded > 0, // 部分成功也算成功
		ExitCode: 0,
		Data:     proto.MarshalData(result),
	}
}

// reportProgress 上报任务进度（长任务用）。
func (a *Agent) reportProgress(ctx context.Context, conn *wsConn, taskID, step, msg string, percent int) {
	pg := proto.TaskProgress{TaskID: taskID, Step: step, Message: msg, Percent: percent}
	env, err := proto.NewEnvelope(proto.TypeTaskProgress, a.cfg.AgentID, "relay", pg)
	if err != nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_ = conn.writeJSON(writeCtx, env)
}

// execTrain Phase 5：执行 LoRA/QLoRA 微调。
//
// 实现策略：生成训练 Python 脚本（unsloth/peft），用子进程执行，
// 流式读 stdout 解析 loss/step 上报进度，遇让位（BUSY）存 checkpoint 暂停。
//
// 8GB 显存约束：默认 QLoRA 4bit，小 batch。
// 让位协作：训练期间定期检查 yield，进 BUSY 时优雅停止训练进程并存档。
func (a *Agent) execTrain(ctx context.Context, task proto.TaskRequest, conn *wsConn) proto.TaskResult {
	var t proto.TrainTask
	if err := jsonUnmarshal(task.Payload, &t); err != nil {
		return proto.TaskResult{Success: false, Error: "解析 train 载荷失败: " + err.Error()}
	}
	applyTrainDefaults(&t)

	// 生成训练脚本路径
	scriptPath, err := writeTrainScript(&t)
	if err != nil {
		return proto.TaskResult{Success: false, Error: "生成训练脚本失败: " + err.Error()}
	}

	// 构造训练命令（python 执行）
	cmd := exec.CommandContext(ctx, "python", scriptPath)
	start := time.Now()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return proto.TaskResult{Success: false, Error: "创建 stdout 管道失败: " + err.Error()}
	}
	cmd.Stderr = cmd.Stdout // 合并 stderr 到 stdout
	if err := cmd.Start(); err != nil {
		return proto.TaskResult{Success: false, Error: "启动训练失败: " + err.Error() + "（确认 python 在 PATH）"}
	}
	log.Printf("[agent] 训练 %s 启动: framework=%s base=%s", t.JobID, t.Framework, t.BaseModel)

	// 流式读输出，解析 loss/step，上报进度，检测让位
	result := parseTrainOutput(ctx, a, conn, task.TaskID, bufio.NewReader(stdout), &t)
	_ = cmd.Wait()

	result.Duration = int(time.Since(start).Seconds())
	result.JobID = t.JobID
	result.OutputDir = t.OutputDir
	if result.Paused {
		log.Printf("[agent] 训练 %s 因让位暂停，checkpoint=%s", t.JobID, result.CheckpointDir)
	}
	return proto.TaskResult{
		Success:  !result.Paused, // 暂停视为未完成（Relay 据此续训）
		ExitCode: 0,
		Data:     proto.MarshalData(result),
		Error: func() string {
			if result.Paused {
				return "train_paused_for_yield"
			}
			return ""
		}(),
	}
}
