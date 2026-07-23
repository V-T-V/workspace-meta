package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// FTSSearcher 用 SQLite FTS5 做全文检索。
// 只返回 active 文档的片段，过滤过期文档。
type FTSSearcher struct {
	db    *sql.DB
	limit int
}

// NewFTSSearcher 构造。limit 为 FTS 取条数上限。
func NewFTSSearcher(db *sql.DB, limit int) *FTSSearcher {
	if limit <= 0 {
		limit = 20
	}
	return &FTSSearcher{db: db, limit: limit}
}

// Search 执行 FTS5 检索，关联 documents 过滤 active 且未过期。
// 策略：提取 ≥3 字符的关键词用 trigram FTS MATCH（OR 连接）；
// 若用户问题过短导致无 FTS 命中，回退 LIKE 兜底。
func (f *FTSSearcher) Search(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	limit := q.FTSLimit
	if limit <= 0 {
		limit = f.limit
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 提取关键词（≥3 字符，trigram 友好）
	keywords := extractKeywords(q.Text)
	ftsQuery := joinFTSKeywords(keywords)

	now := time.Now().Format("2006-01-02")
	var rows *sql.Rows
	var err error

	if ftsQuery != "" {
		rows, err = f.db.QueryContext(queryCtx, `
			SELECT c.id, c.document_id, COALESCE(c.title,''), COALESCE(c.section,''), c.content,
			       COALESCE(c.page_number,0), bm25(chunks_fts) AS score,
			       d.name, COALESCE(d.version,''), COALESCE(d.institution,''),
			       COALESCE(d.product_code,''), COALESCE(d.region,''), COALESCE(d.customer_type,''),
			       COALESCE(d.effective_date,''), COALESCE(d.expiry_date,'')
			FROM chunks_fts f
			JOIN chunks c ON c.id = f.chunk_id
			JOIN documents d ON d.id = c.document_id
			WHERE chunks_fts MATCH ? AND d.status = 'active'
			ORDER BY score
			LIMIT ?
		`, ftsQuery, limit)
	} else {
		// 关键词都太短，用 LIKE 兜底（取用户问题前 20 字符做子串匹配）
		like := truncateForLike(q.Text, 20)
		if like == "" {
			return nil, nil
		}
		rows, err = f.db.QueryContext(queryCtx, `
			SELECT c.id, c.document_id, COALESCE(c.title,''), COALESCE(c.section,''), c.content,
			       COALESCE(c.page_number,0), 0.5 AS score,
			       d.name, COALESCE(d.version,''), COALESCE(d.institution,''),
			       COALESCE(d.product_code,''), COALESCE(d.region,''), COALESCE(d.customer_type,''),
			       COALESCE(d.effective_date,''), COALESCE(d.expiry_date,'')
			FROM chunks c
			JOIN documents d ON d.id = c.document_id
			WHERE c.content LIKE ? AND d.status = 'active'
			LIMIT ?
		`, "%"+like+"%", limit)
	}
	if err != nil {
		return nil, fmt.Errorf("[rag] 检索查询失败: %w", err)
	}
	defer rows.Close()

	var out []SearchResult
	for rows.Next() {
		r := SearchResult{}
		var effectiveRaw, expiryRaw string
		var bm25 float64
		if err := rows.Scan(&r.ChunkID, &r.DocumentID, &r.Title, &r.Section, &r.Content,
			&r.PageNumber, &bm25, &r.DocumentName, &r.Version, &r.Institution,
			&r.ProductCode, &r.Region, &r.CustomerType, &effectiveRaw, &expiryRaw); err != nil {
			return nil, err
		}
		r.EffectiveDate = effectiveRaw
		r.ExpiryDate = expiryRaw
		if expiryRaw != "" && expiryRaw < now {
			continue
		}
		r.Score = bm25ToScore(bm25, r.Content, q.Text)
		out = append(out, r)
	}
	return out, rows.Err()
}

// extractKeywords 从用户问题提取 ≥3 字符的关键词。
// 中文：滑窗取 3-4 字短语；英文：取完整单词。
func extractKeywords(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	var keywords []string
	seen := map[string]bool{}

	// 中文滑窗：取所有 3 字连续子串（去重）
	for i := 0; i+3 <= len(runes); i++ {
		if isCJK(runes[i]) && isCJK(runes[i+1]) && isCJK(runes[i+2]) {
			tri := string(runes[i : i+3])
			if !seen[tri] {
				seen[tri] = true
				keywords = append(keywords, tri)
			}
		}
	}
	// 限制关键词数量（避免 FTS 查询过长）
	if len(keywords) > 8 {
		// 均匀采样 8 个
		step := len(keywords) / 8
		sampled := make([]string, 0, 8)
		for i := 0; i < len(keywords); i += step {
			sampled = append(sampled, keywords[i])
			if len(sampled) >= 8 {
				break
			}
		}
		keywords = sampled
	}
	return keywords
}

// joinFTSKeywords 把关键词用 OR 连接为 FTS5 MATCH 表达式。
func joinFTSKeywords(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	// 用 OR 连接，每个 token 用引号包裹（trigram 下裸字符串也行，但引号更安全）
	parts := make([]string, len(keywords))
	for i, k := range keywords {
		parts[i] = "\"" + strings.ReplaceAll(k, "\"", "\"\"") + "\""
	}
	return strings.Join(parts, " OR ")
}

// truncateForLike 截取用于 LIKE 的子串（避免 % 通配注入）。
func truncateForLike(s string, maxRunes int) string {
	runes := []rune(s)
	// 去除 % _ 等通配符
	var clean []rune
	for _, r := range runes {
		if r == '%' || r == '_' {
			continue
		}
		clean = append(clean, r)
		if len(clean) >= maxRunes {
			break
		}
	}
	return string(clean)
}

// buildFTSQuery 把自然语言问题转为 FTS5 MATCH 表达式。
// 策略：提取关键词，用 OR 连接（召回优先）。
// FTS5 中 OR 用空格分隔，AND 用空格但默认行为依赖配置。
func buildFTSQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// 去标点
	var cleaned strings.Builder
	for _, r := range text {
		if isCJK(r) || isAlnum(r) || r == ' ' {
			cleaned.WriteRune(r)
		}
	}
	// 按空格分词（中文无空格，整体作为一个 token；英文按空格）
	tokens := strings.Fields(cleaned.String())
	// 中文整体也作为一个查询项
	if isMostlyCJK(text) {
		tokens = append([]string{text}, tokens...)
	}
	if len(tokens) == 0 {
		return ""
	}
	// 去重 + 用 OR 连接（FTS5 用空格表示 OR 当默认连接符为 OR）
	seen := map[string]bool{}
	var parts []string
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		// 含特殊字符的 token 用双引号包裹
		if needsQuoting(t) {
			parts = append(parts, "\""+strings.ReplaceAll(t, "\"", "\"\"")+"\"")
		} else {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " OR ")
}

// bm25ToScore 把相关度转为 0~1 正分。
// 统一用查询 3-gram 在内容中的命中率估算（对 FTS 与 LIKE 都适用，稳定可靠）。
// bm25 仅在命中时作微调。
func bm25ToScore(bm25 float64, content, query string) float64 {
	if content == "" || query == "" {
		return 0.3
	}
	runes := []rune(query)
	if len(runes) < 3 {
		return 0.3
	}
	hits := 0
	total := 0
	for i := 0; i+3 <= len(runes); i++ {
		if isCJK(runes[i]) && isCJK(runes[i+1]) && isCJK(runes[i+2]) {
			total++
			if strings.Contains(content, string(runes[i:i+3])) {
				hits++
			}
		}
	}
	if total == 0 {
		return 0.3
	}
	overlap := float64(hits) / float64(total)
	// 基础分 0.4 + overlap × 0.55 = 0.4~0.95
	score := 0.4 + 0.55*overlap
	// bm25 命中（负值）微调：越负越相关，加一点
	if bm25 < 0 {
		boost := -bm25 / 20.0
		if boost > 0.1 {
			boost = 0.1
		}
		score += boost
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

func isAlnum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isMostlyCJK(s string) bool {
	cjk := 0
	total := 0
	for _, r := range s {
		if r == ' ' {
			continue
		}
		total++
		if isCJK(r) {
			cjk++
		}
	}
	return total > 0 && cjk*2 >= total
}

func needsQuoting(s string) bool {
	for _, r := range s {
		if r == '"' || r == '\'' || r == ':' || r == '*' || r == '(' || r == ')' {
			return true
		}
	}
	return false
}
