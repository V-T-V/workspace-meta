package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/QiuShichang/auto-finance-assistant/internal/config"
)

// verifyModelHash 校验模型权重文件的 SHA-256 哈希。
// 如果 config 中配置了 model_hash，启动时自动校验，防止模型被篡改。
// 未配置 model_hash 时跳过（仅告警）。
func verifyModelHash(cfg *config.Config) error {
	expectedHash := cfg.Security.ModelHash
	if expectedHash == "" {
		// 未配置哈希校验，跳过
		return nil
	}

	// 推断模型文件路径
	modelPath := cfg.Ollama.ModelPath
	if modelPath == "" {
		return nil // 非本地文件模式（如 API 调用），跳过
	}

	if _, err := os.Stat(modelPath); err != nil {
		return nil // 文件不存在（可能用 Ollama 内部模型），跳过
	}

	actualHash, err := computeFileHash(modelPath)
	if err != nil {
		return fmt.Errorf("[security] 模型哈希计算失败: %w", err)
	}

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("[security] ⚠ 模型权重哈希不匹配！可能已被篡改\n  期望: %s\n  实际: %s\n  文件: %s",
			expectedHash, actualHash, modelPath)
	}

	return nil
}

// computeFileHash 计算文件 SHA-256 哈希。
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
