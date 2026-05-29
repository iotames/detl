package detl

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/iotames/detl/internal/engine"
	"github.com/iotames/detl/internal/load"
	"github.com/iotames/detl/internal/source"
	"github.com/iotames/detl/internal/transform"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

const (
	testTargetTable = "etl_integration_target"
)

// TestSQLLoad_MySQL_to_PG 全链路集成测试：
// MySQL 抽取 → Python 脚本转换 → PostgreSQL UPSERT 写入
func TestSQLLoad_MySQL_to_PG(t *testing.T) {
	// 1. 准备 MySQL 源数据
	mysqlDB, err := sql.Open(testMySQLDriver, testMySQLDSN)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer mysqlDB.Close()
	if err := mysqlDB.Ping(); err != nil {
		t.Fatalf("MySQL 不可用，跳过测试: %v", err)
	}

	setupMySQLData(t, mysqlDB)
	defer func() {
		mysqlDB.Exec("DROP TABLE IF EXISTS " + testMySQLTable)
	}()

	// 2. 准备 PG 目标环境
	pgDB, err := sql.Open(testDriver, testDSN)
	if err != nil {
		t.Fatalf("连接 PG 失败: %v", err)
	}
	defer pgDB.Close()
	if err := pgDB.Ping(); err != nil {
		t.Fatalf("PG 不可用，跳过测试: %v", err)
	}

	// 清理旧数据
	pgDB.Exec("DROP TABLE IF EXISTS " + testTargetTable)

	// 3. 定义 Source：MySQL 抽取
	src := source.NewSQL(source.SQLConfig{
		Driver: testMySQLDriver,
		DSN:    testMySQLDSN,
		Query:  `SELECT id, first_name, last_name, email, age, created_at FROM ` + testMySQLTable + ` ORDER BY id`,
	})

	// 4. 定义 Transform：Python 脚本清洗
	pythonScript := filepath.Join("main", "script", "t_users.py")
	if _, err := os.Stat(pythonScript); err != nil {
		t.Fatalf("Python 脚本不存在: %v", err)
	}
	tf := transform.NewPython(transform.PythonConfig{
		ScriptPath: pythonScript,
	})

	// 5. 定义 Load：PostgreSQL UPSERT
	ld := load.NewSQL(load.SQLConfig{
		Driver:      testDriver,
		DSN:         testDSN,
		Table:       testTargetTable,
		Mode:        "upsert",
		KeyColumns:  []string{"id"},
		CreateTable: true,
		BatchSize:   3,
	})

	// 6. 执行 Pipeline
	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		t.Fatalf("Pipeline 执行失败: %v", err)
	}

	// 7. 验证 PG 目标表数据
	rows, err := pgDB.Query("SELECT id, full_name, email, age, source FROM " + testTargetTable + " ORDER BY id")
	if err != nil {
		t.Fatalf("查询 PG 目标表失败: %v", err)
	}
	defer rows.Close()

	type resultRow struct {
		ID       int
		FullName string
		Email    string
		Age      int
		Source   string
	}
	var results []resultRow
	for rows.Next() {
		var r resultRow
		if err := rows.Scan(&r.ID, &r.FullName, &r.Email, &r.Age, &r.Source); err != nil {
			t.Fatalf("扫描结果行失败: %v", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows 迭代错误: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("PG 目标表行数 = %d, 期望 5", len(results))
	}

	// 逐行验证
	t.Logf("验证 UPSERT 写入结果（%d 行）:", len(results))
	for _, r := range results {
		t.Logf("  id=%d  name=%s  email=%s  age=%d  source=%s",
			r.ID, r.FullName, r.Email, r.Age, r.Source)

		switch r.ID {
		case 1:
			assertEqual(t, "full_name", r.FullName, "John Doe")
			assertEqual(t, "email", r.Email, "john@example.com")
			assertEqual(t, "age", r.Age, 30)
		case 2:
			assertEqual(t, "full_name", r.FullName, "Jane Smith")
			assertEqual(t, "email", r.Email, "jane@example.com") // 小写
		case 3:
			assertEqual(t, "full_name", r.FullName, "Bob")
			assertEqual(t, "email", r.Email, "bob@test.com")
			assertEqual(t, "age", r.Age, 0) // NULL → 0
		case 4:
			assertEqual(t, "full_name", r.FullName, "Alice Wang")
			assertEqual(t, "email", r.Email, "alice.wang@test.com") // 小写
		case 5:
			assertEqual(t, "full_name", r.FullName, "Lee")
			assertEqual(t, "email", r.Email, "null_first@example.com")
			assertEqual(t, "age", r.Age, 35)
		}
	}

	// 8. 测试 UPSERT 幂等性：再跑一次，数据应不变
	t.Log("测试 UPSERT 幂等性：重新执行 Pipeline")
	src2 := source.NewSQL(source.SQLConfig{
		Driver: testMySQLDriver,
		DSN:    testMySQLDSN,
		Query:  `SELECT id, first_name, last_name, email, age, created_at FROM ` + testMySQLTable + ` ORDER BY id`,
	})
	tf2 := transform.NewPython(transform.PythonConfig{
		ScriptPath: pythonScript,
	})
	ld2 := load.NewSQL(load.SQLConfig{
		Driver:      testDriver,
		DSN:         testDSN,
		Table:       testTargetTable,
		Mode:        "upsert",
		KeyColumns:  []string{"id"},
		CreateTable: false,
		BatchSize:   3,
	})
	p2 := engine.New(src2, tf2, ld2)
	if err := p2.Run(); err != nil {
		t.Fatalf("第二次 Pipeline 执行失败: %v", err)
	}

	// 验证行数不变
	var count int
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM " + testTargetTable).Scan(&count); err != nil {
		t.Fatalf("统计行数失败: %v", err)
	}
	if count != 5 {
		t.Fatalf("UPSERT 幂等性测试失败：行数 = %d, 期望 5（不应产生重复行）", count)
	}
	t.Log("✅ UPSERT 幂等性验证通过：重复运行数据一致")

	// 清理
	pgDB.Exec("DROP TABLE IF EXISTS " + testTargetTable)
	t.Log("✅ 集成测试通过：MySQL → Python 转换 → PostgreSQL UPSERT")
}

// TestSQLLoad_InsertMode 测试 INSERT 模式：MySQL → 内置转换 → PG INSERT
func TestSQLLoad_InsertMode_PG(t *testing.T) {
	pgDB, err := sql.Open(testDriver, testDSN)
	if err != nil {
		t.Fatalf("连接 PG 失败: %v", err)
	}
	defer pgDB.Close()
	if err := pgDB.Ping(); err != nil {
		t.Fatalf("PG 不可用，跳过测试: %v", err)
	}
	pgDB.Exec("DROP TABLE IF EXISTS " + testTargetTable)

	mysqlDB, err := sql.Open(testMySQLDriver, testMySQLDSN)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer mysqlDB.Close()
	if err := mysqlDB.Ping(); err != nil {
		t.Fatalf("MySQL 不可用: %v", err)
	}
	setupMySQLData(t, mysqlDB)
	defer mysqlDB.Exec("DROP TABLE IF EXISTS " + testMySQLTable)

	src := source.NewSQL(source.SQLConfig{
		Driver: testMySQLDriver,
		DSN:    testMySQLDSN,
		Query:  `SELECT id, first_name, last_name, email, age, created_at FROM ` + testMySQLTable + ` ORDER BY id`,
	})

	tf := transform.Func(func(row map[string]any) ([]map[string]any, error) {
		return []map[string]any{{
			"id":         row["id"],
			"full_name":  nullStr(row["first_name"]) + " " + nullStr(row["last_name"]),
			"email":      lower(nullStr(row["email"])),
			"age":        row["age"],
			"etl_source": "mysql",
		}}, nil
	})

	ld := load.NewSQL(load.SQLConfig{
		Driver:      testDriver,
		DSN:         testDSN,
		Table:       testTargetTable,
		Mode:        "insert",
		CreateTable: true,
		BatchSize:   5,
	})

	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		t.Fatalf("INSERT Pipeline 执行失败: %v", err)
	}

	var count int
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM " + testTargetTable).Scan(&count); err != nil {
		t.Fatalf("统计行数失败: %v", err)
	}
	if count != 5 {
		t.Fatalf("INSERT 行数 = %d, 期望 5", count)
	}
	t.Logf("✅ INSERT 模式测试通过：写入 %d 行", count)

	pgDB.Exec("DROP TABLE IF EXISTS " + testTargetTable)
}

// TestSQLLoad_StagingSchema 测试带 schema 的表名：public.schema
func TestSQLLoad_SchemaTable(t *testing.T) {
	pgDB, err := sql.Open(testDriver, testDSN)
	if err != nil {
		t.Fatalf("连接 PG 失败: %v", err)
	}
	defer pgDB.Close()
	if err := pgDB.Ping(); err != nil {
		t.Fatalf("PG 不可用: %v", err)
	}
	pgDB.Exec("DROP TABLE IF EXISTS " + testTargetTable)

	mysqlDB, err := sql.Open(testMySQLDriver, testMySQLDSN)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer mysqlDB.Close()
	if err := mysqlDB.Ping(); err != nil {
		t.Fatalf("MySQL 不可用: %v", err)
	}
	setupMySQLData(t, mysqlDB)
	defer mysqlDB.Exec("DROP TABLE IF EXISTS " + testMySQLTable)

	src := source.NewSQL(source.SQLConfig{
		Driver: testMySQLDriver,
		DSN:    testMySQLDSN,
		Query:  `SELECT id, first_name, last_name FROM ` + testMySQLTable + ` ORDER BY id LIMIT 2`,
	})

	schemaTable := "public." + testTargetTable
	ld := load.NewSQL(load.SQLConfig{
		Driver:      testDriver,
		DSN:         testDSN,
		Table:       schemaTable,
		Mode:        "insert",
		CreateTable: true,
		BatchSize:   2,
	})

	p := engine.New(src, nil, ld)
	if err := p.Run(); err != nil {
		t.Fatalf("Schema 表 Pipeline 执行失败: %v", err)
	}

	var count int
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM " + schemaTable).Scan(&count); err != nil {
		t.Fatalf("查询 schema 表失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("Schema 表行数 = %d, 期望 2", count)
	}
	t.Logf("✅ Schema 表测试通过：%s 写入 %d 行", schemaTable, count)

	pgDB.Exec("DROP TABLE IF EXISTS " + schemaTable)
}

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, 期望 %v", name, got, want)
	}
}
