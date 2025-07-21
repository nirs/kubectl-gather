package e2e_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/e2e/commands"
)

const executable = "../kubectl-gather"

func TestGather(t *testing.T) {
	cmd := exec.Command(
		executable,
		"--contexts", strings.Join(clusters.Names(), ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--directory", "out/test-gather",
	)
	if err := commands.LogStderr(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}
	// XXX verify gathered data.
}

func TestJSONLogs(t *testing.T) {
	cmd := exec.Command(
		executable,
		"--contexts", strings.Join(clusters.Names(), ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--directory", "out/test-json-logs",
		"--log-format", "json",
	)
	if err := commands.LogStderr(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}
	// XXX verify gathered data.
}
