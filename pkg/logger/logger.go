package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func Init() error {
	return InitWithLevel("warn") // Default to info level
}

func InitWithLevel(level string) error {
	return InitWithOptions(level, false)
}

func InitWithOptions(level string, stdoutOnly bool) error {
	// Convert string level to zap level
	var zapLevel zap.AtomicLevel
	switch level {
	case "debug":
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	cfg := zap.Config{
		Level:       zapLevel,
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:      "console",
		EncoderConfig: encoderCfg,
	}

	if stdoutOnly {
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	} else {
		// Ensure logs directory exists
		logsDir := "logs"
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			return fmt.Errorf("failed to create logs directory: %v", err)
		}

		// Generate log file name (including date)
		now := time.Now()
		logFileName := fmt.Sprintf("quota-manager-%s.log", now.Format("2006-01-02"))
		logFilePath := filepath.Join(logsDir, logFileName)

		cfg.OutputPaths = []string{"stdout", logFilePath}
		cfg.ErrorOutputPaths = []string{"stderr", logFilePath}
	}

	var err error
	Logger, err = cfg.Build()
	if err != nil {
		return err
	}
	return nil
}

func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}
