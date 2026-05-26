package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/iotames/detl"
	"github.com/iotames/detl/internal/engine"
	"github.com/iotames/detl/internal/load"
	"github.com/iotames/detl/internal/source"
	"github.com/iotames/detl/internal/transform"
	pkgdsn "github.com/iotames/easydb/dsn"
	_ "github.com/go-sql-driver/mysql"
)

func getSQLText(filename string) string {
	sqltxt, err := detl.GetSqlText(cf, filename)
	if err != nil {
		log.Fatalf("读取脚本 %s 失败: %v", filename, err)
	}
	return sqltxt
}

// getMySQLDSN 从 DSN 配置中查找 MySQL 连接字符串
func getMySQLDSN() string {
	dsnconf := pkgdsn.GetDsnConf(nil)
	dgp := pkgdsn.DsnGroup{}
	if err := dsnconf.GetDsnGroup(&dgp); err != nil {
		log.Printf("读取 DSN 配置失败: %v", err)
		return ""
	}
	for _, ds := range dgp.DsnList {
		if ds.DriverName == "mysql" {
			return ds.Dsn
		}
	}
	return ""
}

// etlDemo 演示 ETL Hello 流程：
// MySQL(detl_test) → Transform(清洗转换) → CSV(文件输出)
func etlDemo() error {
	// 1. 从配置中获取 MySQL DSN 和抽取 SQL
	dsn := getMySQLDSN()
	if dsn == "" {
		return fmt.Errorf("未找到 MySQL DSN 配置")
	}
	sql := getSQLText("e_detl_users.sql")
	log.Printf("MySQL DSN: %s", dsn)
	log.Printf("抽取 SQL: %s", sql)

	// 2. Source：从 MySQL 抽取数据
	src := source.NewSQL(source.SQLConfig{
		Driver: "mysql",
		DSN:    dsn,
		Query:  sql,
	})

	// 3. Transform：清洗转换
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

		email := toLower(nullStr(row["email"]))

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
			"etl_time":   time.Now().Format("2006-01-02 15:04:05"),
		}}, nil
	})

	// 4. Load：写入 CSV
	outPath := filepath.Join(".", "output", "etl_demo.csv")
	ld := load.NewCSV(load.CSVConfig{
		Path:    outPath,
		Columns: []string{"id", "full_name", "email", "age", "created_at", "source", "etl_time"},
	})

	// 5. Pipeline 执行
	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		return fmt.Errorf("ETL demo 执行失败: %w", err)
	}

	fmt.Printf("✅ ETL Hello 完成！输出文件: %s\n", outPath)
	return nil
}

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

func toLower(s string) string {
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
