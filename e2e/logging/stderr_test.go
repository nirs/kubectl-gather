package logging

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func TestStderr(t *testing.T) {
	exitErr := &exec.ExitError{Stderr: []byte("boom\n")}

	if got := Stderr(exitErr); string(got) != "boom\n" {
		t.Errorf(`Stderr(exitErr): expected "boom\n", got %q`, got)
	}
	if got := Stderr(&exec.ExitError{}); got != nil {
		t.Errorf("Stderr(empty ExitError): expected nil, got %q", got)
	}
	if got := Stderr(errors.New("other")); got != nil {
		t.Errorf("Stderr(other error): expected nil, got %q", got)
	}
	if got := Stderr(fmt.Errorf("wrapped: %w", exitErr)); string(got) != "boom\n" {
		t.Errorf(`Stderr(wrapped): expected "boom\n", got %q`, got)
	}
}

func TestWithStderr(t *testing.T) {
	err := errors.New("exit status 1")

	if got := WithStderr(err, nil); got != err {
		t.Errorf("WithStderr(nil): expected the original error, got %v", got)
	}
	if got := WithStderr(err, []byte{}); got != err {
		t.Errorf("WithStderr(empty): expected the original error, got %v", got)
	}

	got := WithStderr(err, []byte("  boom\n\n"))
	if want := "exit status 1: boom"; got.Error() != want {
		t.Errorf("expected %q, got %q", want, got)
	}
	if !errors.Is(got, err) {
		t.Errorf("expected the wrapped error to match %v, got %v", err, got)
	}
}
