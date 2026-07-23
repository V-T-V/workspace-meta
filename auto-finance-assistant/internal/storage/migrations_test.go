package storage

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	db, err := OpenDB(dbPath, log)
	if err != nil {
		t.Fatalf("打开测试 DB 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestMigrate_M1 验证 M1 激活版本只建基础表，不建 FAQ/documents。
func TestMigrate_M1(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	if err := Migrate(ctx, db, M1ActiveVersions(), log); err != nil {
		t.Fatalf("M1 迁移失败: %v", err)
	}

	// 基础表应存在
	for _, table := range []string{"conversations", "messages", "settings", "schema_migrations"} {
		if !tableExists(t, db, table) {
			t.Errorf("M1 后表 %s 应存在", table)
		}
	}
	// M2/M3 表不应存在
	for _, table := range []string{"faqs", "documents", "chunks"} {
		if tableExists(t, db, table) {
			t.Errorf("M1 后表 %s 不应存在（后续 Milestone）", table)
		}
	}
}

// TestMigrate_Idempotent 验证重复执行不报错。
func TestMigrate_Idempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	for i := 0; i < 3; i++ {
		if err := Migrate(ctx, db, M1ActiveVersions(), log); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", i+1, err)
		}
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	if err != nil {
		t.Fatalf("查询表 %s 存在性失败: %v", name, err)
	}
	return n > 0
}
