package logging

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"

	"go.uber.org/zap"
)

// JSONLogger logs every complete line written to it at debug level.
//
// Each line is expected to be a zap JSON log entry, as written by
// kubectl-gather with --log-format json. The entry level and logger name are
// added to the message, so the e2e log shows where the message came from:
//
//	| INFO gather.c1: Gathered 10 pods
//
// Every entry is logged at debug level. The e2e log warns and errors are
// reserved for test problems, so an expected failure of the command does not
// look like a test failure. The original timestamps and callers are not
// logged; the full command log is available in gather.log.
//
// A line that is not a JSON object is logged as is, for example stdlog.Fatalf
// output or a Go panic.
//
// A line is logged when a newline is written. Call Flush after the command
// exits to log a final partial line.
//
// JSONLogger is not safe for concurrent use. Use one logger per stream.
type JSONLogger struct {
	log *zap.SugaredLogger
	buf []byte
}

// NewJSONLogger creates a JSONLogger logging to log.
func NewJSONLogger(log *zap.SugaredLogger) *JSONLogger {
	return &JSONLogger{log: log}
}

// Write logs the complete lines in p. It always returns len(p), nil.
func (l *JSONLogger) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i == -1 {
			break
		}
		l.logEntry(l.buf[:i])
		l.buf = l.buf[i+1:]
	}
	return len(p), nil
}

// Flush logs a final partial line, if any.
func (l *JSONLogger) Flush() {
	if len(l.buf) == 0 {
		return
	}
	l.logEntry(l.buf)
	l.buf = nil
}

func (l *JSONLogger) logEntry(line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\r'})

	var fields map[string]any
	if err := json.Unmarshal(line, &fields); err != nil {
		l.log.Debug(separator + string(line))
		return
	}

	prefix := separator
	if level := stringField(fields, "level"); level != "" {
		prefix += level + " "
	}
	if logger := stringField(fields, "logger"); logger != "" {
		prefix += logger + ": "
	}

	msg := prefix + stringField(fields, "msg")

	// Logged by the e2e logger, or available in gather.log. Logging them here
	// duplicates information and makes the e2e log harder to read.
	delete(fields, "level")
	delete(fields, "logger")
	delete(fields, "msg")
	delete(fields, "ts")
	delete(fields, "caller")

	if len(fields) == 0 {
		l.log.Debug(msg)
		return
	}

	l.log.Debugw(msg, sortedFields(fields)...)
}

// stringField returns the string value of key, or "" if key is missing or has
// another type.
func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

// sortedFields returns fields as sorted key-value pairs, so log output is
// deterministic.
func sortedFields(fields map[string]any) []any {
	keys := slices.Sorted(maps.Keys(fields))

	keysAndValues := make([]any, 0, len(keys)*2)
	for _, key := range keys {
		keysAndValues = append(keysAndValues, key, fields[key])
	}
	return keysAndValues
}
