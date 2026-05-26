package load

import (
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"sort"
)

// CSVConfig CSV 文件输出配置
type CSVConfig struct {
	Path    string   // 输出文件路径
	Columns []string // 列名顺序，为空则按首次写入的 map key 排序
}

type csvLoad struct {
	cfg    CSVConfig
	file   *os.File
	writer *csv.Writer
	wrote  bool // 是否已写入表头
}

// NewCSV 创建 CSV 文件写入器
func NewCSV(cfg CSVConfig) Load {
	return &csvLoad{cfg: cfg}
}

func (l *csvLoad) Open() error {
	f, err := os.Create(l.cfg.Path)
	if err != nil {
		return fmt.Errorf("load csv create: %w", err)
	}
	l.file = f
	l.writer = csv.NewWriter(f)
	return nil
}

func (l *csvLoad) Write(row map[string]any) error {
	cols := l.cfg.Columns
	if !l.wrote {
		// 首次写入：决定列顺序并写表头
		if len(cols) == 0 {
			cols = make([]string, 0, len(row))
			for k := range row {
				cols = append(cols, k)
			}
			sort.Strings(cols)
		}
		if err := l.writer.Write(cols); err != nil {
			return fmt.Errorf("load csv header: %w", err)
		}
		l.wrote = true
	}

	record := make([]string, len(cols))
	for i, col := range cols {
		record[i] = formatValue(row[col])
	}
	if err := l.writer.Write(record); err != nil {
		return fmt.Errorf("load csv row: %w", err)
	}
	return nil
}

func (l *csvLoad) Close() error {
	if l.writer != nil {
		l.writer.Flush()
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return ""
		}
		return formatValue(rv.Elem().Interface())
	case reflect.String:
		return rv.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", rv.Int())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%v", rv.Float())
	case reflect.Bool:
		return fmt.Sprintf("%t", rv.Bool())
	default:
		return fmt.Sprintf("%v", v)
	}
}
