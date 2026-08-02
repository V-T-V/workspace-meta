package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Conversation 对应 conversations 表。M1 子集字段。
type Conversation struct {
	ID        string
	UserID    string
	Title     string
	Summary   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message 对应 messages 表。sources 为 JSON 文本（M4 填充）。
type Message struct {
	ID               string
	ConversationID   string
	Role             string // user | assistant | system
	Content          string
	Intent           string
	Confidence       float64
	Sources          string
	DurationMS       int64
	PromptTokens     int
	CompletionTokens int
	CreatedAt        time.Time
}

// ErrNotFound 表示查询无结果。
var ErrNotFound = errors.New("not found")

// CreateConversation 插入新会话并返回完整记录。
func CreateConversation(ctx context.Context, db *sql.DB, id, userID, title string) (*Conversation, error) {
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO conversations(id, user_id, title, created_at, updated_at) VALUES(?, ?, ?, ?, ?)
	`, id, nullString(userID), nullString(title), now, now); err != nil {
		return nil, fmt.Errorf("[storage] 创建会话失败: %w", err)
	}
	return &Conversation{ID: id, UserID: userID, Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// GetConversation 按 ID 查询会话。
func GetConversation(ctx context.Context, db *sql.DB, id string) (*Conversation, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, COALESCE(user_id,''), COALESCE(title,''), COALESCE(summary,''), created_at, updated_at
		FROM conversations WHERE id = ?
	`, id)
	var c Conversation
	var createdRaw, updatedRaw string
	if err := row.Scan(&c.ID, &c.UserID, &c.Title, &c.Summary, &createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("[storage] 查询会话失败: %w", err)
	}
	c.CreatedAt = parseTime(createdRaw)
	c.UpdatedAt = parseTime(updatedRaw)
	return &c, nil
}

// ListConversations 按更新时间倒序返回会话列表。
func ListConversations(ctx context.Context, db *sql.DB, limit int) ([]*Conversation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(user_id,''), COALESCE(title,''), COALESCE(summary,''), created_at, updated_at
		FROM conversations ORDER BY updated_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询会话列表失败: %w", err)
	}
	defer rows.Close()

	var out []*Conversation
	for rows.Next() {
		var c Conversation
		var createdRaw, updatedRaw string
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.Summary, &createdRaw, &updatedRaw); err != nil {
			return nil, err
		}
		c.CreatedAt = parseTime(createdRaw)
		c.UpdatedAt = parseTime(updatedRaw)
		out = append(out, &c)
	}
	return out, rows.Err()
}

// AppendMessage 插入一条消息并更新会话的 updated_at。
func AppendMessage(ctx context.Context, db *sql.DB, m *Message) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages(id, conversation_id, role, content, intent, confidence, sources, duration_ms, prompt_tokens, completion_tokens)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.ID, m.ConversationID, m.Role, m.Content,
		nullString(m.Intent), m.Confidence, nullString(m.Sources),
		m.DurationMS, m.PromptTokens, m.CompletionTokens,
	); err != nil {
		return fmt.Errorf("[storage] 插入消息失败: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE conversations SET updated_at = datetime('now') WHERE id = ?`, m.ConversationID); err != nil {
		return fmt.Errorf("[storage] 更新会话时间失败: %w", err)
	}
	return tx.Commit()
}

// ListMessages 返回指定会话的消息（按时间正序）。
func ListMessages(ctx context.Context, db *sql.DB, conversationID string, limit int) ([]*Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, conversation_id, role, content, COALESCE(intent,''), COALESCE(confidence,0), COALESCE(sources,''),
		       COALESCE(duration_ms,0), COALESCE(prompt_tokens,0), COALESCE(completion_tokens,0), created_at
		FROM messages WHERE conversation_id = ? ORDER BY created_at ASC LIMIT ?
	`, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询消息失败: %w", err)
	}
	defer rows.Close()

	var out []*Message
	for rows.Next() {
		var m Message
		var createdRaw string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Intent, &m.Confidence,
			&m.Sources, &m.DurationMS, &m.PromptTokens, &m.CompletionTokens, &createdRaw); err != nil {
			return nil, err
		}
		m.CreatedAt = parseTime(createdRaw)
		out = append(out, &m)
	}
	return out, rows.Err()
}

// nullString 空串转 NULL（SQLite COALESCE 友好）。
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// parseTime 解析 SQLite datetime 文本。
func parseTime(raw string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

// DeleteConversation 删除会话（messages 表 ON DELETE CASCADE 会自动删除关联消息）。
func DeleteConversation(ctx context.Context, db *sql.DB, id string) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, id); err != nil {
		return fmt.Errorf("[storage] 删除会话失败: %w", err)
	}
	return nil
}
