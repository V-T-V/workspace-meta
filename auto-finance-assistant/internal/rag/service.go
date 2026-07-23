package rag

import (
	"context"
	"log/slog"
)

// Service 编排检索流程：FTS 检索 → 结果过滤 → 置信度计算。
// M6 将增加向量检索与融合。
type Service struct {
	retriever       Retriever
	minConfidence   float64
	highConfidence  float64
	finalLimit      int
	log             *slog.Logger
}

// NewService 构造。
func NewService(retriever Retriever, minConf, highConf float64, finalLimit int, log *slog.Logger) *Service {
	return &Service{
		retriever:      retriever,
		minConfidence:  minConf,
		highConfidence: highConf,
		finalLimit:     finalLimit,
		log:            log,
	}
}

// RetrieveResponse 是一次检索的结果。
type RetrieveResponse struct {
	Results    []SearchResult
	Context    string          // 构造好的证据上下文（供模型）
	Confidence float64         // 整体置信度
	Level      ConfidenceLevel // high | medium | low
}

// Retrieve 执行完整检索流程。
func (s *Service) Retrieve(ctx context.Context, query SearchQuery) (*RetrieveResponse, error) {
	results, err := s.retriever.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	// 截断到 finalLimit
	if s.finalLimit > 0 && len(results) > s.finalLimit {
		results = results[:s.finalLimit]
	}

	confidence := ComputeConfidence(results)
	level := Classify(confidence, s.minConfidence, s.highConfidence)
	context := BuildContext(results, s.finalLimit)

	return &RetrieveResponse{
		Results:    results,
		Context:    context,
		Confidence: confidence,
		Level:      level,
	}, nil
}

// ShouldAnswer 低置信度时拒答。
func (s *Service) ShouldAnswer(level ConfidenceLevel) bool {
	return level != ConfidenceLow
}

// MinConfidence 返回阈值（供外部展示）。
func (s *Service) MinConfidence() float64 { return s.minConfidence }

// HighConfidence 返回阈值。
func (s *Service) HighConfidence() float64 { return s.highConfidence }
