package log

import (
	"log/slog"
	"os"
	"sync"
)

var (
	lg   *slog.Logger
	once sync.Once
)

// Debug 记录调试级别日志
func Debug(msg string, args ...any) {
	getLogger().Debug(msg, args...)
}

// Info 记录信息级别日志
func Info(msg string, args ...any) {
	getLogger().Info(msg, args...)
}

// Warn 记录警告级别日志
func Warn(msg string, args ...any) {
	getLogger().Warn(msg, args...)
}

// Error 记录错误级别日志
func Error(msg string, args ...any) {
	getLogger().Error(msg, args...)
}

// getLogger 获取单例日志实例
func getLogger() *slog.Logger {
	once.Do(func() {
		if logWriter == nil {
			logWriter = os.Stdout
		}
		if opts == nil {
			opts = NewOptions()
		}
		lg = slog.New(slog.NewTextHandler(logWriter, opts))
	})
	return lg
}

// ResetLogger 重置日志 似乎无法即刻生效。
func ResetLogger() {
	lg = slog.New(slog.NewTextHandler(logWriter, opts))
}
