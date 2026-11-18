package logging

import (
	"os/exec"
	"slices"
	"testing"
)

// TestLineLoggerIO checks the io.Writer and Flush behavior: how writes are
// split into lines, including CRLF, empty lines, and a final partial line.
func TestLineLoggerIO(t *testing.T) {
	for _, tc := range []struct {
		name   string
		writes []string
		// flush calls Flush after the writes to test logging of a final
		// partial line. When false, partial lines must not be logged.
		flush bool
		want  []string
	}{
		{
			name:   "complete lines",
			writes: []string{"line 1\nline 2\n"},
			want:   []string{"| line 1", "| line 2"},
		},
		{
			name:   "partial writes",
			writes: []string{"line 1", "\nline", " 2\n"},
			want:   []string{"| line 1", "| line 2"},
		},
		{
			name:   "partial line not logged",
			writes: []string{"line 1"},
		},
		{
			name:   "flush partial line",
			writes: []string{"line 1\nline 2"},
			flush:  true,
			want:   []string{"| line 1", "| line 2"},
		},
		{
			name:   "empty lines",
			writes: []string{"\n\n"},
			want:   []string{"| ", "| "},
		},
		{
			name:   "crlf",
			writes: []string{"one\r\ntwo\r\n"},
			want:   []string{"| one", "| two"},
		},
		{
			name:   "command output separator",
			writes: []string{"* minikube output\n"},
			want:   []string{"| * minikube output"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log, logs := newLogger()
			logger := NewLineLogger(log)

			for _, write := range tc.writes {
				n, err := logger.Write([]byte(write))
				if err != nil {
					t.Fatal(err)
				}
				if n != len(write) {
					t.Errorf("Write(%q) returned %d, want %d", write, n, len(write))
				}
			}
			if tc.flush {
				logger.Flush()
			}

			if got := messages(logs); !slices.Equal(got, tc.want) {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// TestLineLoggerCommand checks the logger with a real command, including
// flushing a final partial line after the command exits.
func TestLineLoggerCommand(t *testing.T) {
	log, logs := newLogger()
	logger := NewLineLogger(log)

	cmd := exec.Command("bash", "-c", "printf 'line 1\\npartial' >&2")
	cmd.Stderr = logger
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	logger.Flush()

	want := []string{"| line 1", "| partial"}
	if got := messages(logs); !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestLineLoggerSharedWriter checks that using the same logger for stdout and
// stderr works. exec.Cmd copies both streams with one goroutine only when the
// writers are equal, so this must not race. Run with -race.
func TestLineLoggerSharedWriter(t *testing.T) {
	log, logs := newLogger()
	logger := NewLineLogger(log)

	cmd := exec.Command("bash", "-c", "echo out; echo err >&2")
	cmd.Stdout = logger
	cmd.Stderr = logger
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	logger.Flush()

	want := []string{"| out", "| err"}
	if got := messages(logs); !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestLineLoggerSeparateWriters checks that stdout and stderr are logged when
// they use separate loggers.
func TestLineLoggerSeparateWriters(t *testing.T) {
	log, logs := newLogger()
	stdout := NewLineLogger(log)
	stderr := NewLineLogger(log)

	cmd := exec.Command("bash", "-c", "echo out; echo err >&2")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	stdout.Flush()
	stderr.Flush()

	// The order of stdout and stderr lines is not deterministic.
	want := []string{"| err", "| out"}
	got := messages(logs)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}
