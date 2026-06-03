package log

import (
	"io"
	"log/slog"
	"os"
	"time"
)

var (
	lgLevel    *slog.LevelVar = &slog.LevelVar{}
	logWriter  io.Writer
	opts       *slog.HandlerOptions
	isSetLevel bool
)

// SetLevel 设置日志级别
//
//	import "log/slog"
//	log.SetLevel(slog.LevelDebug)
func SetLevel(level slog.Level) {
	lgLevel.Set(level)
	isSetLevel = true
}

// SetLogWriter 设置日志输出
//
//	f, err = os.OpenFile("netguard.log", os.O_CREATE|os.O_APPEND, 0644)
//	if err != nil {
//		panic(err)
//	}
//	defer f.Close()
//	log.SetLogWriter(f)
func SetLogWriter(writer io.Writer) {
	logWriter = writer
}

// SetLogWriterByFile 创建文件并设置日志输出
//
//	f, err := log.SetLogWriterByFile("netguard.log")
//	if err != nil {
//		panic(err)
//	}
//	defer f.Close()
func SetLogWriterByFile(filepath string) (f *os.File, err error) {
	f, err = os.OpenFile(filepath, os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		return
	}
	SetLogWriter(f)
	return f, err
}

// SetOptions 设置日志选项
//
//	import "log/slog"
//	lgLevel = &slog.LevelVar{}
//	sopts := &slog.HandlerOptions{
//		Level: lgLevel,
//		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
//			// 如果当前属性是时间戳
//			if a.Key == slog.TimeKey && len(groups) == 0 {
//				a.Key = "time" // 键名可以保持不变或修改
//				// 将时间值转换为自定义格式
//				if t, ok := a.Value.Any().(time.Time); ok {
//					a.Value = slog.StringValue(t.Format(time.DateTime))
//				}
//			}
//			return a
//		},
//	}
//	SetOptions(sopts)
func SetOptions(sopts *slog.HandlerOptions) {
	opts = sopts
}

func NewOptions() *slog.HandlerOptions {
	// 设置 HandlerOptions，自定义时间属性
	if !isSetLevel {
		// 设置默认日志级别为 Info
		lgLevel.Set(slog.LevelInfo)
	}
	return &slog.HandlerOptions{
		Level: lgLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 如果当前属性是时间戳
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Key = "time" // 键名可以保持不变或修改
				// 将时间值转换为自定义格式
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(time.DateTime))
				}
			}
			return a
		},
	}
}
