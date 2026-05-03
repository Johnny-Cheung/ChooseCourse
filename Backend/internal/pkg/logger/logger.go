package logger

import (
	"strings"

	appconfig "choose-course-backend/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// base 是全局 logger 实例。
// 初始化完成后，整个项目都通过 logger.L() 获取它。
var base *zap.Logger

// Init 根据配置创建 zap logger。
func Init(cfg appconfig.LogConfig) error {
	level := zap.NewAtomicLevel()

	// 配置写错时，日志级别默认回落到 info，避免程序因为日志配置无法启动。
	if err := level.UnmarshalText([]byte(strings.ToLower(cfg.Level))); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	encoding := strings.ToLower(cfg.Encoding)
	if encoding != "json" {
		encoding = "console"
	}

	zapCfg := zap.Config{
		Level:       level,
		Encoding:    encoding,
		OutputPaths: []string{"stdout"},
		ErrorOutputPaths: []string{
			"stderr",
		},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}

	logger, err := zapCfg.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	if err != nil {
		return err
	}

	base = logger
	return nil
}

// L 返回全局 logger。
// 如果尚未初始化，则返回一个空 logger，避免空指针崩溃。
func L() *zap.Logger {
	if base == nil {
		base = zap.NewNop()
	}

	return base
}

// Sync 把日志缓冲区中的内容刷到输出目标。
func Sync() {
	_ = L().Sync()
}

// 下面这些函数只是对 zap.Field 的简单封装，
// 这样业务代码里不用直接依赖太多 zap 细节。
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func Duration(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}

func Any(key string, value any) zap.Field {
	return zap.Any(key, value)
}
