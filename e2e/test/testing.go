package test

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/nirs/kubectl-gather/e2e/logging"
)

var logger *zap.SugaredLogger

// Main creates the test logger and runs the tests. Must be called from
// TestMain.
func Main(m *testing.M) {
	var err error
	logger, err = logging.NewTestLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()
	m.Run()
}

// T extends testing.T to log test errors and fatals to a debug log file.
type T struct {
	*testing.T
	Log *zap.SugaredLogger
}

// WithLog returns a T wrapping dt with a named logger.
func WithLog(dt *testing.T) *T {
	return &T{T: dt, Log: logger.Named(dt.Name())}
}

func (t *T) Errorf(format string, args ...any) {
	t.Helper()
	t.Log.Errorf(format, args...)
	t.Fail()
}

func (t *T) Fatal(args ...any) {
	t.Helper()
	t.Log.Error(args...)
	t.FailNow()
}

func (t *T) Fatalf(format string, args ...any) {
	t.Helper()
	t.Log.Errorf(format, args...)
	t.FailNow()
}

func (t *T) Debugf(format string, args ...any) {
	t.Log.Debugf(format, args...)
}
