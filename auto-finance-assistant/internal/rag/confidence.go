package rag

// ComputeConfidence 计算检索结果的整体置信度。
// 对应原计划 11.7，但调整为：检索相关性为主，元数据为加分项。
//   最终分数 = 检索分数 × 0.55 + 元数据匹配 × 0.15 + 文档有效性 × 0.15 + 证据覆盖度 × 0.15
// 调整原因：元数据在导入初期常缺失，不应成为拒答主因；检索相关性应主导。
func ComputeConfidence(results []SearchResult) float64 {
	if len(results) == 0 {
		return 0
	}
	top := results[0]
	// 1. 检索分数（取 top1）——主导因素
	retrievalScore := top.Score

	// 2. 元数据匹配度：加分项（有则加，无不影响）
	metadataMatch := 0.0
	if top.ProductCode != "" {
		metadataMatch += 0.4
	}
	if top.Institution != "" {
		metadataMatch += 0.3
	}
	if top.Region != "" {
		metadataMatch += 0.3
	}
	if metadataMatch > 1 {
		metadataMatch = 1
	}

	// 3. 文档有效性
	validity := 1.0
	if top.EffectiveDate == "" {
		validity = 0.9 // 无生效日期轻微降权（不应大幅影响置信度）
	}

	// 4. 证据覆盖度
	coverage := 0.0
	switch len(results) {
	case 0:
		coverage = 0
	case 1:
		coverage = 0.5
	case 2:
		coverage = 0.8
	default:
		coverage = 1.0
	}

	score := retrievalScore*0.55 + metadataMatch*0.15 + validity*0.15 + coverage*0.15
	if score > 1 {
		score = 1
	}
	return score
}

// ConfidenceLevel 把分数映射为置信度等级。
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"   // >= highThreshold
	ConfidenceMedium ConfidenceLevel = "medium" // >= minThreshold
	ConfidenceLow    ConfidenceLevel = "low"    // < minThreshold → 拒答
)

// Classify 按阈值分级。
func Classify(score, minThreshold, highThreshold float64) ConfidenceLevel {
	if score >= highThreshold {
		return ConfidenceHigh
	}
	if score >= minThreshold {
		return ConfidenceMedium
	}
	return ConfidenceLow
}
