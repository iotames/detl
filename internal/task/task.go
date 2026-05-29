// Package task 提供 ETL 任务定义和 YAML 解析
//
// 支持两种 kind：
//   - 转换（Transform）：单个 ETL 流程，包含 Source → Transform → Load
//   - 作业（Job）：多个转换的集合（预留，尚未实现执行引擎）
package task

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TaskConfig 单个 ETL 转换任务定义
type TaskConfig struct {
	Kind      string           `yaml:"kind"`                // "转换" 或 "作业"
	Name      string           `yaml:"name"`                // 任务名称
	Source    *SourceConfig    `yaml:"source,omitempty"`    // 数据源配置（转换用）
	Transform *TransformConfig `yaml:"transform,omitempty"` // 转换配置（转换用）
	Load      *LoadConfig      `yaml:"load,omitempty"`      // 输出配置（转换用）
	Tasks     []JobEntry       `yaml:"tasks,omitempty"`     // 子任务列表（作业用）
}

// SourceConfig 数据源配置
type SourceConfig struct {
	Connection string `yaml:"connection"`            // dsn.json 中的连接名
	QueryFile  string `yaml:"query_file"`            // SQL 脚本文件名（相对于 SCRIPT_DIR）
	Query      string `yaml:"query,omitempty"`       // 内联 SQL（与 query_file 二选一）
}

// TransformConfig 转换配置
type TransformConfig struct {
	Mode   string `yaml:"mode"`              // builtin | python | none
	Script string `yaml:"script,omitempty"`  // Python 脚本路径（mode=python 时生效）
}

// LoadConfig 输出配置
type LoadConfig struct {
	Type    string   `yaml:"type"`              // csv | stdout | sql
	File    string   `yaml:"file,omitempty"`    // 输出文件名（csv 用）
	Columns []string `yaml:"columns,omitempty"` // 列名顺序

	// SQL Load 配置
	Connection  string   `yaml:"connection,omitempty"`   // dsn.json 连接名
	Table       string   `yaml:"table,omitempty"`        // 目标表（支持 schema.table）
	Mode        string   `yaml:"mode,omitempty"`         // insert | upsert
	KeyColumns  []string `yaml:"key_columns,omitempty"`  // upsert 唯一键
	CreateTable bool     `yaml:"create_table,omitempty"` // 自动建表
	BatchSize   int      `yaml:"batch_size,omitempty"`   // 每批写入行数（默认 50）
}

// JobEntry 作业中的子任务
type JobEntry struct {
	Task string `yaml:"task"` // 任务文件路径（相对于父任务所在目录）
}

// IsTransform 是否转换任务
func (t TaskConfig) IsTransform() bool {
	return t.Kind == "转换"
}

// IsJob 是否作业
func (t TaskConfig) IsJob() bool {
	return t.Kind == "作业"
}

// LoadTask 从 YAML 文件加载任务/作业定义
func LoadTask(path string) (*TaskConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取任务文件 %s 失败: %w", path, err)
	}
	var cfg TaskConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析任务文件 %s 失败: %w", path, err)
	}
	if cfg.Kind != "转换" && cfg.Kind != "作业" {
		return nil, fmt.Errorf("未知任务类型 %q（仅支持 转换/作业）", cfg.Kind)
	}
	return &cfg, nil
}

