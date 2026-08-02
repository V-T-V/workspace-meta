package storage

// 第六轮：对话历史持久化深层测试。
// 覆盖消息全字段持久化、时序排序、updated_at 刷新、级联删除、
// 分页 limit 边界、过期清理（retention）。

import (
	"context"
	"log/slog"
	"strconv"
	"os"
	"testing"
	"time"
)

// ctxLog 构造静默 logger。
func ctxLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// ===========================================================================
// 消息全字段持久化
// ===========================================================================

func TestAppendMessage_AllFieldsPreserved(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	convID := "conv-full"
	CreateConversation(ctx, db, convID, "u1", "全字段测试")

	msg := &Message{
		ID:               "m1",
		ConversationID:   convID,
		Role:             "assistant",
		Content:          "这是带元数据的回答",
		Intent:           "faq_short",
		Confidence:       0.92,
		Sources:          `{"doc":"d1","section":"利率"}`,
		DurationMS:       1234,
		PromptTokens:     56,
		CompletionTokens: 78,
	}
	if err := AppendMessage(ctx, db, msg); err != nil {
		t.Fatalf("追加失败: %v", err)
	}
	got, err := ListMessages(ctx, db, convID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("应 1 条，实际 %d", len(got))
	}
	m := got[0]
	if m.Intent != "faq_short" {
		t.Errorf("Intent 应 faq_short，实际 %s", m.Intent)
	}
	if m.Confidence != 0.92 {
		t.Errorf("Confidence 应 0.92，实际 %v", m.Confidence)
	}
	if m.Sources != `{"doc":"d1","section":"利率"}` {
		t.Errorf("Sources 不匹配，实际 %s", m.Sources)
	}
	if m.DurationMS != 1234 {
		t.Errorf("DurationMS 应 1234，实际 %d", m.DurationMS)
	}
	if m.PromptTokens != 56 || m.CompletionTokens != 78 {
		t.Errorf("token 计数不匹配：%d/%d", m.PromptTokens, m.CompletionTokens)
	}
}

// ===========================================================================
// 消息时序排序
// ===========================================================================

func TestListMessages_ChronologicalOrder(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	convID := "conv-order"
	CreateConversation(ctx, db, convID, "", "排序测试")

	// 依次追加 3 条（created_at 由 DB 默认值填充，间隔靠手动 sleep 保证递增）
	msgs := []Message{
		{ID: "m1", ConversationID: convID, Role: "user", Content: "第一"},
		{ID: "m2", ConversationID: convID, Role: "assistant", Content: "第二"},
		{ID: "m3", ConversationID: convID, Role: "user", Content: "第三"},
	}
	for i := range msgs {
		AppendMessage(ctx, db, &msgs[i])
		if i < len(msgs)-1 {
			time.Sleep(1100 * time.Millisecond) // SQLite datetime 秒级精度
		}
	}
	got, err := ListMessages(ctx, db, convID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("应 3 条，实际 %d", len(got))
	}
	// 应按 created_at 升序：m1 → m2 → m3
	if got[0].ID != "m1" || got[1].ID != "m2" || got[2].ID != "m3" {
		ids := []string{got[0].ID, got[1].ID, got[2].ID}
		t.Errorf("应按时间升序，实际 %v", ids)
	}
}

// ===========================================================================
// updated_at 刷新
// ===========================================================================

func TestAppendMessage_RefreshesUpdatedAt(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	convID := "conv-ts"
	conv, _ := CreateConversation(ctx, db, convID, "", "时间戳测试")
	origUpdated := conv.UpdatedAt

	time.Sleep(1100 * time.Millisecond)
	AppendMessage(ctx, db, &Message{ID: "m1", ConversationID: convID, Role: "user", Content: "x"})

	got, _ := GetConversation(ctx, db, convID)
	if !got.UpdatedAt.After(origUpdated) {
		t.Errorf("追加消息后 updated_at 应刷新，orig=%v now=%v", origUpdated, got.UpdatedAt)
	}
}

// ===========================================================================
// 级联删除
// ===========================================================================

func TestDeleteConversation_CascadesMessages(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	convID := "conv-cascade"
	CreateConversation(ctx, db, convID, "", "级联测试")
	AppendMessage(ctx, db, &Message{ID: "m1", ConversationID: convID, Role: "user", Content: "a"})
	AppendMessage(ctx, db, &Message{ID: "m2", ConversationID: convID, Role: "assistant", Content: "b"})

	// 删除会话
	if err := DeleteConversation(ctx, db, convID); err != nil {
		t.Fatal(err)
	}
	// 会话不存在
	if _, err := GetConversation(ctx, db, convID); err != ErrNotFound {
		t.Errorf("会话应已删除（ErrNotFound），实际 %v", err)
	}
	// 关联消息也被级联删除
	msgs, _ := ListMessages(ctx, db, convID, 10)
	if len(msgs) != 0 {
		t.Errorf("级联删除后应无消息，实际 %d", len(msgs))
	}
}

// ===========================================================================
// 分页 limit 边界
// ===========================================================================

func TestListConversations_LimitClamping(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	// 建 3 个会话
	for i := 0; i < 3; i++ {
		CreateConversation(ctx, db, "c"+strconv.Itoa(i), "", "会话")
	}
	// limit=0 → 默认 50
	convs, _ := ListConversations(ctx, db, 0)
	if len(convs) != 3 {
		t.Errorf("limit=0 应返回全部 3，实际 %d", len(convs))
	}
	// limit=500+ → 钳制 50
	convs, _ = ListConversations(ctx, db, 9999)
	if len(convs) != 3 {
		t.Errorf("超大 limit 应返回全部 3，实际 %d", len(convs))
	}
	// limit=2 → 只 2 条
	convs, _ = ListConversations(ctx, db, 2)
	if len(convs) != 2 {
		t.Errorf("limit=2 应 2 条，实际 %d", len(convs))
	}
}

func TestListMessages_LimitClamping(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	CreateConversation(ctx, db, "c1", "", "x")
	for i := 0; i < 3; i++ {
		AppendMessage(ctx, db, &Message{ID: "m" + strconv.Itoa(i), ConversationID: "c1", Role: "user", Content: "x"})
	}
	// limit=0 → 默认 100
	msgs, _ := ListMessages(ctx, db, "c1", 0)
	if len(msgs) != 3 {
		t.Errorf("limit=0 应 3 条，实际 %d", len(msgs))
	}
	// limit=600 → 钳制 100（仍返回全部 3）
	msgs, _ = ListMessages(ctx, db, "c1", 600)
	if len(msgs) != 3 {
		t.Errorf("超大 limit 应 3 条，实际 %d", len(msgs))
	}
	// limit=2
	msgs, _ = ListMessages(ctx, db, "c1", 2)
	if len(msgs) != 2 {
		t.Errorf("limit=2 应 2 条，实际 %d", len(msgs))
	}
}

// ===========================================================================
// 会话列表按 updated_at 倒序
// ===========================================================================

func TestListConversations_OrderByUpdatedDesc(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	CreateConversation(ctx, db, "old", "", "旧")
	time.Sleep(1100 * time.Millisecond)
	CreateConversation(ctx, db, "new", "", "新")

	convs, _ := ListConversations(ctx, db, 10)
	if len(convs) != 2 {
		t.Fatalf("应 2 条，实际 %d", len(convs))
	}
	// newer 应排前面（updated_at 倒序）
	if convs[0].ID != "new" {
		t.Errorf("最新会话应排首，实际首条 %s", convs[0].ID)
	}
}

// ===========================================================================
// 过期数据清理（retention）
// ===========================================================================

func TestPurgeExpiredData_DeletesOldConversations(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	// 手动插入一个"30 天前"的会话（绕过 CreateConversation 的 now()）
	oldTime := time.Now().AddDate(0, 0, -30).UTC().Format("2006-01-02 15:04:05")
	db.ExecContext(ctx, `INSERT INTO conversations(id, user_id, title, created_at, updated_at) VALUES('old','','旧',?,?)`, oldTime, oldTime)
	// 插入一个当前会话
	CreateConversation(ctx, db, "fresh", "", "新")

	// 保留 90 天：两者都不该被删（old 才 30 天）
	r, err := PurgeExpiredData(ctx, db, 90)
	if err != nil {
		t.Fatal(err)
	}
	if r.Conversations != 0 {
		t.Errorf("保留 90 天不应删除 30 天前的会话，实际删除 %d", r.Conversations)
	}

	// 保留 10 天：old（30 天前）应被删，fresh 不删
	r, err = PurgeExpiredData(ctx, db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if r.Conversations != 1 {
		t.Errorf("保留 10 天应删除 1 个旧会话，实际 %d", r.Conversations)
	}
	// fresh 仍在
	if _, err := GetConversation(ctx, db, "fresh"); err != nil {
		t.Errorf("fresh 不应被删，实际 %v", err)
	}
}

func TestPurgeExpiredData_DeletesOldFeedback(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	// 插入旧反馈
	oldTime := time.Now().AddDate(0, 0, -100).UTC().Format("2006-01-02 15:04:05")
	db.ExecContext(ctx, `INSERT INTO feedback(id, message_id, rating, created_at) VALUES('fb1','',1,?)`, oldTime)
	// 当前反馈
	CreateFeedback(ctx, db, &Feedback{ID: "fb2", MessageID: "", Rating: 1})

	r, err := PurgeExpiredData(ctx, db, 30)
	if err != nil {
		t.Fatal(err)
	}
	if r.Feedback != 1 {
		t.Errorf("应删除 1 条旧反馈，实际 %d", r.Feedback)
	}
}

func TestPurgeExpiredData_DefaultRetainDays(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	// retainDays<=0 应钳制为 90
	r, err := PurgeExpiredData(ctx, db, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 空库清理应成功（不报错）
	_ = r
}

func TestPurgeExpiredData_DeletesOldAuditLogs(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	oldTime := time.Now().AddDate(0, 0, -200).UTC().Format("2006-01-02 15:04:05")
	CreateAuditLog(ctx, db, &AuditLog{ID: "a1", Action: "old.test"})
	// 手动把审计日志改老
	db.ExecContext(ctx, `UPDATE audit_logs SET created_at=? WHERE id=?`, oldTime, "a1")
	CreateAuditLog(ctx, db, &AuditLog{ID: "a2", Action: "new.test"})

	r, err := PurgeExpiredData(ctx, db, 30)
	if err != nil {
		t.Fatal(err)
	}
	if r.AuditLogs != 1 {
		t.Errorf("应删除 1 条旧审计日志，实际 %d", r.AuditLogs)
	}
}

// ===========================================================================
// 反馈与审计持久化
// ===========================================================================

func TestFeedback_CRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	// 创建
	CreateFeedback(ctx, db, &Feedback{ID: "f1", MessageID: "m1", Rating: 1, Reason: "好", Correction: ""})
	CreateFeedback(ctx, db, &Feedback{ID: "f2", MessageID: "m2", Rating: -1, Reason: "", Correction: "纠错"})
	// 查询
	items, err := ListFeedback(ctx, db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("应 2 条，实际 %d", len(items))
	}
	// 验证字段
	var f1 *Feedback
	for _, f := range items {
		if f.ID == "f1" {
			f1 = f
		}
	}
	if f1 == nil {
		t.Fatal("未找到 f1")
	}
	if f1.Rating != 1 || f1.Reason != "好" {
		t.Errorf("f1 字段不匹配：rating=%d reason=%s", f1.Rating, f1.Reason)
	}
}

func TestListFeedback_LimitClamping(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	for i := 0; i < 3; i++ {
		CreateFeedback(ctx, db, &Feedback{ID: "f" + strconv.Itoa(i), Rating: 1})
	}
	items, _ := ListFeedback(ctx, db, 0)
	if len(items) != 3 {
		t.Errorf("limit=0 应 3 条，实际 %d", len(items))
	}
	items, _ = ListFeedback(ctx, db, 9999)
	if len(items) != 3 {
		t.Errorf("超大 limit 应 3 条，实际 %d", len(items))
	}
}

func TestAuditLog_FilterByAction(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())

	CreateAuditLog(ctx, db, &AuditLog{ID: "a1", Action: "faq.create", TargetID: "f1"})
	CreateAuditLog(ctx, db, &AuditLog{ID: "a2", Action: "doc.upload", TargetID: "d1"})
	CreateAuditLog(ctx, db, &AuditLog{ID: "a3", Action: "faq.create", TargetID: "f2"})

	// 过滤 faq.create
	items, _ := ListAuditLogs(ctx, db, "faq.create", 10)
	if len(items) != 2 {
		t.Errorf("faq.create 应 2 条，实际 %d", len(items))
	}
	for _, a := range items {
		if a.Action != "faq.create" {
			t.Errorf("过滤结果应都是 faq.create，实际 %s", a.Action)
		}
	}
	// 不过滤
	items, _ = ListAuditLogs(ctx, db, "", 10)
	if len(items) != 3 {
		t.Errorf("不过滤应 3 条，实际 %d", len(items))
	}
}

func TestListRefusedMessages(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Migrate(ctx, db, AllActiveVersions(), ctxLog())
	CreateConversation(ctx, db, "c1", "", "x")
	// 普通 + 拒答消息
	AppendMessage(ctx, db, &Message{ID: "m1", ConversationID: "c1", Role: "user", Content: "普通问", Intent: "normal"})
	AppendMessage(ctx, db, &Message{ID: "m2", ConversationID: "c1", Role: "assistant", Content: "拒答", Intent: "refuse", Confidence: 0.2})

	items, err := ListRefusedMessages(ctx, db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("应只返回 1 条拒答消息，实际 %d", len(items))
	}
	if items[0].Intent != "refuse" {
		t.Errorf("intent 应 refuse，实际 %s", items[0].Intent)
	}
}
