package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/iotames/detl/hotswap"
	"github.com/iotames/detl/internal/engine"
	"github.com/iotames/detl/internal/load"
	"github.com/iotames/detl/internal/source"
	"github.com/iotames/detl/internal/task"
	"github.com/iotames/detl/internal/transform"
	pkgdsn "github.com/iotames/easydb/dsn"
	_ "github.com/lib/pq"
)

func getActiveDSN() string {
	ds, ok := cf.GetDSNByDriver(DbDriver)
	if ok {
		return ds.Dsn
	}
	// dsn.json 中无匹配时回退到 ACTIVE_DSN 环境变量
	if ActiveDsn != "" {
		log.Printf("dsn.json 中未找到驱动 %s 的配置，使用 ACTIVE_DSN 回退", DbDriver)
		return ActiveDsn
	}
	return ""
}

func getSQLText(filename string) (string, error) {
	content, err := hotswap.GetScriptDir().GetScriptBytes(filename)
	if err != nil {
		return "", fmt.Errorf("读取脚本 %s 失败: %w", filename, err)
	}
	return string(content), nil
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

// pipelineCfg 承载一次 ETL 执行的全部配置
type pipelineCfg struct {
	Driver, DSN, Query string

	TransformMode   string
	TransformScript string
	TransformEnv    map[string]string

	LoadType    string
	LoadFile    string
	LoadColumns []string

	LoadDSN, LoadDriver       string
	LoadTable, LoadMode       string
	LoadKeyColumns            []string
	LoadCreateTable           bool
	LoadBatchSize             int
}

// runPipeline 组装并执行一次 ETL 流程
func runPipeline(cfg pipelineCfg) error {
	// --- Source ---
	src := source.NewSQL(source.SQLConfig{
		Driver: cfg.Driver, DSN: cfg.DSN, Query: cfg.Query,
	})

	// --- Transform ---
	var tf transform.Transformer
	switch cfg.TransformMode {
	case "none":
		log.Printf("转换模式: none（透传原始数据）")
	case "python":
		scriptPath := hotswap.GetScriptDir().GetScriptPath(cfg.TransformScript)
		if scriptPath == "" {
			scriptPath = cf.GetScriptFilePath(cfg.TransformScript)
		}
		log.Printf("转换模式: python  脚本=%s", scriptPath)
		tf = transform.NewPython(transform.PythonConfig{
			ScriptPath: scriptPath,
			Env:        cfg.TransformEnv,
		})
	default:
		log.Printf("转换模式: builtin（通用透传）")
		tf = builtinTransform()
	}

	// --- Load ---
	var ld load.Load
	switch cfg.LoadType {
	case "csv":
		outPath := cfg.LoadFile
		if !filepath.IsAbs(outPath) {
			outPath = filepath.Join(OutputDir, outPath)
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("创建输出目录失败: %w", err)
		}
		log.Printf("输出类型: csv  路径=%s  列=%v", outPath, cfg.LoadColumns)
		ld = load.NewCSV(load.CSVConfig{Path: outPath, Columns: cfg.LoadColumns})
	case "stdout":
		log.Printf("输出类型: stdout  列=%v", cfg.LoadColumns)
		ld = load.NewStdout(cfg.LoadColumns)
	case "sql":
		dsn := cfg.LoadDSN
		if dsn == "" {
			dsn = cfg.DSN
		}
		driver := cfg.LoadDriver
		if driver == "" {
			driver = cfg.Driver
		}
		if dsn == "" {
			return fmt.Errorf("SQL Load 需要有效的 DSN 配置")
		}
		batchSize := cfg.LoadBatchSize
		if batchSize <= 0 {
			batchSize = 50
		}
		log.Printf("输出类型: sql  驱动=%s  表=%s  模式=%s", driver, cfg.LoadTable, cfg.LoadMode)
		ld = load.NewSQL(load.SQLConfig{
			Driver: driver, DSN: dsn, Table: cfg.LoadTable,
			Mode: cfg.LoadMode, KeyColumns: cfg.LoadKeyColumns,
			CreateTable: cfg.LoadCreateTable, BatchSize: batchSize,
		})
	default:
		return fmt.Errorf("不支持的输出类型: %q（可选 csv/stdout/sql）", cfg.LoadType)
	}

	p := engine.New(src, tf, ld)
	if err := p.Run(); err != nil {
		return fmt.Errorf("ETL 执行失败: %w", err)
	}
	if cfg.LoadType == "csv" {
		log.Printf("输出文件: %s", cfg.LoadFile)
	}
	return nil
}

func builtinTransform() transform.Transformer {
	return transform.Func(func(row map[string]any) ([]map[string]any, error) {
		result := make(map[string]any, len(row))
		for k, v := range row {
			if v == nil {
				result[k] = nil
				continue
			}
			if b, ok := v.([]byte); ok {
				result[k] = string(b)
				continue
			}
			result[k] = v
		}
		return []map[string]any{result}, nil
	})
}

// runETL 通过环境变量/配置文件驱动的 ETL 流程
// 文件名、列名等任务细节使用固定默认值，自定义配置请使用 YAML 任务模式
func runETL() error {
	dsn := getActiveDSN()
	if dsn == "" {
		return fmt.Errorf("未找到驱动 %s 的 DSN 配置（请检查 CONF_DIR/dsn.json）", DbDriver)
	}
	sqlText, err := getSQLText(ScriptFile)
	if err != nil {
		return err
	}
	log.Printf("Source: 驱动=%s  脚本=%s", DbDriver, ScriptFile)
	outputFile := "etl_output.csv"
	outputColumns := []string{"id", "full_name", "email", "age", "created_at", "source", "etl_time"}
	cfg := pipelineCfg{
		Driver: DbDriver, DSN: dsn, Query: sqlText,
		TransformMode: TransformMode, TransformScript: TransformScript,
		LoadType: LoadType, LoadFile: outputFile,
		LoadColumns: outputColumns,
	}
	if LoadType == "sql" {
		cfg.LoadTable = outputFile
		cfg.LoadMode = "upsert"
	}
	return runPipeline(cfg)
}

// runETLFromTask 从 YAML 任务文件执行 ETL
func runETLFromTask(taskPath string) error {
	t, err := task.LoadTask(taskPath)
	if err != nil {
		return err
	}
	log.Printf("加载任务: kind=%s  name=%s  file=%s", t.Kind, t.Name, taskPath)

	if t.IsJob() {
		return runJob(taskPath, t, make(map[string]bool))
	}
	return runTask(t)
}

// runTask 执行单个转换任务
func runTask(t *task.TaskConfig) error {
	ds, ok := cf.GetDSNByName(t.Source.Connection)
	if !ok {
		ds, ok = cf.GetDSNByDriver(t.Source.Connection)
		if !ok {
			return fmt.Errorf("未找到连接 %q（请检查 dsn.json）", t.Source.Connection)
		}
		log.Printf("按驱动名回退匹配到连接: %s", ds.DriverName)
	}

	sqlText := t.Source.Query
	if sqlText == "" && t.Source.QueryFile != "" {
		var err error
		sqlText, err = getSQLText(t.Source.QueryFile)
		if err != nil {
			return err
		}
	}
	if sqlText == "" {
		return fmt.Errorf("source 未指定 query 或 query_file")
	}
	log.Printf("Source: 连接=%s  驱动=%s", t.Source.Connection, ds.DriverName)

	transformMode := "builtin"
	transformScript := ""
	var transformEnv map[string]string
	if t.Transform != nil {
		transformMode = t.Transform.Mode
		transformScript = t.Transform.Script
		transformEnv = t.Transform.Env
	}

	if t.Load == nil {
		return fmt.Errorf("转换任务必须包含 load 配置")
	}

	cfg := pipelineCfg{
		Driver: ds.DriverName, DSN: ds.Dsn, Query: sqlText,
		TransformMode: transformMode, TransformScript: transformScript,
		TransformEnv:          transformEnv,
		LoadType:              t.Load.Type,
		LoadFile:              t.Load.File,
		LoadColumns:           t.Load.Columns,
		LoadTable:             t.Load.Table,
		LoadMode:              t.Load.Mode,
		LoadKeyColumns:        t.Load.KeyColumns,
		LoadCreateTable:       t.Load.CreateTable,
		LoadBatchSize:         t.Load.BatchSize,
	}

	if t.Load.Type == "sql" && t.Load.Connection != "" {
		ds2, ok := cf.GetDSNByName(t.Load.Connection)
		if !ok {
			ds2, ok = cf.GetDSNByDriver(t.Load.Connection)
			if !ok {
				return fmt.Errorf("SQL Load 未找到连接 %q（请检查 dsn.json）", t.Load.Connection)
			}
		}
		cfg.LoadDSN = ds2.Dsn
		cfg.LoadDriver = ds2.DriverName
	}

	return runPipeline(cfg)
}

// runJob 执行作业：按顺序依次执行子任务，带循环引用检测
func runJob(parentPath string, j *task.TaskConfig, visited map[string]bool) error {
	parentDir := filepath.Dir(parentPath)
	log.Printf("作业 %q 开始执行，共 %d 个子任务", j.Name, len(j.Tasks))

	absPath, _ := filepath.Abs(parentPath)
	if visited[absPath] {
		return fmt.Errorf("检测到作业循环引用: %s", parentPath)
	}
	visited[absPath] = true

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

// runDBTest 测试数据库连通性
func runDBTest(dsnName string) error {
	dgp, err := cf.GetDSNGroup()
	if err != nil {
		return fmt.Errorf("读取 DSN 配置失败: %w", err)
	}

	list := dgp.DsnList

	// 如果指定了连接名，只测试指定的那个
	if dsnName != "" {
		ds, ok := dgp.GetDSNByName(dsnName)
		if !ok {
			return fmt.Errorf("未找到连接名为 %q 的数据源", dsnName)
		}
		list = []pkgdsn.DataSource{ds}
	}

	for _, ds := range list {
		fmt.Printf("\n--- 测试连接: %s (%s) ---", ds.Name, ds.DriverName)
		if err := pingDS(ds.DriverName, ds.Dsn); err != nil {
			fmt.Printf("  ❌ 失败: %v\n", err)
		} else {
			fmt.Printf("  ✅ 成功\n")
		}
	}
	return nil
}

func pingDS(driver, dsn string) error {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}
