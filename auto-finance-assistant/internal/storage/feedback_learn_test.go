package storage

// 第七轮：用户反馈学习闭环测试（storage 层）。
// 覆盖统计汇总、纠正候选提取、提升为 FAQ、重复提升防护。

import (
	"context"
	"strconv"
	"testing"
)

func TestComputeFeedbackStats_Empty(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	stats, err := ComputeFeedbackStats(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 0 {
		t.Errorf("空库 total 应 0，实际 %d", stats.Total)
	}
	if stats.Satisfaction != 0 {
		t.Errorf("空库 satisfaction 应 0，实际 %v", stats.Satisfaction)
	}
}

func TestComputeFeedbackStats_Distribution(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	// 3 赞 1 踩（带纠正）
	CreateFeedback(ctx, db, &Feedback{ID: "p1", Rating: 1})
	CreateFeedback(ctx, db, &Feedback{ID: "p2", Rating: 1})
	CreateFeedback(ctx, db, &Feedback{ID: "p3", Rating: 1})
	CreateFeedback(ctx, db, &Feedback{ID: "n1", Rating: -1, Correction: "应该这样答"})

	stats, err := ComputeFeedbackStats(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 4 {
		t.Errorf("total 应 4，实际 %d", stats.Total)
	}
	if stats.Positive != 3 {
		t.Errorf("positive 应 3，实际 %d", stats.Positive)
	}
	if stats.Negative != 1 {
		t.Errorf("negative 应 1，实际 %d", stats.Negative)
	}
	if stats.WithCorrection != 1 {
		t.Errorf("withCorrection 应 1，实际 %d", stats.WithCorrection)
	}
	if stats.Satisfaction != 0.75 {
		t.Errorf("satisfaction 应 0.75，实际 %v", stats.Satisfaction)
	}
}

func TestListCorrectionCandidates_OnlyNegativeWithCorrection(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	// 正面反馈（不应出现）
	CreateFeedback(ctx, db, &Feedback{ID: "p1", Rating: 1, Correction: "好评"})
	// 负面无纠正（不应出现）
	CreateFeedback(ctx, db, &Feedback{ID: "n1", Rating: -1})
	// 负面带纠正（应出现）
	CreateFeedback(ctx, db, &Feedback{ID: "n2", Rating: -1, Correction: "答错了，正确是5%"})

	items, err := ListCorrectionCandidates(ctx, db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("应只 1 个候选，实际 %d", len(items))
	}
	if items[0].FeedbackID != "n2" {
		t.Errorf("应为 n2，实际 %s", items[0].FeedbackID)
	}
	if items[0].Correction != "答错了，正确是5%" {
		t.Errorf("correction 不匹配，实际 %s", items[0].Correction)
	}
}

func TestListCorrectionCandidates_LimitClamping(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	for i := 0; i < 3; i++ {
		CreateFeedback(ctx, db, &Feedback{ID: "n" + strconv.Itoa(i), Rating: -1, Correction: "纠"})
	}
	items, _ := ListCorrectionCandidates(ctx, db, 0)
	if len(items) != 3 {
		t.Errorf("limit=0 应 3，实际 %d", len(items))
	}
	items, _ = ListCorrectionCandidates(ctx, db, 9999)
	if len(items) != 3 {
		t.Errorf("超大 limit 应 3，实际 %d", len(items))
	}
	items, _ = ListCorrectionCandidates(ctx, db, 2)
	if len(items) != 2 {
		t.Errorf("limit=2 应 2，实际 %d", len(items))
	}
}

func TestPromoteCorrectionToFAQ_Success(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	CreateFeedback(ctx, db, &Feedback{ID: "n1", Rating: -1, Correction: "正确利率4.5%"})

	faqID, err := PromoteCorrectionToFAQ(ctx, db, "n1", "利率是多少", "利率是4.5%")
	if err != nil {
		t.Fatal(err)
	}
	if faqID == "" {
		t.Error("faqID 不应为空")
	}
	// 验证 FAQ 已创建且启用
	faq, err := GetFAQ(ctx, db, faqID)
	if err != nil {
		t.Fatalf("查询提升的 FAQ 失败: %v", err)
	}
	if !faq.Enabled {
		t.Error("提升的 FAQ 应启用")
	}
	if faq.Answer != "利率是4.5%" {
		t.Errorf("answer 不匹配，实际 %s", faq.Answer)
	}
	if faq.Category != "反馈学习" {
		t.Errorf("category 应为 反馈学习，实际 %s", faq.Category)
	}
}

func TestPromoteCorrectionToFAQ_AlreadyPromoted(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	CreateFeedback(ctx, db, &Feedback{ID: "n1", Rating: -1, Correction: "纠"})

	_, _ = PromoteCorrectionToFAQ(ctx, db, "n1", "Q", "A")
	// 重复提升应报错
	_, err := PromoteCorrectionToFAQ(ctx, db, "n1", "Q2", "A2")
	if err != ErrAlreadyPromoted {
		t.Errorf("重复提升应返回 ErrAlreadyPromoted，实际 %v", err)
	}
}

func TestPromoteCorrectionToFAQ_EmptyQA(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	if _, err := PromoteCorrectionToFAQ(ctx, db, "f1", "", "A"); err == nil {
		t.Error("空 question 应报错")
	}
	if _, err := PromoteCorrectionToFAQ(ctx, db, "f1", "Q", ""); err == nil {
		t.Error("空 answer 应报错")
	}
}

// 验证提升后候选标记为 promoted
func TestPromoteCorrectionToFAQ_MarksPromoted(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	CreateFeedback(ctx, db, &Feedback{ID: "n1", Rating: -1, Correction: "纠"})

	items, _ := ListCorrectionCandidates(ctx, db, 10)
	if items[0].Promoted {
		t.Error("未提升前 promoted 应 false")
	}
	PromoteCorrectionToFAQ(ctx, db, "n1", "Q", "A")
	items, _ = ListCorrectionCandidates(ctx, db, 10)
	if !items[0].Promoted {
		t.Error("提升后 promoted 应 true")
	}
}
