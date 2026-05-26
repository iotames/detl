package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/iotames/detl"
	"github.com/iotames/detl/internal/engine"
	"github.com/iotames/detl/internal/load"
	"github.com/iotames/detl/internal/source"
	"github.com/iotames/detl/internal/transform"
	pkgdsn "github.com/iotames/easydb/dsn"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func getActiveDSN() string {
	dsnconf := pkgdsn.GetDsnConf(nil)
	if dsnconf == nil {
		return ""
	}
	dgp := pkgdsn.DsnGroup{}
	if err := dsnconf.GetDsnGroup(&dgp); err != nil {
		log.Printf("读取 DSN 配置失败: %v", err)
		return ""
	}
	for _, ds := range dgp.DsnList {
		if ds.DriverName == DbDriver {
			return ds.Dsn
		}
	}
	return ""
}

func getSQLText(filename string) string {
	sqltxt, err := detl.GetSqlText(cf, filename)
	if err != nil {
		log.Fatalf("读取脚本 %s 失败: %v", filename, err)
	}
	return sqltxt
}

func parseCSVColumns(s string) []string {
	if s == "" {
		return nil
	}
	cols := strings.Split(s, ",")
	for i := range cols {
		cols[i] = strings.TrimSpace(cols[i])
	}
	return cols
}

// buildTransform 根据 TRANSFORM_MODE 创建转换器
func buildTransform() transform.Transformer {
	switch TransformMode {
	case "none":
		log.Printf("转换模式: none（透传原始数据）")
		return nil
	case "python":
		scriptPath := cf.GetScriptFilePath(TransformScript)
		log.Printf("转换模式: python  脚本=%s", scriptPath)
		return transform.NewPython(transform.PythonConfig{
			ScriptPath: scriptPath,
		})
	case "builtin":
		log.Printf("转换模式: builtin（内置清洗：姓名拼接、邮箱小写、NULL处理、日期格式化）")
		return builtinTransform()
	default:
		log.Printf("转换模式: %s（未知，回退 builtin）", TransformMode)
		return builtinTransform()
	}
}

func builtinTransform() transform.Transformer {
	return transform.Func(func(row map[string]any) ([]map[string]any, error) {
		fn := fmt.Sprintf("%v", row["first_name"])
		ln := fmt.Sprintf("%v", row["last_name"])
		if fn == "<nil>" {
			fn = ""
		}
		if ln == "<nil>" {
			ln = ""
		}
		fullName := fn
		if ln != "" {
			if fullName != "" {
				fullName += " "
			}
			fullName += ln
		}

		email := fmt.Sprintf("%v", row["email"])
		if email != "<nil>" {
			email = toLower(email)
		} else {
			email = ""
		}

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
			createdAt = fmt.Sprintf("%v", row["created_at"])
			if createdAt == "<nil>" {
				createdAt = ""
			}
		}

		return []map[string]any{{
			"id":         row["id"],
			"full_name":  fullName,
			"email":      email,
			"age":        age,
			"created_at": createdAt,
			"source":     DbDriver,
			"etl_time":   time.Now().Format("2006-01-02 15:04:05"),
		}}, nil
	})
}

// buildLoad 根据 LOAD_TYPE 创建输出端
func buildLoad() (load.Load, error) {
	cols := parseCSVColumns(OutputColumns)

	switch LoadType {
	case "csv":
		outPath := filepath.Join(OutputDir, OutputFile)
		log.Printf("输出类型: csv  路径=%s  列=%v", outPath, cols)
		return load.NewCSV(load.CSVConfig{
			Path:    outPath,
			Columns: cols,
		}), nil
	case "stdout":
		log.Printf("输出类型: stdout  列=%v", cols)
		return load.NewStdout(cols), nil
	default:
		return nil, fmt.Errorf("不支持的输出类型: %s（可选 csv/stdout）", LoadType)
	}
}

// runETL 通过环境变量/配置文件驱动的 ETL 流程
func runETL() error {
	dsn := getActiveDSN()
	if dsn == "" {
		return fmt.Errorf("未找到驱动 %s 的 DSN 配置（请检查 CONF_DIR/dsn.json）", DbDriver)
	}
	sql := getSQLText(ScriptFile)
	log.Printf("Source: 驱动=%s  脚本=%s", DbDriver, ScriptFile)

	// Source
	src := source.NewSQL(source.SQLConfig{
		Driver: DbDriver,
		DSN:    dsn,
		Query:  sql,
	})

	// Transform
	tf := buildTransform()

	// Load
	ld, err := buildLoad()
	if err != nil {
		return err
	}

	// Pipeline
	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		return fmt.Errorf("ETL 执行失败: %w", err)
	}

	if LoadType == "csv" {
		outPath := filepath.Join(OutputDir, OutputFile)
		fmt.Printf("输出文件: %s\n", outPath)
	}
	return nil
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
