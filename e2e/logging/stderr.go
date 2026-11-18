package logging

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

// Stderr returns the stderr captured by cmd.Output() in err, or nil if err is
// not an *exec.ExitError or captured no stderr.
func Stderr(err error) []byte {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.Stderr
	}
	return nil
}

// WithStderr adds stderr to err, if stderr is not empty.
func WithStderr(err error, stderr []byte) error {
	if len(stderr) == 0 {
		return err
	}
	return fmt.Errorf("%w: %s", err, bytes.TrimSpace(stderr))
}
