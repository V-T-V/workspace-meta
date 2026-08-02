package rag

// 第五轮：RAG 检索质量测试。
// 覆盖 VectorIndex（余弦相似度/TopK/编码）、FTSSearcher（中文召回）、
// HybridRetriever（融合/短路/降级）、ComputeConfidence/Classify、
// 以及召回率/精确率交叉验证。

import (
	"context"
	"database/sql"
	"log/slog"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// silentLog 构造静默 logger。
func silentLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// ===========================================================================
// VectorIndex：余弦相似度与 TopK
// ===========================================================================

func TestVectorIndex_CosineBasic(t *testing.T) {
	idx := NewVectorIndex()
	idx.Add([]VectorEntry{
		{ChunkID: "a", Vector: []float32{1, 0, 0}},
		{ChunkID: "b", Vector: []float32{0, 1, 0}},
		{ChunkID: "c", Vector: []float32{1, 1, 0}}, // 与 query 夹角更小
	})
	// query 与 c 最相似（归一化后 cosine=0.707，与 a/b 相同但 c 更接近 45度）
	matches := idx.Search([]float32{1, 1, 0}, 1)
	if len(matches) != 1 {
		t.Fatalf("应返回 1 条，实际 %d", len(matches))
	}
	if matches[0].ChunkID != "c" {
		t.Errorf("最相似应为 c（完全相同），实际 %s", matches[0].ChunkID)
	}
	if math.Abs(matches[0].Score-1.0) > 0.001 {
		t.Errorf("完全相同向量 cosine 应 1.0，实际 %v", matches[0].Score)
	}
}

func TestVectorIndex_TopKOrdering(t *testing.T) {
	idx := NewVectorIndex()
	// 用不同方向使余弦相似度有区分度（query=[1,1,0]）
	idx.Add([]VectorEntry{
		{ChunkID: "far", Vector: []float32{0, 0, 1}},   // 与 query 正交 → cosine≈0
		{ChunkID: "mid", Vector: []float32{1, 0, 0}},    // cosine≈0.707
		{ChunkID: "near", Vector: []float32{0.9, 0.9, 0}}, // cosine≈1.0
	})
	matches := idx.Search([]float32{1, 1, 0}, 3)
	if len(matches) != 3 {
		t.Fatalf("应返回 3 条，实际 %d", len(matches))
	}
	// 应按相似度降序：near > mid > far
	if matches[0].ChunkID != "near" || matches[1].ChunkID != "mid" || matches[2].ChunkID != "far" {
		names := []string{matches[0].ChunkID, matches[1].ChunkID, matches[2].ChunkID}
		t.Errorf("排序应 near>mid>far，实际 %v", names)
	}
	// 分数应递减
	for i := 1; i < len(matches); i++ {
		if matches[i].Score > matches[i-1].Score {
			t.Error("分数应递减")
		}
	}
}

func TestVectorIndex_TopKLargerThanSize(t *testing.T) {
	idx := NewVectorIndex()
	idx.Add([]VectorEntry{{ChunkID: "a", Vector: []float32{1}}})
	// topK > 条目数：返回全部
	matches := idx.Search([]float32{1}, 10)
	if len(matches) != 1 {
		t.Errorf("topK 超量应返回全部 1 条，实际 %d", len(matches))
	}
}

func TestVectorIndex_Empty(t *testing.T) {
	idx := NewVectorIndex()
	if matches := idx.Search([]float32{1}, 5); matches != nil {
		t.Errorf("空索引应返回 nil，实际 %v", matches)
	}
	if idx.Size() != 0 {
		t.Errorf("空索引 Size 应 0")
	}
}

func TestVectorIndex_ZeroQueryReturnsNil(t *testing.T) {
	idx := NewVectorIndex()
	idx.Add([]VectorEntry{{ChunkID: "a", Vector: []float32{1, 0}}})
	if matches := idx.Search([]float32{0, 0}, 5); matches != nil {
		t.Errorf("零向量查询应返回 nil，实际 %v", matches)
	}
}

func TestVectorIndex_RemoveByDocument(t *testing.T) {
	idx := NewVectorIndex()
	idx.Add([]VectorEntry{
		{ChunkID: "c1", Vector: []float32{1}, DocumentID: "doc1"},
		{ChunkID: "c2", Vector: []float32{1}, DocumentID: "doc1"},
		{ChunkID: "c3", Vector: []float32{1}, DocumentID: "doc2"},
	})
	removed := idx.RemoveByDocument("doc1")
	if removed != 2 {
		t.Errorf("应删除 2 条，实际 %d", removed)
	}
	if idx.Size() != 1 {
		t.Errorf("删除后应剩 1 条，实际 %d", idx.Size())
	}
}

func TestVectorIndex_DimensionMismatchReturnsZero(t *testing.T) {
	idx := NewVectorIndex()
	idx.Add([]VectorEntry{{ChunkID: "a", Vector: []float32{1, 2, 3}}})
	// query 维度不同 → cosine 返回 0
	matches := idx.Search([]float32{1, 2}, 5)
	if len(matches) != 1 {
		t.Fatalf("应返回 1 条（含 0 分），实际 %d", len(matches))
	}
	if matches[0].Score != 0 {
		t.Errorf("维度不匹配 cosine 应 0，实际 %v", matches[0].Score)
	}
}

func TestEncodeDecodeVector_RoundTrip(t *testing.T) {
	orig := []float32{1.5, -2.3, 0.0, 3.14159, -0.001}
	blob := EncodeVector(orig)
	if len(blob) != 4*len(orig) {
		t.Errorf("BLOB 长度应 %d，实际 %d", 4*len(orig), len(blob))
	}
	decoded, err := DecodeVector(blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(orig) {
		t.Fatalf("维度应一致 %d，实际 %d", len(orig), len(decoded))
	}
	for i := range orig {
		if math.Abs(float64(decoded[i]-orig[i])) > 1e-6 {
			t.Errorf("第 %d 维 %v != %v", i, decoded[i], orig[i])
		}
	}
}

func TestDecodeVector_InvalidLength(t *testing.T) {
	// 长度非 4 的倍数
	if _, err := DecodeVector([]byte{1, 2, 3}); err == nil {
		t.Error("非法 BLOB 长度应报错")
	}
}

func TestEncodeVector_Empty(t *testing.T) {
	if blob := EncodeVector(nil); len(blob) != 0 {
		t.Errorf("空向量 BLOB 应空，实际 %d", len(blob))
	}
}

// ===========================================================================
// ComputeConfidence / Classify
// ===========================================================================

func TestComputeConfidence_Empty(t *testing.T) {
	if s := ComputeConfidence(nil); s != 0 {
		t.Errorf("空结果置信度应 0，实际 %v", s)
	}
}

func TestComputeConfidence_HighScoreRetrieval(t *testing.T) {
	// 高检索分 + 多结果 + 元数据齐全 → 高置信
	results := []SearchResult{
		{ChunkID: "1", Score: 0.9, ProductCode: "P1", Institution: "工行", Region: "华东",
			EffectiveDate: "2026-01-01"},
		{ChunkID: "2", Score: 0.8},
		{ChunkID: "3", Score: 0.7},
	}
	s := ComputeConfidence(results)
	if s < 0.7 {
		t.Errorf("高检索分+多结果应高置信(>0.7)，实际 %v", s)
	}
	if s > 1.0 {
		t.Errorf("置信度应 <=1.0，实际 %v", s)
	}
}

func TestComputeConfidence_LowScoreRetrieval(t *testing.T) {
	// 低检索分 + 单结果 + 无元数据 → 低置信
	results := []SearchResult{
		{ChunkID: "1", Score: 0.3},
	}
	s := ComputeConfidence(results)
	if s > 0.5 {
		t.Errorf("低检索分应低置信(<0.5)，实际 %v", s)
	}
}

func TestComputeConfidence_MetadataBoost(t *testing.T) {
	// 相同检索分，有元数据 vs 无元数据
	base := []SearchResult{{ChunkID: "1", Score: 0.7}}
	withMeta := []SearchResult{{ChunkID: "1", Score: 0.7, ProductCode: "P", Institution: "I", Region: "R"}}
	sBase := ComputeConfidence(base)
	sMeta := ComputeConfidence(withMeta)
	if sMeta <= sBase {
		t.Errorf("有元数据置信度 %v 应 > 无元数据 %v", sMeta, sBase)
	}
}

func TestComputeConfidence_CoverageByCount(t *testing.T) {
	// 结果数影响覆盖度
	s1 := ComputeConfidence([]SearchResult{{Score: 0.5}})
	s2 := ComputeConfidence([]SearchResult{{Score: 0.5}, {Score: 0.4}})
	s3 := ComputeConfidence([]SearchResult{{Score: 0.5}, {Score: 0.4}, {Score: 0.3}})
	if !(s1 < s2 && s2 < s3) {
		t.Errorf("覆盖度应随结果数递增：%v < %v < %v", s1, s2, s3)
	}
}

func TestClassify_Thresholds(t *testing.T) {
	if Classify(0.9, 0.4, 0.8) != ConfidenceHigh {
		t.Error("0.9 应 High")
	}
	if Classify(0.5, 0.4, 0.8) != ConfidenceMedium {
		t.Error("0.5 应 Medium")
	}
	if Classify(0.3, 0.4, 0.8) != ConfidenceLow {
		t.Error("0.3 应 Low")
	}
	// 边界：等于阈值归上一级
	if Classify(0.8, 0.4, 0.8) != ConfidenceHigh {
		t.Error("0.8（=高阈值）应 High")
	}
	if Classify(0.4, 0.4, 0.8) != ConfidenceMedium {
		t.Error("0.4（=低阈值）应 Medium")
	}
}

// ===========================================================================
// FTSSearcher：中文召回（真实 SQLite FTS5）
// ===========================================================================

// ===========================================================================
// FTSSearcher（真实 DB）
// ===========================================================================

func TestFTSSearcher_Recall(t *testing.T) {
	db := setupFTSDB(t)
	fts := NewFTSSearcher(db, 20)
	ctx := context.Background()
	results, err := fts.Search(ctx, SearchQuery{Text: "贷款利率是多少", FTSLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("贷款利率查询应有召回")
	}
	// 至少一个结果内容含"利率"
	found := false
	for _, r := range results {
		if containsStr(r.Content, "利率") {
			found = true
			break
		}
	}
	if !found {
		t.Error("召回结果应含 利率 相关内容")
	}
	// 分数应在 0~1
	for _, r := range results {
		if r.Score < 0 || r.Score > 1.0 {
			t.Errorf("分数应 0~1，实际 %v", r.Score)
		}
	}
}

func TestFTSSearcher_NoMatch(t *testing.T) {
	db := setupFTSDB(t)
	fts := NewFTSSearcher(db, 20)
	ctx := context.Background()
	// 查完全不相关的内容（zxb 无 3-gram 命中）
	results, err := fts.Search(ctx, SearchQuery{Text: "量子纠缠相对论", FTSLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	// 应无召回或召回很少（无相关内容）
	for _, r := range results {
		// 不应命中利率/材料/还款相关
		if containsStr(r.Content, "利率") || containsStr(r.Content, "材料") {
			t.Errorf("无关查询不应命中相关内容")
		}
	}
}

func TestFTSSearcher_OnlyActiveDocs(t *testing.T) {
	db := setupFTSDB(t)
	fts := NewFTSSearcher(db, 20)
	ctx := context.Background()
	// 查 draft 文档的内容（应被过滤）
	results, err := fts.Search(ctx, SearchQuery{Text: "草稿内容测试", FTSLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if containsStr(r.Content, "草稿内容") {
			t.Errorf("draft 文档不应被召回")
		}
	}
}

func TestFTSSearcher_ExpiredDocFiltered(t *testing.T) {
	db := setupFTSDB(t)
	ctx := context.Background()
	// 直接插入一个过期文档+片段，验证检索过滤
	storage.CreateDocument(ctx, db, &storage.Document{
		ID: "dexp", Name: "过期政策", OriginalName: "x", FileType: ".txt",
		FileSize: 1, FileHash: "hexp", Status: storage.DocStatusActive,
		ExpiryDate: "2020-01-01", // 已过期
	})
	storage.CreateChunk(ctx, db, &storage.Chunk{
		ID: "cexp", DocumentID: "dexp", Sequence: 1,
		Content: "过期政策的贷款利率说明",
	})
	fts := NewFTSSearcher(db, 20)
	results, err := fts.Search(ctx, SearchQuery{Text: "过期政策的贷款利率", FTSLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.DocumentID == "dexp" {
			t.Errorf("过期文档不应被召回")
		}
	}
}

// ===========================================================================
// HybridRetriever：融合与短路
// ===========================================================================

func TestFuseResults_DualPathBoost(t *testing.T) {
	// 同一 chunk 被 FTS 和向量双路命中，分数应叠加
	fts := []SearchResult{{ChunkID: "c1", Content: "x", Score: 0.8}}
	vec := []SearchResult{{ChunkID: "c1", Content: "x", Score: 0.7}}
	merged := fuseResults(fts, vec, 0.5, 0.5)
	if len(merged) != 1 {
		t.Fatalf("融合后应 1 条，实际 %d", len(merged))
	}
	// FTS 贡献 0.8*0.5=0.4，向量贡献 (0.4+0.7*0.55)*0.5≈0.39 → 叠加约 0.79
	if merged[0].Score < 0.5 {
		t.Errorf("双路命中分数应叠加较高，实际 %v", merged[0].Score)
	}
	if merged[0].Score > 1.0 {
		t.Errorf("分数应 <=1.0，实际 %v", merged[0].Score)
	}
}

func TestFuseResults_OrderByScoreDesc(t *testing.T) {
	fts := []SearchResult{
		{ChunkID: "low", Score: 0.3},
		{ChunkID: "high", Score: 0.9},
	}
	merged := fuseResults(fts, nil, 1.0, 0.0)
	if len(merged) != 2 {
		t.Fatalf("应 2 条，实际 %d", len(merged))
	}
	if merged[0].ChunkID != "high" {
		t.Errorf("应按分数降序，首条 high，实际 %s", merged[0].ChunkID)
	}
}

func TestFuseResults_EmptyInputs(t *testing.T) {
	merged := fuseResults(nil, nil, 0.5, 0.5)
	if len(merged) != 0 {
		t.Errorf("空输入应空输出，实际 %d", len(merged))
	}
}

func TestFuseResults_VectorScoreBoost(t *testing.T) {
	// 验证向量分数提升：cosine 0.5 → 映射到 0.4+0.5*0.55=0.675
	vec := []SearchResult{{ChunkID: "c1", Score: 0.5}}
	merged := fuseResults(nil, vec, 0.0, 1.0) // 纯向量
	if len(merged) != 1 {
		t.Fatal("应 1 条")
	}
	expect := 0.4 + 0.5*0.55
	if math.Abs(merged[0].Score-expect) > 0.01 {
		t.Errorf("向量提升分应约 %v，实际 %v", expect, merged[0].Score)
	}
}

// ===========================================================================
// 召回率/精确率交叉验证
// ===========================================================================

func TestRetrieval_RecallPrecision(t *testing.T) {
	// 对一组已知查询，验证相关文档是否被召回（recall@K）
	db := setupFTSDB(t)
	fts := NewFTSSearcher(db, 20)
	ctx := context.Background()

	cases := []struct {
		query         string
		relevantDocID string // 人工标注的相关文档
	}{
		{"贷款利率是多少", "d1"},
		{"申请贷款需要哪些材料", "d2"},
		{"还款方式有哪些", "d3"},
	}
	hits := 0
	for _, c := range cases {
		results, err := fts.Search(ctx, SearchQuery{Text: c.query, FTSLimit: 5})
		if err != nil {
			t.Fatalf("查询 %q 失败: %v", c.query, err)
		}
		recalled := false
		for _, r := range results {
			if r.DocumentID == c.relevantDocID {
				recalled = true
				break
			}
		}
		if recalled {
			hits++
		} else {
			t.Logf("查询 %q 未召回相关文档 %s", c.query, c.relevantDocID)
		}
	}
	recall := float64(hits) / float64(len(cases))
	// 召回率应 >= 0.66（允许个别查询因 trigram 召回不全）
	if recall < 0.66 {
		t.Errorf("召回率 %.2f 过低（应 >=0.66）", recall)
	}
}

func TestRetrieval_FTSvsFusionConsistency(t *testing.T) {
	// 交叉验证：纯 FTS 结果与 Hybrid（向量索引空时）结果一致
	db := setupFTSDB(t)
	log := silentLog()
	fts := NewFTSSearcher(db, 20)
	// 向量索引为空 → Hybrid 退化为纯 FTS
	hybrid := NewHybridRetriever(fts, nil, 0.5, 0.5, log)
	ctx := context.Background()
	q := SearchQuery{Text: "贷款利率", FTSLimit: 5}

	ftsResults, err := fts.Search(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	hybridResults, err := hybrid.Search(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	// 向量索引空时 Hybrid 直接返回 FTS 结果（但经 fuseResults 处理，分数会变）
	// 验证召回的 chunkID 集合一致
	if !sameChunkIDs(ftsResults, hybridResults) {
		t.Errorf("向量索引空时 Hybrid 应与 FTS 召回一致")
	}
}

// ===========================================================================
// BuildContext
// ===========================================================================

func TestBuildContext_IncludesContent(t *testing.T) {
	results := []SearchResult{
		{ChunkID: "1", Content: "第一段证据", Section: "A"},
		{ChunkID: "2", Content: "第二段证据", Section: "B"},
	}
	ctx := BuildContext(results, 5)
	if !containsStr(ctx, "第一段证据") || !containsStr(ctx, "第二段证据") {
		t.Errorf("上下文应包含证据内容，实际 %s", ctx)
	}
}

func TestBuildContext_RespectsLimit(t *testing.T) {
	// limit=1 应只取首条
	results := []SearchResult{
		{ChunkID: "1", Content: "证据一"},
		{ChunkID: "2", Content: "证据二"},
	}
	ctx := BuildContext(results, 1)
	if containsStr(ctx, "证据二") {
		t.Errorf("limit=1 不应含第二条")
	}
	if !containsStr(ctx, "证据一") {
		t.Errorf("应含第一条")
	}
}

func TestBuildContext_Empty(t *testing.T) {
	if ctx := BuildContext(nil, 5); ctx != "" {
		t.Errorf("空结果上下文应空，实际 %q", ctx)
	}
}

// ===========================================================================
// Service 编排
// ===========================================================================

func TestService_Retrieve_FinalLimitTruncation(t *testing.T) {
	// mock retriever 返回 5 条，finalLimit=2 → 截断到 2
	retriever := &mockRetriever{results: []SearchResult{
		{ChunkID: "1", Score: 0.9}, {ChunkID: "2", Score: 0.8},
		{ChunkID: "3", Score: 0.7}, {ChunkID: "4", Score: 0.6},
		{ChunkID: "5", Score: 0.5},
	}}
	svc := NewService(retriever, 0.3, 0.7, 2, silentLog())
	resp, err := svc.Retrieve(context.Background(), SearchQuery{Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("finalLimit=2 应截断到 2，实际 %d", len(resp.Results))
	}
}

func TestService_ShouldAnswer(t *testing.T) {
	retriever := &mockRetriever{}
	svc := NewService(retriever, 0.3, 0.7, 5, silentLog())
	if !svc.ShouldAnswer(ConfidenceHigh) {
		t.Error("High 应可回答")
	}
	if !svc.ShouldAnswer(ConfidenceMedium) {
		t.Error("Medium 应可回答")
	}
	if svc.ShouldAnswer(ConfidenceLow) {
		t.Error("Low 应拒答")
	}
}

func TestService_ThresholdsExposed(t *testing.T) {
	svc := NewService(&mockRetriever{}, 0.3, 0.7, 5, silentLog())
	if svc.MinConfidence() != 0.3 {
		t.Errorf("MinConfidence 应 0.3，实际 %v", svc.MinConfidence())
	}
	if svc.HighConfidence() != 0.7 {
		t.Errorf("HighConfidence 应 0.7，实际 %v", svc.HighConfidence())
	}
}

// ===========================================================================
// 辅助
// ===========================================================================

type mockRetriever struct {
	results []SearchResult
	err     error
}

func (m *mockRetriever) Search(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// setupFTSDB 构造迁移后 DB + 种子文档/片段（真实实现）。
func setupFTSDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := dir + "/rag.db"
	log := silentLog()
	db, err := storage.OpenDB(dbPath, log)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := storage.Migrate(ctx, db, storage.AllActiveVersions(), log); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// 种子：3 个 active 文档
	seeds := []struct {
		docID, name, content string
	}{
		{"d1", "车贷利率政策", "汽车金融贷款利率根据产品而定，新车贷款年利率最低4.5%，二手车贷款利率6.8%起。"},
		{"d2", "申请材料清单", "申请汽车贷款需要以下材料：身份证、收入证明、居住证明、银行流水、车辆购销合同。"},
		{"d3", "还款方式说明", "还款方式包括等额本息和等额本金两种，客户可根据自身情况选择，提前还款可节省利息。"},
	}
	for _, s := range seeds {
		_ = storage.CreateDocument(ctx, db, &storage.Document{
			ID: s.docID, Name: s.name, OriginalName: s.name + ".txt",
			FileType: ".txt", FileSize: int64(len(s.content)), FileHash: "h-" + s.docID,
			Status: storage.DocStatusActive,
		})
		_ = storage.CreateChunk(ctx, db, &storage.Chunk{
			ID: "c-" + s.docID, DocumentID: s.docID, Sequence: 1,
			Content: s.content, Title: s.name,
		})
	}
	// 一个 draft 文档（应被过滤）
	_ = storage.CreateDocument(ctx, db, &storage.Document{
		ID: "ddraft", Name: "草稿", OriginalName: "x", FileType: ".txt",
		FileSize: 1, FileHash: "hd", Status: storage.DocStatusDraft,
	})
	_ = storage.CreateChunk(ctx, db, &storage.Chunk{
		ID: "c-draft", DocumentID: "ddraft", Sequence: 1, Content: "草稿内容测试",
	})
	return db
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func sameChunkIDs(a, b []SearchResult) bool {
	if len(a) != len(b) {
		return false
	}
	sa := make([]string, len(a))
	sb := make([]string, len(b))
	for i, r := range a {
		sa[i] = r.ChunkID
	}
	for i, r := range b {
		sb[i] = r.ChunkID
	}
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
