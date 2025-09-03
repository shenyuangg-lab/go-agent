package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var (
	// 全局日志实例
	log *logrus.Logger
)

// Init 初始化日志系统
func Init(verbose bool) error {
	return InitWithConfig("info", "json", "stdout", verbose)
}

// InitWithConfig 使用配置初始化日志系统
func InitWithConfig(level, format, output string, verbose bool) error {
	log = logrus.New()

	// 设置日志级别
	if verbose {
		log.SetLevel(logrus.DebugLevel)
	} else {
		if err := SetLevel(level); err != nil {
			return fmt.Errorf("设置日志级别失败: %v", err)
		}
	}

	// 设置日志格式
	SetFormat(format)

	// 设置输出
	if err := SetOutput(output); err != nil {
		return fmt.Errorf("设置日志输出失败: %v", err)
	}

	return nil
}

// SetLevel 设置日志级别
func SetLevel(level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	log.SetLevel(lvl)
	return nil
}

// SetFormat 设置日志格式
func SetFormat(format string) {
	switch format {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	case "text":
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}
}

// SetOutput 设置日志输出
func SetOutput(output string) error {
	switch output {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	case "file":
		// 这里可以扩展为文件输出
		return nil
	default:
		return nil
	}
	return nil
}

// SetOutputWriter 设置自定义输出写入器
func SetOutputWriter(writer io.Writer) {
	log.SetOutput(writer)
}

// Debug 调试级别日志
func Debug(args ...interface{}) {
	if log != nil {
		log.Debug(args...)
	}
}

// Debugf 格式化调试级别日志
func Debugf(format string, args ...interface{}) {
	if log != nil {
		log.Debugf(format, args...)
	}
}

// Info 信息级别日志
func Info(args ...interface{}) {
	if log != nil {
		log.Info(args...)
	}
}

// Infof 格式化信息级别日志
func Infof(format string, args ...interface{}) {
	if log != nil {
		log.Infof(format, args...)
	}
}

// Warn 警告级别日志
func Warn(args ...interface{}) {
	if log != nil {
		log.Warn(args...)
	}
}

// Warnf 格式化警告级别日志
func Warnf(format string, args ...interface{}) {
	if log != nil {
		log.Warnf(format, args...)
	}
}

// Error 错误级别日志
func Error(args ...interface{}) {
	if log != nil {
		log.Error(args...)
	}
}

// Errorf 格式化错误级别日志
func Errorf(format string, args ...interface{}) {
	if log != nil {
		log.Errorf(format, args...)
	}
}

// Fatal 致命错误级别日志
func Fatal(args ...interface{}) {
	if log != nil {
		log.Fatal(args...)
	}
}

// Fatalf 格式化致命错误级别日志
func Fatalf(format string, args ...interface{}) {
	if log != nil {
		log.Fatalf(format, args...)
	}
}

// Panic 恐慌级别日志
func Panic(args ...interface{}) {
	if log != nil {
		log.Panic(args...)
	}
}

// Panicf 格式化恐慌级别日志
func Panicf(format string, args ...interface{}) {
	if log != nil {
		log.Panicf(format, args...)
	}
}

// WithField 添加字段到日志
func WithField(key string, value interface{}) *logrus.Entry {
	if log != nil {
		return log.WithField(key, value)
	}
	return nil
}

// WithFields 添加多个字段到日志
func WithFields(fields logrus.Fields) *logrus.Entry {
	if log != nil {
		return log.WithFields(fields)
	}
	return nil
}

// WithError 添加错误到日志
func WithError(err error) *logrus.Entry {
	if log != nil {
		return log.WithError(err)
	}
	return nil
}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	return log
}

// IsInitialized 检查日志是否已初始化
func IsInitialized() bool {
	return log != nil
}
