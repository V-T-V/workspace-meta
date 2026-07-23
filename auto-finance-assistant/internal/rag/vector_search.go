package rag

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/QiuShichang/auto-finance-assistant/internal/modelclient"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// VectorSearcher 用内存向量索引做语义检索。
type VectorSearcher struct {
	db     *sql.DB
	client modelclient.ModelClient
	index  *VectorIndex
	limit  int
	log    *slog.Logger
}

// NewVectorSearcher 构造。
func NewVectorSearcher(db *sql.DB, client modelclient.ModelClient, limit int, log *slog.Logger) *VectorSearcher {
	if limit <= 0 {
		limit = 20
	}
	return &VectorSearcher{
		db: db, client: client, index: NewVectorIndex(), limit: limit, log: log,
	}
}

// Index 返回底层索引（供加载/更新）。
func (v *VectorSearcher) Index() *VectorIndex { return v.index }

// Search 生成查询向量并在内存索引上检索。
func (v *VectorSearcher) Search(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	if v.index.Size() == 0 {
		return nil, nil
	}
	vecs, err := v.client.Embed(ctx, []string{q.Text}, 1)
	if err != nil {
		return nil, fmt.Errorf("[rag] 生成查询向量失败: %w", err)
	}
	if len(vecs) == 0 {
		return nil, nil
	}

	matches := v.index.Search(vecs[0], v.limit)
	if len(matches) == 0 {
		return nil, nil
	}

	var ids []string
	for _, m := range matches {
		ids = append(ids, m.ChunkID)
	}
	details, err := v.fetchChunkDetails(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]SearchResult, 0, len(matches))
	for _, m := range matches {
		d, ok := details[m.ChunkID]
		if !ok {
			continue
		}
		out = append(out, SearchResult{
			ChunkID: m.ChunkID, DocumentID: d.documentID,
			Title: d.title, Section: d.section, Content: d.content,
			PageNumber: d.page, Score: float64(m.Score),
			DocumentName: d.docName, Version: d.version,
			Institution: d.institution, ProductCode: d.productCode,
			Region: d.region, CustomerType: d.customerType,
			EffectiveDate: d.effectiveDate, ExpiryDate: d.expiryDate,
		})
	}
	return out, nil
}

type chunkDetail struct {
	documentID                                      string
	title, section, content                         string
	page                                            int
	docName, version, institution, productCode      string
	region, customerType, effectiveDate, expiryDate string
}

func (v *VectorSearcher) fetchChunkDetails(ctx context.Context, ids []string) (map[string]chunkDetail, error) {
	placeholders := ""
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	q := `
		SELECT c.id, c.document_id, COALESCE(c.title,''), COALESCE(c.section,''), c.content,
		       COALESCE(c.page_number,0),
		       d.name, COALESCE(d.version,''), COALESCE(d.institution,''),
		       COALESCE(d.product_code,''), COALESCE(d.region,''), COALESCE(d.customer_type,''),
		       COALESCE(d.effective_date,''), COALESCE(d.expiry_date,'')
		FROM chunks c JOIN documents d ON d.id = c.document_id
		WHERE c.id IN (` + placeholders + `) AND d.status = 'active'
	`
	rows, err := v.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[string]chunkDetail{}
	for rows.Next() {
		var id string
		var d chunkDetail
		if err := rows.Scan(&id, &d.documentID, &d.title, &d.section, &d.content,
			&d.page, &d.docName, &d.version, &d.institution, &d.productCode,
			&d.region, &d.customerType, &d.effectiveDate, &d.expiryDate); err != nil {
			return nil, err
		}
		m[id] = d
	}
	return m, rows.Err()
}

// EmbedAndStore 对文档的所有片段批量生成向量并写回 SQLite。
// 对应原计划 13.2 第二阶段。
func (v *VectorSearcher) EmbedAndStore(ctx context.Context, documentID string, batchSize int) (int, error) {
	chunks, err := storage.ListChunksByDocument(ctx, v.db, documentID)
	if err != nil {
		return 0, err
	}
	if len(chunks) == 0 {
		return 0, nil
	}

	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}

	vecs, err := v.client.Embed(ctx, texts, batchSize)
	if err != nil {
		return 0, err
	}

	count := 0
	for i, c := range chunks {
		if i >= len(vecs) {
			break
		}
		blob := EncodeVector(vecs[i])
		if _, err := v.db.ExecContext(ctx,
			`UPDATE chunks SET embedding=? WHERE id=?`, blob, c.ID); err != nil {
			v.log.Error("[rag] 写入向量失败", "chunkId", c.ID, "err", err)
			continue
		}
		// 增量加入内存索引
		v.index.Add([]VectorEntry{{
			ChunkID: c.ID, Vector: vecs[i], DocumentID: documentID,
		}})
		count++
	}
	v.log.Info("[rag] 文档向量化完成", "docId", documentID, "count", count)
	return count, nil
}

// LoadFromDB 启动时从 SQLite 加载所有 active 文档的向量到内存索引。
func (v *VectorSearcher) LoadFromDB(ctx context.Context) (int, error) {
	rows, err := v.db.QueryContext(ctx, `
		SELECT c.id, c.document_id, c.embedding
		FROM chunks c JOIN documents d ON d.id = c.document_id
		WHERE d.status = 'active' AND c.embedding IS NOT NULL
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var entries []VectorEntry
	for rows.Next() {
		var id, docID string
		var blob []byte
		if err := rows.Scan(&id, &docID, &blob); err != nil {
			continue
		}
		vec, err := DecodeVector(blob)
		if err != nil {
			continue
		}
		entries = append(entries, VectorEntry{ChunkID: id, Vector: vec, DocumentID: docID})
	}
	v.index.Clear()
	v.index.Add(entries)
	v.log.Info("[rag] 向量索引已加载", "count", len(entries))
	return len(entries), rows.Err()
}

// RemoveDocument 从索引移除文档（文档停用时）。
func (v *VectorSearcher) RemoveDocument(documentID string) {
	v.index.RemoveByDocument(documentID)
}
