// Package source 提供数据抽取接口和实现
package source

// Source 数据源接口：返回行数据迭代器
type Source interface {
	Open() error
	Read() (map[string]any, bool) // (行数据, 是否还有更多)
	Close() error
}
