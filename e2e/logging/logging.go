package logging

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const loggerName = "e2e"

// CreateLogger creates a logger that writes debug output to e2e/out/e2e.log
// and minimal info messages to stderr. Must be called from the project root.
func NewLogger() (*zap.SugaredLogger, error) {
	fileCore, err := createFileCore("e2e/out/e2e.log")
	if err != nil {
		return nil, err
	}
	core := zapcore.NewTee(fileCore, createConsoleCore())
	return zap.New(core).Named(loggerName).Sugar(), nil
}

// CreateTestLogger creates a logger that writes debug output to out/e2e.log
// only, keeping the test console output clean. Must be called from the e2e
// package directory (go test runs from the package directory).
func NewTestLogger() (*zap.SugaredLogger, error) {
	core, err := createFileCore("out/e2e.log")
	if err != nil {
		return nil, err
	}
	return zap.New(core).Named(loggerName).Sugar(), nil
}

func createFileCore(logFile string) (zapcore.Core, error) {
	if err := os.MkdirAll(filepath.Dir(logFile), 0700); err != nil {
		return nil, fmt.Errorf("cannot create log directory: %w", err)
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot open log file: %w", err)
	}
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(config)
	return zapcore.NewCore(encoder, zapcore.Lock(f), zapcore.DebugLevel), nil
}

func createConsoleCore() zapcore.Core {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = zapcore.OmitKey
	config.CallerKey = zapcore.OmitKey
	config.StacktraceKey = zapcore.OmitKey
	config.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewCore(
		zapcore.NewConsoleEncoder(config),
		zapcore.Lock(os.Stderr),
		zapcore.InfoLevel,
	)
}
