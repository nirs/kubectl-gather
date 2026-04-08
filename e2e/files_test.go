package e2e

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/pkg/gather"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

func TestRemoteDirectory(t *testing.T) {
	client := newClientset(t, clusters.C1)
	log := zap.NewNop().Sugar()
	ctx, cancel := context.WithTimeout(context.Background(), timeoutLocal)
	defer cancel()
	pod := busyboxPod(t, client)

	t.Run("copy", func(t *testing.T) {
		createRemoteFiles(t, clusters.C1, pod)
		t.Cleanup(func() { removeRemoteFiles(t, clusters.C1, pod, "/tmp/test-files") })

		dst := t.TempDir()
		opts := gather.Options{Context: clusters.C1}
		rd := gather.NewRemoteDirectory(ctx, pod, &opts, log)

		if err := rd.Gather("/tmp/test-files", dst); err != nil {
			t.Fatal(err)
		}

		assertFileContent(t, filepath.Join(dst, "file1.txt"), "hello\n")
		assertFileContent(t, filepath.Join(dst, "sub", "file2.txt"), "world\n")
	})

	t.Run("cancel", func(t *testing.T) {
		createSparseFile(t, clusters.C1, pod, "/tmp/sparse-file", "1G")
		t.Cleanup(func() { removeRemoteFiles(t, clusters.C1, pod, "/tmp/sparse-file") })

		dst := t.TempDir()
		opts := gather.Options{Context: clusters.C1}

		cancelCtx, cancelGather := context.WithCancel(ctx)
		time.AfterFunc(200*time.Millisecond, cancelGather)

		rd := gather.NewRemoteDirectory(cancelCtx, pod, &opts, log)
		err := rd.Gather("/tmp/sparse-file", dst)
		if err == nil {
			t.Fatal("Gather should fail with cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %s", err)
		}
	})
}

func createRemoteFiles(t *testing.T, clusterContext string, pod *corev1.Pod) {
	t.Helper()
	script := "mkdir -p /tmp/test-files/sub && " +
		"echo hello > /tmp/test-files/file1.txt && " +
		"echo world > /tmp/test-files/sub/file2.txt"
	cmd := exec.Command(
		"kubectl", "exec",
		pod.Name,
		"--context="+clusterContext,
		"--namespace="+pod.Namespace,
		"--", "sh", "-c", script,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create remote files: %s: %s", err, out)
	}
}

func createSparseFile(t *testing.T, clusterContext string, pod *corev1.Pod, path string, size string) {
	t.Helper()
	cmd := exec.Command(
		"kubectl", "exec",
		pod.Name,
		"--context="+clusterContext,
		"--namespace="+pod.Namespace,
		"--", "truncate", "-s", size, path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create sparse file: %s: %s", err, out)
	}
}

func removeRemoteFiles(t *testing.T, clusterContext string, pod *corev1.Pod, path string) {
	t.Helper()
	cmd := exec.Command(
		"kubectl", "exec",
		pod.Name,
		"--context="+clusterContext,
		"--namespace="+pod.Namespace,
		"--", "rm", "-rf", path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("failed to remove remote files %q: %s: %s", path, err, out)
	}
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %q: %s", path, err)
	}
	if string(data) != expected {
		t.Errorf("expected %q in %q, got %q", expected, path, string(data))
	}
}
