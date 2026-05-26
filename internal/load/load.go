// Package load 提供数据载入接口和实现
package load

// Load 数据目标接口
type Load interface {
	Open() error
	Write(map[string]any) error
	Close() error
}
