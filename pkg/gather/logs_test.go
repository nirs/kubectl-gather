// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// Error text seen when the api server closes the connection during a graceful
// shutdown. See https://github.com/nirs/kubectl-gather/issues/178.
var errGoAway = errors.New(
	`http2: server sent GOAWAY and closed the connection; LastStreamID=97, ErrCode=NO_ERROR, debug=""`,
)

func TestCopyContainerLogRetryGoAway(t *testing.T) {
	a, base, logs := newTestLogsAddon(t)
	streamer := &fakeStreamer{
		streams: []*fakeStream{
			{data: "partial ", err: errGoAway},
			{data: "complete log"},
		},
	}

	a.copyContainerLog(testContainer, "current", streamer)

	checkContainerLog(t, base, "complete log")

	if streamer.attempts != 2 {
		t.Errorf("made %d attempts, expected 2", streamer.attempts)
	}
	if !streamer.streams[0].closed {
		t.Error("interrupted stream was not closed")
	}
	checkWarnings(t, logs, "Retrying")
}

func TestCopyContainerLogRetryLimit(t *testing.T) {
	a, base, logs := newTestLogsAddon(t)
	streamer := &fakeStreamer{
		streams: []*fakeStream{
			{data: "one", err: errGoAway},
			{data: "two", err: errGoAway},
			{data: "three", err: errGoAway},
			{data: "four", err: errGoAway},
		},
	}

	a.copyContainerLog(testContainer, "current", streamer)

	if streamer.attempts != logStreamAttempts {
		t.Errorf("made %d attempts, expected %d", streamer.attempts, logStreamAttempts)
	}

	// The last attempt is kept, truncated as before.
	checkContainerLog(t, base, "three")
	checkWarnings(t, logs, "Retrying", "Retrying", "Cannot copy")
}

func TestCopyContainerLogRetryStreamError(t *testing.T) {
	a, base, logs := newTestLogsAddon(t)
	streamer := &fakeStreamer{streams: []*fakeStream{{data: "partial", err: errGoAway}}}

	a.copyContainerLog(testContainer, "current", streamer)

	if streamer.attempts != 2 {
		t.Errorf("made %d attempts, expected 2", streamer.attempts)
	}

	checkContainerLog(t, base, "partial")
	checkWarnings(t, logs, "Retrying", "Cannot get log")
}

func TestCopyContainerLogNoRetryOnOtherErrors(t *testing.T) {
	// Only a GOAWAY is retried; other http2 errors are not.
	streamErrors := []error{
		io.ErrUnexpectedEOF,
		errors.New("http2: client connection lost"),
	}

	for _, streamErr := range streamErrors {
		t.Run(streamErr.Error(), func(t *testing.T) {
			a, base, logs := newTestLogsAddon(t)
			streamer := &fakeStreamer{
				streams: []*fakeStream{
					{data: "partial ", err: streamErr},
					{data: "complete log"},
				},
			}

			a.copyContainerLog(testContainer, "current", streamer)

			if streamer.attempts != 1 {
				t.Errorf("made %d attempts, expected 1", streamer.attempts)
			}

			checkContainerLog(t, base, "partial ")
			checkWarnings(t, logs, "Cannot copy")
		})
	}
}

func TestCopyContainerLogSuccess(t *testing.T) {
	a, base, logs := newTestLogsAddon(t)
	streamer := &fakeStreamer{streams: []*fakeStream{{data: "complete log"}}}

	a.copyContainerLog(testContainer, "current", streamer)

	if streamer.attempts != 1 {
		t.Errorf("made %d attempts, expected 1", streamer.attempts)
	}

	checkContainerLog(t, base, "complete log")
	checkWarnings(t, logs)
}

func TestCopyContainerLogNoStream(t *testing.T) {
	// A container that is not running yet is expected, and must not create an
	// empty log.
	a, base, logs := newTestLogsAddon(t)
	streamer := &fakeStreamer{}

	a.copyContainerLog(testContainer, "current", streamer)

	if _, err := os.Stat(containerLogPath(base)); !os.IsNotExist(err) {
		t.Errorf("log was created for a container without a stream: %v", err)
	}
	checkWarnings(t, logs)
}

var testContainer = &containerInfo{Namespace: "ns", Pod: "pod", Name: "container"}

func newTestLogsAddon(t *testing.T) (*LogsAddon, string, *observer.ObservedLogs) {
	t.Helper()
	base := t.TempDir()
	core, logs := observer.New(zap.WarnLevel)
	addon := &LogsAddon{
		AddonBackend: &testBackend{output: &OutputDirectory{base: base}},
		log:          zap.New(core).Sugar(),
	}
	return addon, base, logs
}

func containerLogPath(base string) string {
	return filepath.Join(
		base, namespacesDir, testContainer.Namespace, "pods", testContainer.Pod,
		testContainer.Name, "current"+logSuffix,
	)
}

func checkContainerLog(t *testing.T, base string, expected string) {
	t.Helper()
	data, err := os.ReadFile(containerLogPath(base))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != expected {
		t.Errorf("log is %q, expected %q", data, expected)
	}
}

// checkWarnings compares the warnings logged with the expected prefixes.
func checkWarnings(t *testing.T, logs *observer.ObservedLogs, expected ...string) {
	t.Helper()
	entries := logs.All()
	if len(entries) != len(expected) {
		t.Fatalf("logged %v, expected %q", entries, expected)
	}
	for i, prefix := range expected {
		if !strings.HasPrefix(entries[i].Message, prefix) {
			t.Errorf("warning %d is %q, expected prefix %q", i, entries[i].Message, prefix)
		}
	}
}

// testBackend implements the parts of AddonBackend used when gathering logs.
// The embedded interface panics if an unexpected method is called.
type testBackend struct {
	AddonBackend
	output *OutputDirectory
}

func (b *testBackend) Output() *OutputDirectory { return b.output }

// fakeStreamer returns the next fake stream on every call, simulating a new
// connection to the api server.
type fakeStreamer struct {
	streams  []*fakeStream
	attempts int
}

func (f *fakeStreamer) Stream(_ context.Context) (io.ReadCloser, error) {
	f.attempts++
	if f.attempts > len(f.streams) {
		return nil, errors.New("no more streams")
	}
	return f.streams[f.attempts-1], nil
}

// fakeStream returns data, and then err instead of io.EOF, simulating a stream
// interrupted after some data was received.
type fakeStream struct {
	data   string
	err    error
	offset int
	closed bool
}

func (s *fakeStream) Read(p []byte) (int, error) {
	if s.offset == len(s.data) {
		if s.err != nil {
			return 0, s.err
		}
		return 0, io.EOF
	}
	n := copy(p, s.data[s.offset:])
	s.offset += n
	return n, nil
}

func (s *fakeStream) Close() error {
	s.closed = true
	return nil
}
