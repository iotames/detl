package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iotames/detl"
	"github.com/iotames/detl/internal/engine"
	"github.com/iotames/detl/internal/load"
	"github.com/iotames/detl/internal/source"
	"github.com/iotames/detl/internal/task"
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
	case "sql":
		dsn := getActiveDSN()
		if dsn == "" {
			return nil, fmt.Errorf("SQL Load 需要有效的 DSN 配置")
		}
		log.Printf("输出类型: sql  驱动=%s  表=%s", DbDriver, OutputFile)
		return load.NewSQL(load.SQLConfig{
			Driver:      DbDriver,
			DSN:         dsn,
			Table:       OutputFile,
			Mode:        TransformMode,
			CreateTable: true,
			BatchSize:   50,
		}), nil
	default:
		return nil, fmt.Errorf("不支持的输出类型: %s（可选 csv/stdout/sql）", LoadType)
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

// runETLFromTask 从 YAML 任务文件执行 ETL
func runETLFromTask(taskPath string) error {
	taskPath = resolveTaskPath(taskPath, cf.GetScriptDir())
	t, err := task.LoadTask(taskPath)
	if err != nil {
		return err
	}
	log.Printf("加载任务: kind=%s  name=%s  file=%s", t.Kind, t.Name, taskPath)

	if t.IsJob() {
		return runJob(taskPath, t)
	}

	if t.Source == nil || t.Load == nil {
		return fmt.Errorf("转换任务必须包含 source 和 load 配置")
	}

	// --- Source ---
	ds, ok := cf.GetDSNByName(t.Source.Connection)
	if !ok {
		// 回退：按驱动名查找（兼容无 Name 的旧 dsn.json）
		ds, ok = cf.GetDSNByDriver(t.Source.Connection)
		if !ok {
			return fmt.Errorf("未找到连接 %q（请检查 dsn.json）", t.Source.Connection)
		}
		log.Printf("按驱动名回退匹配到连接: %s", ds.DriverName)
	}

	sqlText := t.Source.Query
	if sqlText == "" && t.Source.QueryFile != "" {
		sqlText = getSQLText(t.Source.QueryFile)
	}
	if sqlText == "" {
		return fmt.Errorf("source 未指定 query 或 query_file")
	}

	log.Printf("Source: 连接=%s  驱动=%s", t.Source.Connection, ds.DriverName)
	src := source.NewSQL(source.SQLConfig{
		Driver: ds.DriverName,
		DSN:    ds.Dsn,
		Query:  sqlText,
	})

	// --- Transform ---
	var tf transform.Transformer
	if t.Transform != nil {
		switch t.Transform.Mode {
		case "none":
			log.Printf("转换模式: none（透传原始数据）")
		case "python":
			scriptPath := cf.GetScriptFilePath(t.Transform.Script)
			log.Printf("转换模式: python  脚本=%s  附加环境变量=%v", scriptPath, t.Transform.Env)
			tf = transform.NewPython(transform.PythonConfig{
				ScriptPath: scriptPath,
				Env:        t.Transform.Env,
			})
		default:
			log.Printf("转换模式: builtin（内置清洗）")
			tf = builtinTransform()
		}
	}

	// --- Load ---
	var ld load.Load
	switch t.Load.Type {
	case "csv":
		outPath := t.Load.File
		if !filepath.IsAbs(outPath) {
			outPath = filepath.Join(OutputDir, outPath)
		}
		log.Printf("输出类型: csv  路径=%s  列=%v", outPath, t.Load.Columns)
		ld = load.NewCSV(load.CSVConfig{
			Path:    outPath,
			Columns: t.Load.Columns,
		})
	case "stdout":
		log.Printf("输出类型: stdout  列=%v", t.Load.Columns)
		ld = load.NewStdout(t.Load.Columns)
	case "sql":
		ds, ok := cf.GetDSNByName(t.Load.Connection)
		if !ok {
			ds, ok = cf.GetDSNByDriver(t.Load.Connection)
			if !ok {
				return fmt.Errorf("SQL Load 未找到连接 %q（请检查 dsn.json）", t.Load.Connection)
			}
		}
		batchSize := t.Load.BatchSize
		if batchSize <= 0 {
			batchSize = 50
		}
		log.Printf("输出类型: sql  目标=%s.%s  模式=%s  批量=%d",
			ds.DriverName, t.Load.Table, t.Load.Mode, batchSize)
		ld = load.NewSQL(load.SQLConfig{
			Driver:      ds.DriverName,
			DSN:         ds.Dsn,
			Table:       t.Load.Table,
			Mode:        t.Load.Mode,
			KeyColumns:  t.Load.KeyColumns,
			CreateTable: t.Load.CreateTable,
			BatchSize:   batchSize,
		})
	default:
		return fmt.Errorf("不支持的输出类型: %q（可选 csv/stdout/sql）", t.Load.Type)
	}

	// --- Pipeline ---
	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		return fmt.Errorf("ETL 执行失败: %w", err)
	}

	if t.Load.Type == "csv" {
		log.Printf("输出文件: %s", t.Load.File)
	}
	return nil
}

// runJob 执行作业：按顺序依次执行子任务
func runJob(parentPath string, j *task.TaskConfig) error {
	parentDir := filepath.Dir(parentPath)
	log.Printf("作业 %q 开始执行，共 %d 个子任务", j.Name, len(j.Tasks))

	for i, entry := range j.Tasks {
		childPath := entry.Task
		if !filepath.IsAbs(childPath) {
			childPath = filepath.Join(parentDir, childPath)
		}
		log.Printf("作业 %q [%d/%d]: 执行 %s", j.Name, i+1, len(j.Tasks), childPath)

		if err := runETLFromTask(childPath); err != nil {
			return fmt.Errorf("作业 %q 在任务 %s 失败: %w", j.Name, childPath, err)
		}
	}

	log.Printf("作业 %q 全部执行完成", j.Name)
	return nil
}

// resolveTaskPath 解析任务文件路径。
// 优先级：绝对路径 → 相对路径（存在即用）→ SCRIPT_DIR 下查找
func resolveTaskPath(taskFile, scriptDir string) string {
	if filepath.IsAbs(taskFile) {
		return taskFile
	}
	// 相对路径：当前目录存在则直接用
	if _, err := os.Stat(taskFile); err == nil {
		return taskFile
	}
	// 回退到 SCRIPT_DIR
	return filepath.Join(scriptDir, taskFile)
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
