package sink

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"

	// modernc.org/sqlite 是纯 Go 实现，import 触发 database/sql 驱动注册（驱动名 "sqlite"）。
	_ "modernc.org/sqlite"
)

// SQLiteSink 把行写入 SQLite 数据库文件。
type SQLiteSink struct{}

// Type 返回连接器类型标识。
func (SQLiteSink) Type() string { return "sqlite" }

// Write 把 rows 写到 SQLite。config:
//
//	path   string  数据库文件路径（必填，如 "out.db"）
//	table  string  表名（默认 "records"）
//
// 根据所有行的字段并集自动建表（CREATE TABLE IF NOT EXISTS）：
// 列类型按值类型推断（int/real/text），首行无值的列默认 text。
// 用 INSERT OR REPLACE 写入所有行（按所有列做主键冲突兜底——无显式主键时等价于 INSERT）。
func (SQLiteSink) Write(rows pipeline.Rows, config map[string]any) error {
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("sqlite sink 缺少 path 配置")
	}
	table, _ := config["table"].(string)
	if table == "" {
		table = "records"
	}

	// 列名：所有行字段的并集（排序保证建表/插入确定性）
	cols := collectHeaders(rows)

	// 空输入：无字段无法建表（CREATE TABLE t () 语法错），直接返回。
	if len(cols) == 0 {
		return nil
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("打开 sqlite 失败: %w", err)
	}
	defer db.Close()

	// 可选主键：config.primary_key 指定某字段为 PRIMARY KEY（用于 INSERT OR REPLACE 去重）。
	// 不指定时纯 append（INSERT），适合 ETL 常规的"每次跑都追加"语义。
	// 注意：不用"第一列自动当 PK"——列序由字段名字典序决定，与数据语义无关，
	// 自动猜 PK 会在重复值上静默覆盖丢数据。
	pkField, _ := config["primary_key"].(string)
	pkSet := map[string]bool{}
	if pkField != "" {
		for _, c := range cols {
			if c == pkField {
				pkSet[c] = true
				break
			}
		}
	}

	// 建表（推断列类型）。
	colDefs := make([]string, 0, len(cols))
	for _, c := range cols {
		typ := inferType(rows, c)
		if pkSet[c] {
			colDefs = append(colDefs, fmt.Sprintf("%s %s PRIMARY KEY", quoteIdent(c), typ))
		} else {
			colDefs = append(colDefs, fmt.Sprintf("%s %s", quoteIdent(c), typ))
		}
	}
	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)",
		quoteIdent(table), strings.Join(colDefs, ", "))
	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("建表失败: %w", err)
	}

	if len(rows) == 0 {
		return nil
	}

	// 插入：有主键用 INSERT OR REPLACE（按 PK 去重），无主键用 INSERT（append）。
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "?"
	}
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quoteIdent(c)
	}
	verb := "INSERT"
	if len(pkSet) > 0 {
		verb = "INSERT OR REPLACE"
	}
	insertSQL := fmt.Sprintf("%s INTO %s (%s) VALUES (%s)",
		verb, quoteIdent(table), strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("预处理插入失败: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		args := make([]any, len(cols))
		for i, c := range cols {
			args[i] = normalizeValue(r[c])
		}
		if _, err := stmt.Exec(args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("插入行失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

// inferType 按列里第一个非空值的 Go 类型推断 SQLite 列类型。
// int → INTEGER；float → REAL；其它（含 string/nil 未决） → TEXT。
func inferType(rows pipeline.Rows, col string) string {
	for _, r := range rows {
		v, ok := r[col]
		if !ok || v == nil {
			continue
		}
		switch v.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return "INTEGER"
		case float32, float64:
			return "REAL"
		default:
			return "TEXT"
		}
	}
	return "TEXT"
}

// normalizeValue 把 any 规整成 database/sql 友好的类型。
// （string/bool/数字/null 之外的一律 fmt.Sprint 成字符串。）
func normalizeValue(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, []byte:
		return x
	}
	return fmt.Sprint(v)
}

// quoteIdent 用双引号包裹 SQLite 标识符（防保留字/特殊字符）。
// 内部出现的双引号按 SQL 标准转成两个。
func quoteIdent(name string) string {
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}

func init() {
	pipeline.RegisterSink(&SQLiteSink{})
}
