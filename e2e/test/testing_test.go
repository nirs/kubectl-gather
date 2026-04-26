package test

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestErrorfLogs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger = zap.New(core).Named("test").Sugar()
	tt := WithLog(t)

	tt.Log.Errorf("test error: %s", "something went wrong")

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("expected ERROR level, got %s", entry.Level)
	}
	if entry.Message != "test error: something went wrong" {
		t.Errorf("unexpected message: %s", entry.Message)
	}
}

func TestFatalfLogs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger = zap.New(core).Named("test").Sugar()
	tt := WithLog(t)

	tt.Log.Errorf("fatal error: %s", "cannot continue")

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("expected ERROR level, got %s", entry.Level)
	}
	if entry.Message != "fatal error: cannot continue" {
		t.Errorf("unexpected message: %s", entry.Message)
	}
}

func TestDebugfLogs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger = zap.New(core).Named("test").Sugar()
	tt := WithLog(t)

	tt.Debugf("debug message: %d", 42)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.DebugLevel {
		t.Errorf("expected DEBUG level, got %s", entry.Level)
	}
	if entry.Message != "debug message: 42" {
		t.Errorf("unexpected message: %s", entry.Message)
	}
}
