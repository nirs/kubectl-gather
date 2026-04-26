package e2e

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nirs/kubectl-gather/e2e/test"
)

const kubectlGather = "../kubectl-gather"

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

	cmd := exec.Command(kubectlGather, "--must-gather-version")
	expected, err := cmd.Output()
	if err != nil {
		t.Fatalf("kubectl-gather --must-gather-version failed: %s", err)
	}

	if string(version) != string(expected) {
		t.Fatalf("version mismatch: got %q, want %q", version, expected)
	}

	return filepath.Base(filepath.Dir(matches[0]))
}
