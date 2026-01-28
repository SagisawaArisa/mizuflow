package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	l    *zap.Logger
	once sync.Once
)

func InitLogger(env string) {
	once.Do(func() {
		var cfg zap.Config

		if env == "prod" {
			cfg = zap.NewProductionConfig()
			cfg.EncoderConfig.TimeKey = "time"
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		} else {
			cfg = zap.NewDevelopmentConfig()
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}

		logger, err := cfg.Build(zap.AddCaller(), zap.AddCallerSkip(1))
		if err != nil {
			panic(err)
		}

		l = logger
	})
}

func SetLogger(logger *zap.Logger) {
	l = logger
}

func Info(msg string, fields ...zap.Field) {
	l.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	l.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	l.Debug(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	l.Warn(msg, fields...)
}

func Sync() error {
	return l.Sync()
}
