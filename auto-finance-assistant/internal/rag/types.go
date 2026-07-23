// Package rag 实现知识检索：FTS5 全文检索（M4）+ 向量检索（M6）。
// 对应原计划第十三节。
package rag

import "context"

// SearchQuery 是一次检索请求。
type SearchQuery struct {
	Text       string // 用户问题（已脱敏）
	FTSLimit   int    // FTS 取多少条
	FinalLimit int    // 最终保留多少条
}

// SearchResult 是一条检索结果（片段 + 相关度分数 + 所属文档）。
type SearchResult struct {
	ChunkID       string
	DocumentID    string
	Title         string
	Section       string
	Content       string
	PageNumber    int
	Score         float64 // 归一化 0~1
	// 文档元数据（用于来源展示与过滤）
	DocumentName  string
	Version       string
	Institution   string
	ProductCode   string
	Region        string
	CustomerType  string
	EffectiveDate string
	ExpiryDate    string
}

// Retriever 检索接口。对应原计划 25.2。
type Retriever interface {
	Search(ctx context.Context, q SearchQuery) ([]SearchResult, error)
}
