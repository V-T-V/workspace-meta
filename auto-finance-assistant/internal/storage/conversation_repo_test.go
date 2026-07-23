package storage

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
)

// TestConversationLifecycle 验证会话+消息 CRUD 闭环。
func TestConversationLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := Migrate(ctx, db, M1ActiveVersions(), log); err != nil {
		t.Fatal(err)
	}

	// 创建会话
	convID := uuid.NewString()
	conv, err := CreateConversation(ctx, db, convID, "u1", "测试会话")
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if conv.ID != convID {
		t.Errorf("会话 ID 不匹配")
	}

	// 查询会话
	got, err := GetConversation(ctx, db, convID)
	if err != nil {
		t.Fatalf("查询会话失败: %v", err)
	}
	if got.Title != "测试会话" {
		t.Errorf("标题不匹配: %s", got.Title)
	}

	// 不存在
	if _, err := GetConversation(ctx, db, "nonexistent"); err != ErrNotFound {
		t.Errorf("不存在应返回 ErrNotFound，实际 %v", err)
	}

	// 追加消息
	userMsg := &Message{
		ID:             uuid.NewString(),
		ConversationID: convID,
		Role:           "user",
		Content:        "你好",
	}
	if err := AppendMessage(ctx, db, userMsg); err != nil {
		t.Fatalf("追加用户消息失败: %v", err)
	}
	assistantMsg := &Message{
		ID:               uuid.NewString(),
		ConversationID:   convID,
		Role:             "assistant",
		Content:          "你好，我是汽车金融客服助手",
		Confidence:       0.85,
		DurationMS:       1200,
		CompletionTokens: 12,
	}
	if err := AppendMessage(ctx, db, assistantMsg); err != nil {
		t.Fatalf("追加助手消息失败: %v", err)
	}

	// 查询消息列表
	msgs, err := ListMessages(ctx, db, convID, 100)
	if err != nil {
		t.Fatalf("查询消息失败: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("应有 2 条消息，实际 %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("消息顺序或角色错误: %s %s", msgs[0].Role, msgs[1].Role)
	}
	if msgs[1].Confidence != 0.85 {
		t.Errorf("置信度不匹配: %v", msgs[1].Confidence)
	}

	// 会话列表
	convs, err := ListConversations(ctx, db, 10)
	if err != nil {
		t.Fatalf("查询会话列表失败: %v", err)
	}
	if len(convs) != 1 || convs[0].ID != convID {
		t.Errorf("会话列表不匹配: %+v", convs)
	}
}

// TestSettings 验证设置 upsert。
func TestSettings(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := Migrate(ctx, db, M1ActiveVersions(), log); err != nil {
		t.Fatal(err)
	}

	if _, err := GetSetting(ctx, db, "k1"); err != ErrNotFound {
		t.Fatalf("未写入应返回 ErrNotFound，实际 %v", err)
	}
	if err := SetSetting(ctx, db, "k1", "v1"); err != nil {
		t.Fatal(err)
	}
	if v, _ := GetSetting(ctx, db, "k1"); v != "v1" {
		t.Errorf("应为 v1，实际 %s", v)
	}
	// 覆盖
	if err := SetSetting(ctx, db, "k1", "v2"); err != nil {
		t.Fatal(err)
	}
	if v, _ := GetSetting(ctx, db, "k1"); v != "v2" {
		t.Errorf("应为 v2，实际 %s", v)
	}
}
