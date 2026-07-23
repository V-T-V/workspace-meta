package rag

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
)

// VectorIndex 是内存只读向量索引。
// 启动时从 SQLite 加载，文档发布后增量更新。
type VectorIndex struct {
	mu      sync.RWMutex
	entries []vectorEntry
	dim     int
}

type vectorEntry struct {
	ChunkID    string
	Vector     []float32
	DocumentID string
}

// NewVectorIndex 构造空索引。
func NewVectorIndex() *VectorIndex {
	return &VectorIndex{}
}

// Add 批量加入向量（文档发布时）。
func (v *VectorIndex) Add(entries []VectorEntry) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, e := range entries {
		if v.dim == 0 && len(e.Vector) > 0 {
			v.dim = len(e.Vector)
		}
		v.entries = append(v.entries, vectorEntry{
			ChunkID: e.ChunkID, Vector: e.Vector, DocumentID: e.DocumentID,
		})
	}
}

// VectorEntry 是加入索引的一条记录。
type VectorEntry struct {
	ChunkID    string
	Vector     []float32
	DocumentID string
}

// RemoveByDocument 删除某文档的所有向量（文档停用时）。
func (v *VectorIndex) RemoveByDocument(docID string) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	filtered := v.entries[:0]
	removed := 0
	for _, e := range v.entries {
		if e.DocumentID == docID {
			removed++
			continue
		}
		filtered = append(filtered, e)
	}
	v.entries = filtered
	return removed
}

// Clear 清空索引。
func (v *VectorIndex) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.entries = nil
	v.dim = 0
}

// Size 返回向量数。
func (v *VectorIndex) Size() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.entries)
}

// Search 用余弦相似度检索 Top-K。
// 返回 (chunkID, score) 列表。
func (v *VectorIndex) Search(query []float32, topK int) []VectorMatch {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.entries) == 0 || len(query) == 0 {
		return nil
	}
	if topK <= 0 || topK > len(v.entries) {
		topK = len(v.entries)
	}

	// 计算所有相似度
	matches := make([]VectorMatch, 0, len(v.entries))
	queryNorm := norm(query)
	if queryNorm == 0 {
		return nil
	}
	for _, e := range v.entries {
		score := cosine(e.Vector, query, queryNorm)
		matches = append(matches, VectorMatch{ChunkID: e.ChunkID, Score: score})
	}

	// 部分排序取 Top-K
	partitionTopK(matches, topK)
	return matches[:topK]
}

// VectorMatch 向量检索结果。
type VectorMatch struct {
	ChunkID string
	Score   float64
}

// partitionTopK 简单选择 Top-K（对中小规模数据足够）。
func partitionTopK(matches []VectorMatch, k int) {
	for i := 0; i < k && i < len(matches); i++ {
		maxIdx := i
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[maxIdx].Score {
				maxIdx = j
			}
		}
		matches[i], matches[maxIdx] = matches[maxIdx], matches[i]
	}
}

// cosine 余弦相似度。
func cosine(a, b []float32, bNorm float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	aNorm := norm(a)
	if aNorm == 0 {
		return 0
	}
	return dot / (aNorm * bNorm)
}

// norm 向量模长。
func norm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

// EncodeVector 把 []float32 编码为 BLOB（little-endian）。
func EncodeVector(v []float32) []byte {
	buf := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// DecodeVector 把 BLOB 解码为 []float32。
func DecodeVector(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("向量 BLOB 长度 %d 不是 4 的倍数", len(buf))
	}
	n := len(buf) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return v, nil
}
