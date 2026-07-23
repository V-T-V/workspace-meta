package rag

import (
	"context"
	"log/slog"
)

// HybridRetriever 融合 FTS 与向量检索。
// 对应原计划 11.5 第二阶段：FTS + 向量 + 结果融合。
type HybridRetriever struct {
	fts         *FTSSearcher
	vector      *VectorSearcher
	vectorWeight float64
	keywordWeight float64
	enabled     bool // 向量索引为空时降级为纯 FTS
	log         *slog.Logger
}

// NewHybridRetriever 构造。
func NewHybridRetriever(fts *FTSSearcher, vec *VectorSearcher, vecW, kwW float64, log *slog.Logger) *HybridRetriever {
	return &HybridRetriever{
		fts: fts, vector: vec,
		vectorWeight: vecW, keywordWeight: kwW,
		log: log,
	}
}

// Search 并行执行 FTS + 向量检索，加权融合后返回 Top-K。
func (h *HybridRetriever) Search(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	// 向量索引为空时纯 FTS
	if h.vector == nil || h.vector.Index().Size() == 0 {
		return h.fts.Search(ctx, q)
	}

	// 同步执行（简化；CPU 下向量检索较慢，可接受）
	ftsResults, err := h.fts.Search(ctx, q)
	if err != nil {
		h.log.Warn("[rag] FTS 检索失败，仅用向量", "err", err)
		ftsResults = nil
	}
	vecResults, err := h.vector.Search(ctx, q)
	if err != nil {
		h.log.Warn("[rag] 向量检索失败，仅用 FTS", "err", err)
		vecResults = nil
	}

	return fuseResults(ftsResults, vecResults, h.keywordWeight, h.vectorWeight), nil
}

// fuseResults 融合 FTS 与向量结果。
// 策略：同一 chunk 若被两路都命中，分数叠加（上限 1.0），体现"双路确认"高置信。
// 向量分数做提升（cosine 0.4+ 对中文语义匹配已不错）。
func fuseResults(fts, vec []SearchResult, kwWeight, vecWeight float64) []SearchResult {
	merged := map[string]*SearchResult{}

	add := func(results []SearchResult, weight float64, isVector bool) {
		for _, r := range results {
			rawScore := r.Score
			if isVector {
				// 向量分数提升：cosine 0.3~0.7 → 映射到 0.5~0.95
				rawScore = 0.4 + r.Score*0.55
				if rawScore > 0.95 {
					rawScore = 0.95
				}
			}
			contrib := rawScore * weight
			existing, ok := merged[r.ChunkID]
			if !ok {
				rc := r
				rc.Score = contrib
				merged[r.ChunkID] = &rc
			} else {
				// 双路命中：叠加（上限 1.0）
				existing.Score += contrib
				if existing.Score > 1.0 {
					existing.Score = 1.0
				}
				if existing.Content == "" {
					existing.Content = r.Content
				}
			}
		}
	}

	add(fts, kwWeight, false)
	add(vec, vecWeight, true)

	out := make([]SearchResult, 0, len(merged))
	for _, r := range merged {
		out = append(out, *r)
	}
	// 按分数降序
	for i := 0; i < len(out); i++ {
		maxIdx := i
		for j := i + 1; j < len(out); j++ {
			if out[j].Score > out[maxIdx].Score {
				maxIdx = j
			}
		}
		out[i], out[maxIdx] = out[maxIdx], out[i]
	}
	return out
}
