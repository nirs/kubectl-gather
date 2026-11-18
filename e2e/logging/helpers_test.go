package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newLogger returns a logger logging to an observer, and the observed logs.
func newLogger() (*zap.SugaredLogger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return zap.New(core).Sugar(), logs
}

// messages returns the messages of all observed log entries.
func messages(logs *observer.ObservedLogs) []string {
	var got []string
	for _, entry := range logs.All() {
		got = append(got, entry.Message)
	}
	return got
}
