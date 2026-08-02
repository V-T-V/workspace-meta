package sink

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"

	_ "modernc.org/sqlite"
)

func TestSQLiteSink_CreateAndInsert(t *testing.T) {
	rows := pipeline.Rows{
		{"id": 1, "name": "alice", "amount": 100},
		{"id": 2, "name": "bob", "amount": 200},
	}
	dbPath := filepath.Join(t.TempDir(), "out.db")
	if err := (SQLiteSink{}).Write(rows, map[string]any{"path": dbPath, "table": "records"}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("打开 db 失败: %v", err)
	}
	defer db.Close()

	// 表应存在且行数正确
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count); err != nil {
		t.Fatalf("查询行数失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("应有 2 行，得到 %d", count)
	}

	// 验证值能查回来
	var name string
	var amount int
	if err := db.QueryRow("SELECT name, amount FROM records WHERE id = 1").Scan(&name, &amount); err != nil {
		t.Fatalf("查询行失败: %v", err)
	}
	if name != "alice" || amount != 100 {
		t.Fatalf("数据不符: name=%q amount=%d", name, amount)
	}
}

func TestSQLiteSink_TypeInference(t *testing.T) {
	// int → INTEGER，float → REAL，string → TEXT
	rows := pipeline.Rows{{"i": 1, "f": 1.5, "s": "x"}}
	dbPath := filepath.Join(t.TempDir(), "types.db")
	if err := (SQLiteSink{}).Write(rows, map[string]any{"path": dbPath}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	db, _ := sql.Open("sqlite", dbPath)
	defer db.Close()

	typeCol := func(col string) string {
		var ty string
		if err := db.QueryRow("SELECT type FROM pragma_table_info('records') WHERE name = ?", col).Scan(&ty); err != nil {
			t.Fatalf("查 %s 类型失败: %v", col, err)
		}
		return ty
	}
	if typeCol("i") != "INTEGER" {
		t.Fatalf("i 应为 INTEGER，得到 %s", typeCol("i"))
	}
	if typeCol("f") != "REAL" {
		t.Fatalf("f 应为 REAL，得到 %s", typeCol("f"))
	}
	if typeCol("s") != "TEXT" {
		t.Fatalf("s 应为 TEXT，得到 %s", typeCol("s"))
	}
}

func TestSQLiteSink_DefaultTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "default.db")
	if err := (SQLiteSink{}).Write(pipeline.Rows{{"a": 1}}, map[string]any{"path": dbPath}); err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	db, _ := sql.Open("sqlite", dbPath)
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count); err != nil {
		t.Fatalf("默认表名 records 应存在: %v", err)
	}
	if count != 1 {
		t.Fatalf("默认表应有 1 行，得到 %d", count)
	}
}

func TestSQLiteSink_EmptyRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	// 空输入：无字段，建表应跳过（或建空表），不应崩溃
	if err := (SQLiteSink{}).Write(pipeline.Rows{}, map[string]any{"path": dbPath, "table": "t"}); err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
}

func TestSQLiteSink_MissingPath(t *testing.T) {
	if err := (SQLiteSink{}).Write(pipeline.Rows{{"a": 1}}, map[string]any{}); err == nil {
		t.Fatal("缺少 path 应报错")
	}
}

func TestSQLiteSink_Idempotent_Rerun(t *testing.T) {
	// 同一文件写两次，配 primary_key=id 时（CREATE IF NOT EXISTS + INSERT OR REPLACE）应幂等去重。
	rows := pipeline.Rows{{"id": 1, "name": "alice"}}
	dbPath := filepath.Join(t.TempDir(), "rerun.db")
	for i := 0; i < 2; i++ {
		if err := (SQLiteSink{}).Write(rows, map[string]any{"path": dbPath, "primary_key": "id"}); err != nil {
			t.Fatalf("第 %d 次写失败: %v", i+1, err)
		}
	}
	db, _ := sql.Open("sqlite", dbPath)
	defer db.Close()
	var count int
	db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count)
	if count != 1 {
		t.Fatalf("配 primary_key 重跑后应仍为 1 行（INSERT OR REPLACE），得到 %d", count)
	}
}

func TestSQLiteSink_AppendWithoutPK(t *testing.T) {
	// 不配 primary_key 时纯 append，重跑会累加（ETL 常规语义）。
	rows := pipeline.Rows{{"id": 1, "name": "alice"}}
	dbPath := filepath.Join(t.TempDir(), "append.db")
	for i := 0; i < 2; i++ {
		if err := (SQLiteSink{}).Write(rows, map[string]any{"path": dbPath}); err != nil {
			t.Fatalf("第 %d 次写失败: %v", i+1, err)
		}
	}
	db, _ := sql.Open("sqlite", dbPath)
	defer db.Close()
	var count int
	db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count)
	if count != 2 {
		t.Fatalf("无 primary_key 重跑后应累加为 2 行（append），得到 %d", count)
	}
}

func TestSQLiteSink_Registered(t *testing.T) {
	if _, ok := pipeline.GetSink("sqlite"); !ok {
		t.Fatal("sqlite sink 未注册")
	}
}
