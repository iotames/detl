package load

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
)

type stdoutLoad struct {
	writer  *csv.Writer
	cols    []string
	wrote   bool
}

// NewStdout 创建控制台输出（以 CSV 格式打印）
func NewStdout(cols []string) Load {
	return &stdoutLoad{cols: cols}
}

func (l *stdoutLoad) Open() error {
	l.writer = csv.NewWriter(os.Stdout)
	return nil
}

func (l *stdoutLoad) Write(row map[string]any) error {
	cols := l.cols
	if !l.wrote {
		if len(cols) == 0 {
			cols = make([]string, 0, len(row))
			for k := range row {
				cols = append(cols, k)
			}
			sort.Strings(cols)
		}
		if err := l.writer.Write(cols); err != nil {
			return fmt.Errorf("stdout header: %w", err)
		}
		l.cols = cols
		l.wrote = true
	}

	record := make([]string, len(cols))
	for i, col := range cols {
		record[i] = formatValue(row[col])
	}
	if err := l.writer.Write(record); err != nil {
		return fmt.Errorf("stdout row: %w", err)
	}
	l.writer.Flush()
	return nil
}

func (l *stdoutLoad) Close() error {
	return nil
}
