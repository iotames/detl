package source

import (
	"database/sql"
	"fmt"
)

// SQLConfig 数据库源配置
type SQLConfig struct {
	Driver string // "postgres" 或 "mysql"
	DSN    string
	Query  string
}

type sqlSource struct {
	cfg  SQLConfig
	db   *sql.DB
	rows *sql.Rows
	cols []string
}

// NewSQL 创建 SQL 数据源
func NewSQL(cfg SQLConfig) Source {
	return &sqlSource{cfg: cfg}
}

func (s *sqlSource) Open() error {
	db, err := sql.Open(s.cfg.Driver, s.cfg.DSN)
	if err != nil {
		return fmt.Errorf("source sql open: %w", err)
	}
	if err = db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("source sql ping: %w", err)
	}
	s.db = db

	rows, err := db.Query(s.cfg.Query)
	if err != nil {
		db.Close()
		return fmt.Errorf("source sql query: %w", err)
	}
	s.rows = rows
	s.cols, err = rows.Columns()
	if err != nil {
		rows.Close()
		db.Close()
		return fmt.Errorf("source sql columns: %w", err)
	}
	return nil
}

func (s *sqlSource) Read() (map[string]any, bool) {
	if s.rows == nil || !s.rows.Next() {
		return nil, false
	}

	values := make([]any, len(s.cols))
	valuePtrs := make([]any, len(s.cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := s.rows.Scan(valuePtrs...); err != nil {
		return nil, false
	}

	row := make(map[string]any, len(s.cols))
	for i, col := range s.cols {
		val := values[i]
		// 将 []byte 转为 string
		if b, ok := val.([]byte); ok {
			val = string(b)
		}
		row[col] = val
	}
	return row, true
}

func (s *sqlSource) Close() error {
	if s.rows != nil {
		s.rows.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
