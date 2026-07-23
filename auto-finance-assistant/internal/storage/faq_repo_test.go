package storage

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
)

// TestFAQRepo_CRUD 验证 FAQ 增删改查闭环。
func TestFAQRepo_CRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	// FAQ 表在 004_faqs，需要 M2 版本激活
	if err := Migrate(ctx, db, M2ActiveVersions(), log); err != nil {
		t.Fatal(err)
	}

	// Create
	faq := &FAQ{
		ID:                 uuid.NewString(),
		Category:           "政策",
		Question:           "贷款利率是多少？",
		NormalizedQuestion: "贷款利率是多少",
		Answer:             "利率根据产品而定。",
		Keywords:           "贷款 利率",
		Enabled:            true,
		Priority:           10,
	}
	if err := CreateFAQ(ctx, db, faq); err != nil {
		t.Fatalf("创建 FAQ 失败: %v", err)
	}

	// Get
	got, err := GetFAQ(ctx, db, faq.ID)
	if err != nil {
		t.Fatalf("查询 FAQ 失败: %v", err)
	}
	if got.Question != faq.Question {
		t.Errorf("问题不匹配: %s", got.Question)
	}
	if !got.Enabled {
		t.Error("应为启用")
	}

	// 不存在
	if _, err := GetFAQ(ctx, db, "nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在应返回 ErrNotFound，实际 %v", err)
	}

	// Update
	got.Answer = "更新后的答案"
	got.Enabled = false
	if err := UpdateFAQ(ctx, db, got); err != nil {
		t.Fatalf("更新 FAQ 失败: %v", err)
	}
	updated, _ := GetFAQ(ctx, db, faq.ID)
	if updated.Answer != "更新后的答案" {
		t.Errorf("更新后答案应为'更新后的答案'，实际 %s", updated.Answer)
	}
	if updated.Enabled {
		t.Error("更新后应为停用")
	}

	// Update 不存在
	if err := UpdateFAQ(ctx, db, &FAQ{ID: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("更新不存在应返回 ErrNotFound，实际 %v", err)
	}

	// List（含停用）
	all, err := ListFAQs(ctx, db, false, 100)
	if err != nil {
		t.Fatalf("查询列表失败: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("应有 1 条，实际 %d", len(all))
	}

	// List（仅启用，此时 FAQ 已停用）
	enabled, _ := ListFAQs(ctx, db, true, 100)
	if len(enabled) != 0 {
		t.Errorf("启用列表应为空，实际 %d", len(enabled))
	}

	// ListEnabledFAQsForMatch
	forMatch, err := ListEnabledFAQsForMatch(ctx, db)
	if err != nil {
		t.Fatalf("查询匹配用 FAQ 失败: %v", err)
	}
	if len(forMatch) != 0 {
		t.Errorf("停用后匹配列表应为空，实际 %d", len(forMatch))
	}

	// 重新启用
	got.Enabled = true
	_ = UpdateFAQ(ctx, db, got)
	forMatch2, _ := ListEnabledFAQsForMatch(ctx, db)
	if len(forMatch2) != 1 {
		t.Errorf("启用后匹配列表应为 1，实际 %d", len(forMatch2))
	}

	// Delete
	if err := DeleteFAQ(ctx, db, faq.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := GetFAQ(ctx, db, faq.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应不存在，实际 %v", err)
	}

	// Delete 不存在
	if err := DeleteFAQ(ctx, db, "nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除不存在应返回 ErrNotFound，实际 %v", err)
	}
}

// TestCountFAQs 验证计数。
func TestCountFAQs(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := Migrate(ctx, db, M2ActiveVersions(), log); err != nil {
		t.Fatal(err)
	}

	n, _ := CountFAQs(ctx, db)
	if n != 0 {
		t.Errorf("初始应为 0，实际 %d", n)
	}

	for i := 0; i < 3; i++ {
		_ = CreateFAQ(ctx, db, &FAQ{
			ID:       uuid.NewString(),
			Question: "问题",
			Answer:   "答案",
			Enabled:  true,
		})
	}
	n, _ = CountFAQs(ctx, db)
	if n != 3 {
		t.Errorf("应有 3 条，实际 %d", n)
	}
}
