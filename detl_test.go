package detl

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iotames/detl/internal/engine"
	"github.com/iotames/detl/internal/load"
	"github.com/iotames/detl/internal/source"
	"github.com/iotames/detl/internal/transform"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

const (
	testDriver = "postgres"
	testDSN    = "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable search_path=public"
	testTable  = "detl_test_users"
)

// 建表并插入测试数据
func setupTestData(t *testing.T, db *sql.DB) {
	t.Helper()
	dropSQL := `DROP TABLE IF EXISTS ` + testTable + `;`
	createSQL := `CREATE TABLE ` + testTable + ` (
		id SERIAL PRIMARY KEY,
		first_name VARCHAR(50),
		last_name VARCHAR(50),
		email VARCHAR(100),
		age INT,
		created_at DATE
	);`
	insertSQL := `INSERT INTO ` + testTable + ` (first_name, last_name, email, age, created_at) VALUES
		('John', 'Doe', 'john@example.com', 30, '2024-01-15'),
		('Jane', 'Smith', 'JANE@EXAMPLE.COM', 25, '2024-02-20'),
		('Bob', NULL, 'bob@test.com', NULL, '2024-03-10'),
		('Alice', 'Wang', 'alice.wang@TEST.COM', 28, '2024-04-05'),
		(NULL, 'Lee', 'null_first@example.com', 35, '2024-05-01');`

	for _, q := range []string{dropSQL, createSQL, insertSQL} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("setup 失败: %v\nSQL: %s", err, q)
		}
	}
	t.Log("测试数据已准备")
}

func TestPipeline_PG_to_CSV(t *testing.T) {
	// 1. 连接 PG
	db, err := sql.Open(testDriver, testDSN)
	if err != nil {
		t.Fatalf("连接 PG 失败: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("PG 不可用，跳过测试: %v", err)
	}
	defer db.Close()

	// 2. 建表并准备数据
	setupTestData(t, db)
	defer func() {
		db.Exec(`DROP TABLE IF EXISTS ` + testTable)
	}()

	// 3. 定义 Source：从 PG 抽取
	src := source.NewSQL(source.SQLConfig{
		Driver: testDriver,
		DSN:    testDSN,
		Query:  `SELECT id, first_name, last_name, email, age, created_at FROM ` + testTable + ` ORDER BY id`,
	})

	// 4. 定义 Transform：清洗转换
	tf := transform.Func(func(row map[string]any) ([]map[string]any, error) {
		// 4a. 拼接 full_name（处理 NULL）
		fn := nullStr(row["first_name"])
		ln := nullStr(row["last_name"])
		fullName := fn
		if ln != "" {
			if fullName != "" {
				fullName += " "
			}
			fullName += ln
		}

		// 4b. email 转小写
		email := lower(nullStr(row["email"]))

		// 4c. age 默认 0
		age := 0
		if v, ok := row["age"]; ok && v != nil {
			switch a := v.(type) {
			case int64:
				age = int(a)
			case int:
				age = a
			}
		}

		// 4d. 格式化日期
		createdAt := ""
		if t, ok := row["created_at"].(time.Time); ok {
			createdAt = t.Format("2006-01-02")
		} else {
			createdAt = nullStr(row["created_at"])
		}

		// 4e. 组装新行
		return []map[string]any{{
			"id":         row["id"],
			"full_name":  fullName,
			"email":      email,
			"age":        age,
			"created_at": createdAt,
			"source":     "postgres",
		}}, nil
	})

	// 5. 定义 Load：写入 CSV
	outPath := filepath.Join(t.TempDir(), "users_output.csv")
	ld := load.NewCSV(load.CSVConfig{
		Path:    outPath,
		Columns: []string{"id", "full_name", "email", "age", "created_at", "source"},
	})

	// 6. 执行 Pipeline
	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		t.Fatalf("Pipeline 执行失败: %v", err)
	}

	// 7. 验证 CSV 输出
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读取 CSV 失败: %v", err)
	}
	t.Logf("CSV 输出:\n%s", string(data))

	// 验证内容
	content := string(data)
	assertContains(t, content, "id,full_name,email,age,created_at,source")
	assertContains(t, content, "John Doe")
	assertContains(t, content, "jane@example.com") // 小写
	assertContains(t, content, "bob@test.com")
	assertContains(t, content, "alice.wang@test.com") // 小写
	assertContains(t, content, "0")                   // NULL age 转 0
	assertContains(t, content, "Bob")                 // last_name 为 NULL 时 full_name 只有 first_name
	assertContains(t, content, "Lee")                 // first_name 为 NULL 时 full_name 只有 last_name
	assertContains(t, content, "postgres")            // source 标记

	t.Log("✅ 测试通过：ETL Pipeline（PG → Transform → CSV）运行成功")
}

// --- MySQL 测试 ---

const (
	testMySQLDriver = "mysql"
	testMySQLDSN    = "root:root@tcp(127.0.0.1:3306)/detl_test?timeout=5s&charset=utf8mb4"
	testMySQLTable  = "detl_test_users"
)

func setupMySQLData(t *testing.T, db *sql.DB) {
	t.Helper()
	dropSQL := `DROP TABLE IF EXISTS ` + testMySQLTable + `;`
	createSQL := `CREATE TABLE ` + testMySQLTable + ` (
		id INT AUTO_INCREMENT PRIMARY KEY,
		first_name VARCHAR(50),
		last_name VARCHAR(50),
		email VARCHAR(100),
		age INT,
		created_at DATE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	insertSQL := `INSERT INTO ` + testMySQLTable + ` (first_name, last_name, email, age, created_at) VALUES
		('John', 'Doe', 'john@example.com', 30, '2024-01-15'),
		('Jane', 'Smith', 'JANE@EXAMPLE.COM', 25, '2024-02-20'),
		('Bob', NULL, 'bob@test.com', NULL, '2024-03-10'),
		('Alice', 'Wang', 'alice.wang@TEST.COM', 28, '2024-04-05'),
		(NULL, 'Lee', 'null_first@example.com', 35, '2024-05-01');`

	for _, q := range []string{dropSQL, createSQL, insertSQL} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("MySQL setup 失败: %v\nSQL: %s", err, q)
		}
	}
	t.Log("MySQL 测试数据已准备")
}

func TestPipeline_MySQL_to_CSV(t *testing.T) {
	// 1. 连接 MySQL
	db, err := sql.Open(testMySQLDriver, testMySQLDSN)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("MySQL 不可用，跳过测试: %v", err)
	}
	defer db.Close()

	// 2. 建表并准备数据
	setupMySQLData(t, db)

	// 3. 定义 Source：从 MySQL 抽取
	src := source.NewSQL(source.SQLConfig{
		Driver: testMySQLDriver,
		DSN:    testMySQLDSN,
		Query:  `SELECT id, first_name, last_name, email, age, created_at FROM ` + testMySQLTable + ` ORDER BY id`,
	})

	// 4. 定义 Transform：清洗转换
	tf := transform.Func(func(row map[string]any) ([]map[string]any, error) {
		fn := nullStr(row["first_name"])
		ln := nullStr(row["last_name"])
		fullName := fn
		if ln != "" {
			if fullName != "" {
				fullName += " "
			}
			fullName += ln
		}
		email := lower(nullStr(row["email"]))

		age := 0
		if v, ok := row["age"]; ok && v != nil {
			switch a := v.(type) {
			case int64:
				age = int(a)
			case int:
				age = a
			}
		}

		createdAt := ""
		if t, ok := row["created_at"].(time.Time); ok {
			createdAt = t.Format("2006-01-02")
		} else {
			createdAt = nullStr(row["created_at"])
		}

		return []map[string]any{{
			"id":         row["id"],
			"full_name":  fullName,
			"email":      email,
			"age":        age,
			"created_at": createdAt,
			"source":     "mysql",
		}}, nil
	})

	// 5. CSV 输出
	outPath := filepath.Join(t.TempDir(), "mysql_users_output.csv")
	ld := load.NewCSV(load.CSVConfig{
		Path:    outPath,
		Columns: []string{"id", "full_name", "email", "age", "created_at", "source"},
	})

	// 6. 执行 Pipeline
	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		t.Fatalf("MySQL Pipeline 执行失败: %v", err)
	}

	// 7. 验证
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读取 CSV 失败: %v", err)
	}
	t.Logf("MySQL CSV 输出:\n%s", string(data))

	content := string(data)
	assertContains(t, content, "id,full_name,email,age,created_at,source")
	assertContains(t, content, "John Doe")
	assertContains(t, content, "jane@example.com")
	assertContains(t, content, "alice.wang@test.com")
	assertContains(t, content, "0")
	assertContains(t, content, "Bob")
	assertContains(t, content, "Lee")
	assertContains(t, content, "mysql")

	t.Log("✅ 测试通过：ETL Pipeline（MySQL → Transform → CSV）运行成功")
}

// --- 辅助函数 ---

func nullStr(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprintf("%v", s)
	}
}

func lower(s string) string {
	if s == "" {
		return ""
	}
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !contains(s, substr) {
		t.Errorf("期望包含 %q，但未找到\n完整内容:\n%s", substr, s)
	}
}

func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
