package load

import (
	"testing"
)

func TestParseTableName_Plain(t *testing.T) {
	schema, table := parseTableName("users")
	if schema != "" {
		t.Errorf("schema = %q, want empty", schema)
	}
	if table != "users" {
		t.Errorf("table = %q, want users", table)
	}
}

func TestParseTableName_Schema(t *testing.T) {
	schema, table := parseTableName("staging.users")
	if schema != "staging" {
		t.Errorf("schema = %q, want staging", schema)
	}
	if table != "users" {
		t.Errorf("table = %q, want users", table)
	}
}

func TestParseTableName_MultiDot(t *testing.T) {
	schema, table := parseTableName("a.b.c")
	if schema != "a" {
		t.Errorf("schema = %q, want a", schema)
	}
	if table != "b.c" {
		t.Errorf("table = %q, want b.c", table)
	}
}

func TestNewSQL_DefaultBatchSize(t *testing.T) {
	cfg := SQLConfig{Driver: "postgres", DSN: "user=test", Table: "t"}
	s := NewSQL(cfg)
	sl := s.(*sqlLoad)
	if sl.cfg.BatchSize != 50 {
		t.Errorf("default batch size = %d, want 50", sl.cfg.BatchSize)
	}
}

func TestNewSQL_CustomBatchSize(t *testing.T) {
	cfg := SQLConfig{Driver: "postgres", DSN: "user=test", Table: "t", BatchSize: 100}
	s := NewSQL(cfg)
	sl := s.(*sqlLoad)
	if sl.cfg.BatchSize != 100 {
		t.Errorf("custom batch size = %d, want 100", sl.cfg.BatchSize)
	}
}

func TestQuoteIdent_MySQL(t *testing.T) {
	l := &sqlLoad{cfg: SQLConfig{Driver: "mysql"}}
	result := l.quoteIdent("my_col")
	if result != "`my_col`" {
		t.Errorf("MySQL quote = %q, want `my_col`", result)
	}
}

func TestQuoteIdent_Postgres(t *testing.T) {
	l := &sqlLoad{cfg: SQLConfig{Driver: "postgres"}}
	result := l.quoteIdent("my_col")
	if result != `"my_col"` {
		t.Errorf("PG quote = %q, want \"my_col\"", result)
	}
}

func TestQuotedTable_Plain(t *testing.T) {
	l := &sqlLoad{cfg: SQLConfig{Driver: "mysql"}}
	l.schema, l.table = parseTableName("users")
	result := l.quotedTable()
	if result != "`users`" {
		t.Errorf("quoted table = %q, want `users`", result)
	}
}

func TestQuotedTable_Schema(t *testing.T) {
	l := &sqlLoad{cfg: SQLConfig{Driver: "postgres"}}
	l.schema, l.table = parseTableName("staging.users")
	result := l.quotedTable()
	if result != `"staging"."users"` {
		t.Errorf("quoted table = %q, want \"staging\".\"users\"", result)
	}
}

func TestBuildInsertSQL_MySQL(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "mysql", Mode: "insert"},
		cols:   []string{"age", "id", "name"},
		schema: "",
		table:  "users",
		buf: []map[string]any{
			{"id": 1, "name": "Alice", "age": 30},
			{"id": 2, "name": "Bob", "age": 25},
		},
	}

	sql, args := l.buildBatchSQL()

	expectedSQL := "INSERT INTO `users` (`age`,`id`,`name`) VALUES (?,?,?),(?,?,?)"
	if sql != expectedSQL {
		t.Errorf("SQL = %q, want %q", sql, expectedSQL)
	}
	if len(args) != 6 {
		t.Errorf("args count = %d, want 6", len(args))
	}
}

func TestBuildInsertSQL_Postgres(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "postgres", Mode: "insert"},
		cols:   []string{"id", "name"},
		schema: "",
		table:  "users",
		buf: []map[string]any{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
	}

	sql, args := l.buildBatchSQL()

	expectedSQL := `INSERT INTO "users" ("id","name") VALUES ($1,$2),($3,$4)`
	if sql != expectedSQL {
		t.Errorf("SQL = %q, want %q", sql, expectedSQL)
	}
	if len(args) != 4 {
		t.Errorf("args count = %d, want 4", len(args))
	}
}

func TestBuildUpsertSQL_Postgres(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "postgres", Mode: "upsert", KeyColumns: []string{"id"}},
		cols:   []string{"id", "name"},
		schema: "",
		table:  "users",
		buf: []map[string]any{
			{"id": 1, "name": "Alice"},
		},
	}

	sql, _ := l.buildBatchSQL()

	expectedSQL := `INSERT INTO "users" ("id","name") VALUES ($1,$2) ON CONFLICT ("id") DO UPDATE SET "id"=EXCLUDED."id","name"=EXCLUDED."name"`
	if sql != expectedSQL {
		t.Errorf("SQL = %q, want %q", sql, expectedSQL)
	}
}

func TestBuildUpsertSQL_MySQL(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "mysql", Mode: "upsert", KeyColumns: []string{"id"}},
		cols:   []string{"id", "name"},
		schema: "",
		table:  "users",
		buf: []map[string]any{
			{"id": 1, "name": "Alice"},
		},
	}

	sql, _ := l.buildBatchSQL()

	expectedSQL := "INSERT INTO `users` (`id`,`name`) VALUES (?,?) ON DUPLICATE KEY UPDATE `id`=VALUES(`id`),`name`=VALUES(`name`)"
	if sql != expectedSQL {
		t.Errorf("SQL = %q, want %q", sql, expectedSQL)
	}
}

func TestBuildCreateTableSQL_MySQL(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "mysql"},
		cols:   []string{"id", "name", "email"},
		schema: "",
		table:  "users",
	}

	// 不连真实 DB，只验证 DDL 生成
	sqlStr := "CREATE TABLE IF NOT EXISTS `users` (`id` TEXT,`name` TEXT,`email` TEXT)"
	if ddl := l.createTableSQL(); ddl != sqlStr {
		t.Errorf("CREATE SQL = %q, want %q", ddl, sqlStr)
	}
}

func TestBuildCreateTableSQL_Schema(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "postgres"},
		cols:   []string{"id", "name"},
		schema: "staging",
		table:  "users",
	}

	expectedSQL := `CREATE TABLE IF NOT EXISTS "staging"."users" ("id" TEXT,"name" TEXT)`
	if ddl := l.createTableSQL(); ddl != expectedSQL {
		t.Errorf("CREATE SQL = %q, want %q", ddl, expectedSQL)
	}
}

func TestBuildCreateTableSQL_Upsert(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "postgres", Mode: "upsert", KeyColumns: []string{"id"}},
		cols:   []string{"id", "name", "email"},
		schema: "",
		table:  "users",
	}

	expectedSQL := `CREATE TABLE IF NOT EXISTS "users" ("id" TEXT,"name" TEXT,"email" TEXT,UNIQUE ("id"))`
	if ddl := l.createTableSQL(); ddl != expectedSQL {
		t.Errorf("UPSERT CREATE SQL = %q, want %q", ddl, expectedSQL)
	}
}

func TestBuildCreateTableSQL_UpsertMultiKey(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "postgres", Mode: "upsert", KeyColumns: []string{"id", "tenant_id"}},
		cols:   []string{"id", "name"},
		schema: "",
		table:  "users",
	}

	expectedSQL := `CREATE TABLE IF NOT EXISTS "users" ("id" TEXT,"name" TEXT,UNIQUE ("id","tenant_id"))`
	if ddl := l.createTableSQL(); ddl != expectedSQL {
		t.Errorf("Multi-key CREATE SQL = %q, want %q", ddl, expectedSQL)
	}
}

func TestBuildCreateTableSQL_SingleColumn(t *testing.T) {
	l := &sqlLoad{
		cfg:    SQLConfig{Driver: "postgres"},
		cols:   []string{"id"},
		schema: "",
		table:  "test",
	}

	expectedSQL := `CREATE TABLE IF NOT EXISTS "test" ("id" TEXT)`
	if ddl := l.createTableSQL(); ddl != expectedSQL {
		t.Errorf("CREATE SQL = %q, want %q", ddl, expectedSQL)
	}
}
