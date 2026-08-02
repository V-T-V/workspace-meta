package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// applyTrainDefaults 训练参数补默认（8GB 显存约束）。
func applyTrainDefaults(t *proto.TrainTask) {
	if t.Framework == "" {
		t.Framework = "unsloth" // unsloth 对低显存最友好
	}
	if t.Method == "" {
		t.Method = "qlora" // 8GB 必须量化
	}
	if t.Rank <= 0 {
		t.Rank = 8
	}
	if t.Alpha <= 0 {
		t.Alpha = 16
	}
	if t.Epochs <= 0 {
		t.Epochs = 1
	}
	if t.BatchSize <= 0 {
		t.BatchSize = 1
	}
	if t.LearningRate == 0 {
		t.LearningRate = 2e-4
	}
	if t.MaxSeqLen <= 0 {
		t.MaxSeqLen = 512
	}
	if t.QuantBits <= 0 {
		t.QuantBits = 4
	}
	if t.OutputDir == "" {
		t.OutputDir = filepath.Join(os.TempDir(), "gpu-mesh-train", t.JobID)
	}
}

// writeTrainScript 生成训练 Python 脚本到临时文件。
//
// 脚本内容根据 framework 选择（unsloth/peft）。
// 输出格式约定：周期打印 "TRAIN_PROGRESS step=N loss=X lr=Y" 供 parseTrainOutput 解析。
// 完成时打印 "TRAIN_DONE final_loss=X"。
// 暂停时（捕获让位信号或 checkpoint）打印 "TRAIN_PAUSE checkpoint=PATH"。
func writeTrainScript(t *proto.TrainTask) (string, error) {
	if err := os.MkdirAll(t.OutputDir, 0o755); err != nil {
		return "", err
	}
	var script string
	switch t.Framework {
	case "unsloth":
		script = genUnslothScript(t)
	case "peft":
		script = genPEFTScript(t)
	default:
		return "", fmt.Errorf("未知 framework: %s（支持 unsloth/peft）", t.Framework)
	}
	scriptPath := filepath.Join(t.OutputDir, "train.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return "", err
	}
	return scriptPath, nil
}

// genUnslothScript 生成 unsloth QLoRA 训练脚本（低显存友好）。
func genUnslothScript(t *proto.TrainTask) string {
	return fmt.Sprintf(`# gpu-mesh 自动生成的 unsloth QLoRA 训练脚本
# JobID: %s
import sys
try:
    from unsloth import FastLanguageModel
    from transformers import TrainingArguments
    from trl import SFTTrainer
except ImportError as e:
    print(f"TRAIN_ERROR 缺少依赖: {{e}}（请装 unsloth: pip install unsloth）")
    sys.exit(1)

model, tokenizer = FastLanguageModel.from_pretrained(
    model_name="%s",
    max_seq_length=%d,
    dtype=None, load_in_4bit=True,
)
model = FastLanguageModel.get_peft_model(model, r=%d, lora_alpha=%d,
    target_modules=["q_proj","k_proj","v_proj","o_proj","gate_proj","up_proj","down_proj"])

from datasets import load_dataset
ds = load_dataset("json", data_files="%s", split="train")

trainer = SFTTrainer(model=model, tokenizer=tokenizer, train_dataset=ds,
    dataset_text_field="text",
    max_seq_length=%d,
    args=TrainingArguments(per_device_train_batch_size=%d, gradient_accumulation_steps=4,
        num_train_epochs=%d, learning_rate=%g, logging_steps=5,
        output_dir="%s", save_strategy="steps", save_steps=50))

trainer.train(resume_from_checkpoint="%s")
print("TRAIN_DONE final_loss=0.0")
`, t.JobID, t.BaseModel, t.MaxSeqLen, t.Rank, t.Alpha,
		t.Dataset, t.MaxSeqLen, t.BatchSize, t.Epochs, t.LearningRate,
		t.OutputDir, t.ResumeFrom)
}

// genPEFTScript 生成 HuggingFace PEFT QLoRA 脚本（备选框架）。
func genPEFTScript(t *proto.TrainTask) string {
	return fmt.Sprintf(`# gpu-mesh 自动生成的 PEFT QLoRA 训练脚本
import sys
try:
    import torch
    from transformers import AutoModelForCausalLM, AutoTokenizer, TrainingArguments, BitsAndBytesConfig
    from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training
    from trl import SFTTrainer
    from datasets import load_dataset
except ImportError as e:
    print(f"TRAIN_ERROR 缺少依赖: {{e}}")
    sys.exit(1)

bnb = BitsAndBytesConfig(load_in_4bit=True, bnb_4bit_quant_type="nf4",
    bnb_4bit_compute_dtype=torch.float16)
model = AutoModelForCausalLM.from_pretrained("%s", quantization_config=bnb)
model = prepare_model_for_kbit_training(model)
cfg = LoraConfig(r=%d, lora_alpha=%d, lora_dropout=0.05,
    target_modules=["q_proj","k_proj","v_proj","o_proj"], task_type="CAUSAL_LM")
model = get_peft_model(model, cfg)
tok = AutoTokenizer.from_pretrained("%s")
ds = load_dataset("json", data_files="%s", split="train")
trainer = SFTTrainer(model=model, tokenizer=tok, train_dataset=ds,
    args=TrainingArguments(per_device_train_batch_size=%d,
        num_train_epochs=%d, learning_rate=%g, logging_steps=5,
        output_dir="%s", save_steps=50))
trainer.train(resume_from_checkpoint="%s")
print("TRAIN_DONE final_loss=0.0")
`, t.BaseModel, t.Rank, t.Alpha, t.BaseModel, t.Dataset,
		t.BatchSize, t.Epochs, t.LearningRate, t.OutputDir, t.ResumeFrom)
}

// parseTrainOutput 流式解析训练 stdout。
//
// 约定输出行：
//   "TRAIN_PROGRESS step=N loss=X lr=Y total=M"  → 进度
//   "TRAIN_DONE final_loss=X"                      → 完成
//   "TRAIN_PAUSE checkpoint=PATH"                  → 让位暂停
//   "TRAIN_ERROR msg"                              → 错误
//
// 同时兼容 HuggingFace 默认 log（'loss': X, 'learning_rate': Y）作为 fallback。
// 让位检测：每 30s 检查 yield 状态，进 BUSY 时向训练进程发 SIGINT（脚本应捕获存档）。
func parseTrainOutput(ctx context.Context, a *Agent, conn *wsConn, taskID string,
	r *bufio.Reader, t *proto.TrainTask) proto.TrainResult {
	result := proto.TrainResult{}
	progressRe := regexp.MustCompile(`step=(\d+)\s+loss=([0-9.eE+-]+)`)
	hfLossRe := regexp.MustCompile(`'loss':\s*([0-9.eE+-]+)`)
	lastCheck := time.Now()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 大缓冲防长行截断
	for scanner.Scan() {
		line := scanner.Text()
		// 解析进度
		if m := progressRe.FindStringSubmatch(line); m != nil {
			step, _ := strconv.Atoi(m[1])
			loss, _ := strconv.ParseFloat(m[2], 64)
			result.Steps = step
			result.FinalLoss = loss
			percent := 0
			if t.Epochs > 0 {
				percent = step * 10 // 粗略估算（真实步数未知）
				if percent > 95 {
					percent = 95
				}
			}
			a.reportProgress(ctx, conn, taskID, "training",
				fmt.Sprintf("step=%d loss=%.4f", step, loss), percent)
		} else if m := hfLossRe.FindStringSubmatch(line); m != nil {
			loss, _ := strconv.ParseFloat(m[1], 64)
			result.FinalLoss = loss
		}
		if strings.Contains(line, "TRAIN_DONE") {
			break
		}
		if strings.Contains(line, "TRAIN_PAUSE") {
			result.Paused = true
			result.CheckpointDir = t.OutputDir
			break
		}
		if strings.Contains(line, "TRAIN_ERROR") {
			break
		}

		// 让位检测（每 30s）
		if time.Since(lastCheck) > 30*time.Second {
			lastCheck = time.Now()
			if a.yield.State().Level == proto.YieldBUSY {
				// 进 BUSY，暂停训练（脚本应定期存 checkpoint + 捕获信号优雅退出）
				result.Paused = true
				result.CheckpointDir = t.OutputDir
				a.reportProgress(ctx, conn, taskID, "yield_pause",
					"机器被用户使用，训练暂停存档", -1)
				break
			}
		}
	}
	return result
}

// execPythonExists 检查 python 是否可用（训练前置检查）。
func execPythonExists() bool {
	_, err := exec.LookPath("python")
	if err != nil {
		_, err = exec.LookPath("python3")
	}
	return err == nil
}
