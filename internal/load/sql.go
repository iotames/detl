package load

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// SQLConfig 数据库写入配置
type SQLConfig struct {
	Driver      string   // 数据库驱动（postgres / mysql）
	DSN         string   // 连接字符串
	Table       string   // 目标表（支持 schema.table）
	Mode        string   // insert | upsert
	KeyColumns  []string // upsert 唯一键
	CreateTable bool     // 自动建表
	BatchSize   int      // 每批写入行数
}

type sqlLoad struct {
	cfg    SQLConfig
	db     *sql.DB
	schema string // 解析后的 schema，空则默认
	table  string // 解析后的表名
	buf    []map[string]any
	cols   []string // 列名（首次写入时确定）
}

// NewSQL 创建数据库写入器
func NewSQL(cfg SQLConfig) Load {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	return &sqlLoad{cfg: cfg}
}

func (l *sqlLoad) Open() error {
	db, err := sql.Open(l.cfg.Driver, l.cfg.DSN)
	if err != nil {
		return fmt.Errorf("load sql open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("load sql ping: %w", err)
	}
	l.db = db
	l.schema, l.table = parseTableName(l.cfg.Table)
	return nil
}

func (l *sqlLoad) Write(row map[string]any) error {
	if len(l.cols) == 0 {
		// 首次写入：确定列名
		l.cols = make([]string, 0, len(row))
		for k := range row {
			l.cols = append(l.cols, k)
		}
		// 统一列顺序，避免每次 map 遍历不一致
		sortStrings(l.cols)

		if l.cfg.CreateTable {
			if err := l.createTable(); err != nil {
				return fmt.Errorf("load sql create table: %w", err)
			}
		}
	}
	l.buf = append(l.buf, row)
	if len(l.buf) >= l.cfg.BatchSize {
		return l.flush()
	}
	return nil
}

func (l *sqlLoad) Close() error {
	if l.db == nil {
		return nil
	}
	if len(l.buf) > 0 {
		if err := l.flush(); err != nil {
			l.db.Close()
			return err
		}
	}
	return l.db.Close()
}

func (l *sqlLoad) flush() error {
	if len(l.buf) == 0 {
		return nil
	}
	sqlStr, args := l.buildBatchSQL()
	log.Printf("SQL Load: 写入 %d 行到 %s", len(l.buf), l.quotedTable())
	_, err := l.db.Exec(sqlStr, args...)
	if err != nil {
		return fmt.Errorf("load sql exec: %w\nSQL: %s", err, sqlStr)
	}
	l.buf = l.buf[:0]
	return nil
}

// buildBatchSQL 构建批量 INSERT/UPSERT SQL
func (l *sqlLoad) buildBatchSQL() (string, []any) {
	nCols := len(l.cols)
	nRows := len(l.buf)
	placeholders := make([]string, nRows)
	args := make([]any, 0, nCols*nRows)

	for i, row := range l.buf {
		rowPh := make([]string, nCols)
		for j, col := range l.cols {
			if l.cfg.Driver == "postgres" {
				rowPh[j] = fmt.Sprintf("$%d", i*nCols+j+1)
			} else {
				rowPh[j] = "?"
			}
			args = append(args, formatValue(row[col]))
		}
		placeholders[i] = "(" + strings.Join(rowPh, ",") + ")"
	}

	cols := make([]string, nCols)
	for i, c := range l.cols {
		cols[i] = l.quoteIdent(c)
	}

	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		l.quotedTable(),
		strings.Join(cols, ","),
		strings.Join(placeholders, ","))

	if l.cfg.Mode == "upsert" {
		sqlStr += l.buildUpsertSuffix()
	}

	return sqlStr, args
}

func (l *sqlLoad) buildUpsertSuffix() string {
	if l.cfg.Driver == "postgres" {
		// ON CONFLICT (id) DO UPDATE SET col1=EXCLUDED.col1, col2=EXCLUDED.col2
		keys := make([]string, len(l.cfg.KeyColumns))
		for i, k := range l.cfg.KeyColumns {
			keys[i] = l.quoteIdent(k)
		}
		sets := make([]string, 0, len(l.cols))
		for _, c := range l.cols {
			sets = append(sets, fmt.Sprintf("%s=EXCLUDED.%s", l.quoteIdent(c), l.quoteIdent(c)))
		}
		return fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s",
			strings.Join(keys, ","), strings.Join(sets, ","))
	}

	// MySQL: ON DUPLICATE KEY UPDATE col1=VALUES(col1), col2=VALUES(col2)
	sets := make([]string, 0, len(l.cols))
	for _, c := range l.cols {
		sets = append(sets, fmt.Sprintf("%s=VALUES(%s)", l.quoteIdent(c), l.quoteIdent(c)))
	}
	return " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ",")
}

// createTableSQL 生成建表 DDL（所有列使用 TEXT 类型）
func (l *sqlLoad) createTableSQL() string {
	colDefs := make([]string, len(l.cols))
	for i, c := range l.cols {
		colDefs[i] = fmt.Sprintf("%s TEXT", l.quoteIdent(c))
	}
	// UPSERT 模式：添加唯一约束
	if l.cfg.Mode == "upsert" && len(l.cfg.KeyColumns) > 0 {
		keys := make([]string, len(l.cfg.KeyColumns))
		for i, k := range l.cfg.KeyColumns {
			keys[i] = l.quoteIdent(k)
		}
		colDefs = append(colDefs, fmt.Sprintf("UNIQUE (%s)", strings.Join(keys, ",")))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", l.quotedTable(), strings.Join(colDefs, ","))
}

// createTable 自动建表
func (l *sqlLoad) createTable() error {
	_, err := l.db.Exec(l.createTableSQL())
	return err
}

// parseTableName 解析表名，提取 schema 和 table
func parseTableName(fullTable string) (schema, table string) {
	if parts := strings.SplitN(fullTable, ".", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", fullTable
}

func (l *sqlLoad) quotedTable() string {
	if l.schema != "" {
		return l.quoteIdent(l.schema) + "." + l.quoteIdent(l.table)
	}
	return l.quoteIdent(l.table)
}

func (l *sqlLoad) quoteIdent(name string) string {
	if l.cfg.Driver == "postgres" {
		return `"` + name + `"`
	}
	return "`" + name + "`"
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
