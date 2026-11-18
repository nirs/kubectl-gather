package logging

import (
	"maps"
	"os/exec"
	"slices"
	"testing"
)

// TestJSONLogger checks parsing of zap JSON entries. Entries include the ts
// and caller fields added by zap, which must not be logged.
func TestJSONLogger(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want string
		// wantFields are the entry fields expected in the log context. Use nil
		// when the entry has no fields besides the dropped metadata.
		wantFields map[string]any
	}{
		{
			name: "info",
			line: `{"level":"INFO","ts":1234.5,"caller":"cmd/root.go:42","logger":"gather","msg":"Gathering"}`,
			want: "| INFO gather: Gathering",
		},
		{
			name: "debug",
			line: `{"level":"DEBUG","ts":1234.5,"caller":"pkg/gather/gather.go:42","logger":"gather.c1","msg":"Gathered pods"}`,
			want: "| DEBUG gather.c1: Gathered pods",
		},
		{
			name: "warn",
			line: `{"level":"WARN","ts":1234.5,"caller":"pkg/gather/gather.go:42","logger":"gather.c1","msg":"Cannot list leases"}`,
			want: "| WARN gather.c1: Cannot list leases",
		},
		{
			name: "error",
			line: `{"level":"ERROR","ts":1234.5,"caller":"pkg/gather/gather.go:42","logger":"gather","msg":"Cannot dump secret"}`,
			want: "| ERROR gather: Cannot dump secret",
		},
		{
			// A command that fails as expected should not look like a test
			// failure, so every level is logged as debug.
			name: "fatal logged at debug",
			line: `{"level":"FATAL","ts":1234.5,"caller":"cmd/root.go:42","logger":"gather","msg":"nothing to gather"}`,
			want: "| FATAL gather: nothing to gather",
		},
		{
			name: "missing logger",
			line: `{"level":"INFO","ts":1234.5,"caller":"cmd/root.go:42","msg":"Gathering"}`,
			want: "| INFO Gathering",
		},
		{
			name: "missing level",
			line: `{"ts":1234.5,"caller":"cmd/root.go:42","logger":"gather","msg":"Gathering"}`,
			want: "| gather: Gathering",
		},
		{
			name: "empty level and logger",
			line: `{"level":"","logger":"","ts":1234.5,"caller":"cmd/root.go:42","msg":"Gathering"}`,
			want: "| Gathering",
		},
		{
			name: "non-string level and logger",
			line: `{"level":1,"logger":true,"ts":1234.5,"caller":"cmd/root.go:42","msg":"Gathering"}`,
			want: "| Gathering",
		},
		{
			name: "missing level and logger",
			line: `{"ts":1234.5,"caller":"cmd/root.go:42","msg":"Gathering"}`,
			want: "| Gathering",
		},
		{
			name: "extra fields",
			line: `{"level":"ERROR","ts":1234.5,"caller":"pkg/gather/gather.go:42","logger":"gather.c1","msg":"Cannot dump","error":"timeout","count":3,"stacktrace":"goroutine 1"}`,
			want: "| ERROR gather.c1: Cannot dump",
			wantFields: map[string]any{
				"count":      float64(3),
				"error":      "timeout",
				"stacktrace": "goroutine 1",
			},
		},
		{
			name: "non-json line",
			line: "panic: runtime error",
			want: "| panic: runtime error",
		},
		{
			name: "non-object json line",
			line: `["not", "an", "object"]`,
			want: `| ["not", "an", "object"]`,
		},
		{
			name: "empty line",
			line: "",
			want: "| ",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log, logs := newLogger()
			logger := NewJSONLogger(log)

			if _, err := logger.Write([]byte(tc.line + "\n")); err != nil {
				t.Fatal(err)
			}

			entries := logs.All()
			if len(entries) != 1 {
				t.Fatalf("expected one entry, got %d", len(entries))
			}
			if got := entries[0].Message; got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
			if got := entries[0].Level; got.String() != "debug" {
				t.Errorf("expected debug level, got %s", got)
			}
			if got := entries[0].ContextMap(); !maps.Equal(got, tc.wantFields) {
				t.Errorf("expected fields %v, got %v", tc.wantFields, got)
			}
		})
	}
}

func TestJSONLoggerPartialLines(t *testing.T) {
	log, logs := newLogger()
	logger := NewJSONLogger(log)

	writes := []string{
		`{"level":"INFO","logger":"gather","msg":"line `,
		`1"}` + "\n",
		`{"level":"INFO","logger":"gather","msg":"line 2"}`,
	}
	for _, write := range writes {
		if _, err := logger.Write([]byte(write)); err != nil {
			t.Fatal(err)
		}
	}
	logger.Flush()

	want := []string{"| INFO gather: line 1", "| INFO gather: line 2"}
	if got := messages(logs); !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestJSONLoggerCRLF(t *testing.T) {
	log, logs := newLogger()
	logger := NewJSONLogger(log)

	if _, err := logger.Write(
		[]byte(`{"level":"INFO","logger":"gather","msg":"line"}` + "\r\n"),
	); err != nil {
		t.Fatal(err)
	}
	logger.Flush()

	want := []string{"| INFO gather: line"}
	if got := messages(logs); !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestJSONLoggerCommand checks the logger with output from a real command,
// mixing JSON entries and other output.
func TestJSONLoggerCommand(t *testing.T) {
	log, logs := newLogger()
	logger := NewJSONLogger(log)

	script := `\
echo '{"level":"INFO","logger":"gather","msg":"line 1"}' >&2
echo 'not json' >&2
echo '{"level":"ERROR","logger":"gather.c1","msg":"line 2"}' >&2
`
	cmd := exec.Command("bash", "-c", script)
	cmd.Stderr = logger
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	logger.Flush()

	want := []string{
		"| INFO gather: line 1",
		"| not json",
		"| ERROR gather.c1: line 2",
	}
	if got := messages(logs); !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}
