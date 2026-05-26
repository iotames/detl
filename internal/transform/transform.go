// Package transform 提供数据转换接口
package transform

// Transformer 数据转换接口：输入一行，输出零行或多行
type Transformer interface {
	Transform(map[string]any) ([]map[string]any, error)
}

// Func 适配器：将普通函数转为 Transformer
type Func func(map[string]any) ([]map[string]any, error)

func (f Func) Transform(row map[string]any) ([]map[string]any, error) {
	return f(row)
}
