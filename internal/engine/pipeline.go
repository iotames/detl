// Package engine 提供 ETL Pipeline 编排
package engine

import (
	"fmt"

	"github.com/iotames/detl/internal/load"
	"github.com/iotames/detl/internal/source"
	"github.com/iotames/detl/internal/transform"
)

// Pipeline 编排一个完整的 ETL 流程
type Pipeline struct {
	source      source.Source
	transformer transform.Transformer
	load        load.Load
}

// New 创建 Pipeline
func New(src source.Source, tf transform.Transformer, ld load.Load) *Pipeline {
	return &Pipeline{source: src, transformer: tf, load: ld}
}

// Run 执行 ETL 流程
func (p *Pipeline) Run() error {
	if err := p.source.Open(); err != nil {
		return fmt.Errorf("pipeline source: %w", err)
	}
	defer p.source.Close()

	if err := p.load.Open(); err != nil {
		return fmt.Errorf("pipeline load: %w", err)
	}
	defer p.load.Close()

	var rowCount int
	for {
		row, ok := p.source.Read()
		if !ok {
			break
		}

		var rows []map[string]any
		var err error
		if p.transformer != nil {
			rows, err = p.transformer.Transform(row)
			if err != nil {
				// 转换失败跳过当前行
				continue
			}
		} else {
			rows = []map[string]any{row}
		}

		for _, r := range rows {
			if err := p.load.Write(r); err != nil {
				return fmt.Errorf("pipeline write: %w", err)
			}
			rowCount++
		}
	}

	fmt.Printf("Pipeline 完成，共处理 %d 行\n", rowCount)
	return nil
}
