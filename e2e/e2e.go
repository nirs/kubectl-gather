//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nirs/kubectl-gather/e2e/logging"
	"github.com/nirs/kubectl-gather/e2e/test"
)

const kubectlGather = "../kubectl-gather"

// runGather runs kubectl-gather and logs its output to the test log.
//
// Always uses JSON logs, so the level and logger name from kubectl-gather are
// kept in the message without nesting another timestamp and level.
func runGather(t *test.T, args ...string) error {
	args = append(args, "--log-format", "json")
	cmd := exec.Command(kubectlGather, args...)
	t.Debugf("Running %s", cmd)

	logger := logging.NewJSONLogger(t.Log)
	defer logger.Flush()
	cmd.Stderr = logger

	return cmd.Run()
}

// mustGatherVersion returns the version of the gathered data, as written by
// kubectl-gather --must-gather-version.
func mustGatherVersion() ([]byte, error) {
	out, err := exec.Command(kubectlGather, "--must-gather-version").Output()
	return out, logging.WithStderr(err, logging.Stderr(err))
}

// exportKubeconfig returns the kubeconfig for name.
//
// Never log the returned data; it contains credentials.
func exportKubeconfig(name string) ([]byte, error) {
	out, err := exec.Command(
		"kubectl", "config", "view",
		"--minify",
		"--flatten",
		"--context", name,
	).Output()
	return out, logging.WithStderr(err, logging.Stderr(err))
}

// findDataRoot finds the image digest directory in a remote gather cluster
// output. The directory name format depends on the oc version:
//   - "quay-io-nirsof-gather-sha256-..." (OpenShift)
//   - "sha256-..." (kind)
//
// The directory is verified by checking that it contains a version file
// matching the output of kubectl-gather --must-gather-version.
func findDataRoot(t *test.T, clusterDir string) string {
	t.Helper()

	pattern := filepath.Join(clusterDir, "*sha256-*", "version")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		t.Fatalf("no data root matching %q", pattern)
	}
	if len(matches) > 1 {
		t.Fatalf("multiple data roots matching %q: %q", pattern, matches)
	}

	version, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}

	expected, err := mustGatherVersion()
	if err != nil {
		t.Fatalf("kubectl-gather --must-gather-version failed: %s", err)
	}

	if string(version) != string(expected) {
		t.Fatalf("version mismatch: got %q, want %q", version, expected)
	}

	return filepath.Base(filepath.Dir(matches[0]))
}
