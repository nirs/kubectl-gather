package logging

import (
	"bytes"

	"go.uber.org/zap"
)

// LineLogger logs every complete line written to it at debug level, prefixed
// with separator.
//
// A line is logged when a newline is written. Call Flush after the command
// exits to log a final partial line.
//
// LineLogger is not safe for concurrent use. Use one logger per stream, or
// assign the same logger to both Stdout and Stderr, which makes exec.Cmd copy
// both streams with one goroutine.
type LineLogger struct {
	log *zap.SugaredLogger
	buf []byte
}

// NewLineLogger creates a LineLogger logging to log.
func NewLineLogger(log *zap.SugaredLogger) *LineLogger {
	return &LineLogger{log: log}
}

// Write logs the complete lines in p. It always returns len(p), nil.
func (l *LineLogger) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i == -1 {
			break
		}
		l.logLine(l.buf[:i])
		l.buf = l.buf[i+1:]
	}
	return len(p), nil
}

// Flush logs a final partial line, if any.
func (l *LineLogger) Flush() {
	if len(l.buf) == 0 {
		return
	}
	l.logLine(l.buf)
	l.buf = nil
}

func (l *LineLogger) logLine(line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\r'})
	l.log.Debug(separator + string(line))
}
