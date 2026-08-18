// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestGatherContainerLogRetryGoAway(t *testing.T) {
	server := startLogServer(t,
		logResponse{data: "partial ", end: sendGoAway},
		logResponse{data: "complete log", end: endStream},
	)
	a, base, logs := newTestLogsAddon(t, server)

	a.gatherContainerLog(testContainer, &corev1.PodLogOptions{Container: testContainer.Name})

	checkContainerLog(t, base, "complete log")
	checkWarnings(t, logs)

	checkMessages(t, logs,
		`Gathering "ns/pod/container/current.log" (1/3)`,
		`Failed "ns/pod/container/current.log" in `,
		`Gathering "ns/pod/container/current.log" (2/3)`,
		`Gathered "ns/pod/container/current.log" in `,
	)
}

func TestGatherContainerLogRetryLimit(t *testing.T) {
	server := startLogServer(t,
		logResponse{data: "one", end: sendGoAway},
		logResponse{data: "two", end: sendGoAway},
		logResponse{data: "three", end: sendGoAway},
		logResponse{data: "four", end: endStream},
	)
	a, base, logs := newTestLogsAddon(t, server)

	a.gatherContainerLog(testContainer, &corev1.PodLogOptions{Container: testContainer.Name})

	// Every attempt truncates the log, so the last one is kept.
	checkContainerLog(t, base, "three")
	checkWarnings(t, logs, "Cannot copy")
}

func TestGatherContainerLogNoRetryOnOtherErrors(t *testing.T) {
	// The connection is closed without a GOAWAY.
	server := startLogServer(t,
		logResponse{data: "partial ", end: closeConnection},
		logResponse{data: "complete log", end: endStream},
	)
	a, base, logs := newTestLogsAddon(t, server)

	a.gatherContainerLog(testContainer, &corev1.PodLogOptions{Container: testContainer.Name})

	checkContainerLog(t, base, "partial ")
	checkWarnings(t, logs, "Cannot copy")
}

func TestGatherContainerLogNoStream(t *testing.T) {
	// A container that is not running yet is expected, and must not create an
	// empty log.
	server := startLogServer(t)
	a, base, logs := newTestLogsAddon(t, server)

	a.gatherContainerLog(testContainer, &corev1.PodLogOptions{Container: testContainer.Name})

	if _, err := os.Stat(containerLogPath(base)); !os.IsNotExist(err) {
		t.Errorf("log was created for a container without a stream: %v", err)
	}
	checkWarnings(t, logs)
}

var testContainer = &containerInfo{Namespace: "ns", Pod: "pod", Name: "container"}

func newTestLogsAddon(
	t *testing.T,
	server *logServer,
) (*LogsAddon, string, *observer.ObservedLogs) {
	t.Helper()
	client, err := kubernetes.NewForConfig(&rest.Config{
		Host:            "https://" + server.Addr(),
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	core, logs := observer.New(zap.DebugLevel)
	addon := &LogsAddon{
		AddonBackend: &testBackend{output: &OutputDirectory{base: base}},
		client:       client,
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
	checkMessages(t, logs.FilterLevelExact(zap.WarnLevel), expected...)
}

// checkMessages compares the messages logged with the expected prefixes.
func checkMessages(t *testing.T, logs *observer.ObservedLogs, expected ...string) {
	t.Helper()
	var messages []string
	for _, entry := range logs.All() {
		messages = append(messages, entry.Message)
	}
	if len(messages) != len(expected) {
		t.Fatalf("logged %q, expected %q", messages, expected)
	}
	for i, prefix := range expected {
		if !strings.HasPrefix(messages[i], prefix) {
			t.Errorf("message %d is %q, expected prefix %q", i, messages[i], prefix)
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

// How a logResponse ends.
const (
	// Complete the response.
	endStream = iota
	// Send a GOAWAY and close the connection, as the api server, or a load
	// balancer in front of it, does when shutting down.
	sendGoAway
	// Close the connection without a GOAWAY.
	closeConnection
)

type logResponse struct {
	data string
	end  int
}

// logServer is an HTTP/2 server sending one log response per connection,
// writing the frames directly so it can interrupt a response the same way on
// every run. Connections after the last response are closed before the TLS
// handshake, so getting the log stream fails.
type logServer struct {
	listener  net.Listener
	responses []logResponse
	done      sync.WaitGroup
}

func startLogServer(t *testing.T, responses ...logResponse) *logServer {
	t.Helper()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{testCertificate(t)},
		NextProtos:   []string{http2.NextProtoTLS},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := &logServer{listener: listener, responses: responses}
	s.done.Add(1)
	go s.serve(t)
	t.Cleanup(func() {
		listener.Close()
		s.done.Wait()
	})
	return s
}

func (s *logServer) Addr() string { return s.listener.Addr().String() }

func (s *logServer) serve(t *testing.T) {
	defer s.done.Done()
	for i := 0; ; i++ {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		if i >= len(s.responses) {
			conn.(*tls.Conn).NetConn().Close()
			continue
		}
		if err := s.respond(conn.(*tls.Conn), s.responses[i]); err != nil {
			t.Errorf("cannot send response %d: %s", i, err)
		}
	}
}

func (s *logServer) respond(conn *tls.Conn, response logResponse) error {
	defer conn.Close()

	preface := make([]byte, len(http2.ClientPreface))
	if _, err := io.ReadFull(conn, preface); err != nil {
		return err
	}

	framer := http2.NewFramer(conn, conn)
	if err := framer.WriteSettings(); err != nil {
		return err
	}

	streamID, err := readRequest(framer)
	if err != nil {
		return err
	}

	var headers bytes.Buffer
	encoder := hpack.NewEncoder(&headers)
	if err := encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}); err != nil {
		return err
	}
	if err := framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		BlockFragment: headers.Bytes(),
		EndHeaders:    true,
	}); err != nil {
		return err
	}
	if err := framer.WriteData(
		streamID,
		response.end == endStream,
		[]byte(response.data),
	); err != nil {
		return err
	}
	if response.end == sendGoAway {
		if err := framer.WriteGoAway(streamID, http2.ErrCodeNo, nil); err != nil {
			return err
		}
	}

	// Closing with unread data sends a TCP RST, which may drop the frames we
	// sent. Wait until the client closes the connection after reading them.
	if err := conn.CloseWrite(); err != nil {
		return err
	}
	// The deadline fails the test instead of hanging if the client never
	// closes the connection.
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, conn); errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}
	return nil
}

// readRequest reads frames until the request headers, and returns the request
// stream ID.
func readRequest(framer *http2.Framer) (uint32, error) {
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return 0, err
		}
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				if err := framer.WriteSettingsAck(); err != nil {
					return 0, err
				}
			}
		case *http2.HeadersFrame:
			return f.StreamID, nil
		}
	}
}

// testCertificate returns the self-signed certificate used by httptest.
func testCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	server := httptest.NewUnstartedServer(nil)
	server.StartTLS()
	defer server.Close()
	return server.TLS.Certificates[0]
}
