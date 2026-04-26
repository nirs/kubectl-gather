package e2e

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/e2e/commands"
	"github.com/nirs/kubectl-gather/e2e/test"
	"github.com/nirs/kubectl-gather/pkg/gather"
)

func TestOutput(t *testing.T) {
	t.Run("local", func(dt *testing.T) {
		t := test.WithLog(dt)
		outputDir := "out/test-output-local"

		cmd := exec.Command(
			kubectlGather,
			"--contexts", clusters.C1,
			"--directory", outputDir,
		)
		if err := commands.Run(cmd, t.Log); err != nil {
			t.Fatal(err)
		}

		reader := gather.NewOutputReader(filepath.Join(outputDir, clusters.C1))
		testOutputReader(t, reader)
	})

	t.Run("remote", func(dt *testing.T) {
		t := test.WithLog(dt)
		if _, err := exec.LookPath("oc"); err != nil {
			t.Skip("oc not found, skipping remote test")
		}

		outputDir := "out/test-output-remote"
		os.RemoveAll(outputDir)

		cmd := exec.Command(
			kubectlGather,
			"--contexts", clusters.C1,
			"--remote",
			"--directory", outputDir,
		)
		if err := commands.Run(cmd, t.Log); err != nil {
			t.Fatal(err)
		}

		dataRoot := findDataRoot(t, filepath.Join(outputDir, clusters.C1))
		t.Debugf("Data root: %s", dataRoot)

		reader := gather.NewOutputReader(filepath.Join(outputDir, clusters.C1, dataRoot))
		testOutputReader(t, reader)
	})
}

func testOutputReader(t *test.T, reader *gather.OutputReader) {
	t.Helper()

	t.Run("deployment", func(dt *testing.T) {
		t := test.WithLog(dt)
		name := "common-busybox"
		data, err := reader.ReadResource("test-common", "apps/deployments", name)
		if err != nil {
			t.Fatal(err)
		}
		deployment := apps.Deployment{}
		if err := yaml.Unmarshal(data, &deployment); err != nil {
			t.Fatal(err)
		}
		if deployment.Name != name {
			t.Errorf("expected deployment name %q, got %s", name, deployment.Name)
		}
		t.Debugf("Read deployment %q", deployment.Name)
	})

	t.Run("pods", func(dt *testing.T) {
		t := test.WithLog(dt)
		pods, err := reader.ListResources("test-common", "pods")
		if err != nil {
			t.Fatal(err)
		}
		if len(pods) == 0 {
			t.Fatalf("no pod found")
		}
		t.Debugf("Listed pods %q", pods)

		for _, name := range pods {
			data, err := reader.ReadResource("test-common", "pods", name)
			if err != nil {
				t.Fatal(err)
			}
			pod := core.Pod{}
			if err := yaml.Unmarshal(data, &pod); err != nil {
				t.Fatal(err)
			}
			if pod.Name != name {
				t.Errorf("expected pod name %q, got %s", name, pod.Name)
			}
			t.Debugf("Read pod %q", pod.Name)
		}
	})

	t.Run("cluster scope", func(dt *testing.T) {
		t := test.WithLog(dt)
		namespaces, err := reader.ListResources("", "namespaces")
		if err != nil {
			t.Fatal(err)
		}
		if len(namespaces) == 0 {
			t.Fatalf("no namespaces found")
		}
		t.Debugf("Listed namespaces %q", namespaces)

		for _, name := range namespaces {
			data, err := reader.ReadResource("", "namespaces", name)
			if err != nil {
				t.Fatal(err)
			}
			namespace := core.Namespace{}
			if err := yaml.Unmarshal(data, &namespace); err != nil {
				t.Fatal(err)
			}
			if namespace.Name != name {
				t.Errorf("expected namespace name %q, got %s", name, namespace.Name)
			}
			t.Debugf("Read namespace %q", namespace.Name)
		}
	})

	t.Run("missing namespaced", func(dt *testing.T) {
		t := test.WithLog(dt)
		found, err := reader.ListResources("test-common", "missing")
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 0 {
			t.Errorf("expected empty slice, got %v", found)
		}
	})

	t.Run("missing cluster scope", func(dt *testing.T) {
		t := test.WithLog(dt)
		found, err := reader.ListResources("", "missing")
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 0 {
			t.Errorf("expected empty slice, got %v", found)
		}
	})
}

func TestSecretSanitization(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-secret-sanitization"

	salt := gather.RandomSalt()
	saltB64 := base64.StdEncoding.EncodeToString(salt[:])

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--salt", saltB64,
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Fatalf("kubectl-gather failed: %s", err)
	}

	for _, cluster := range clusters.Names {
		for _, secret := range gatheredSecrets(t, outputDir, cluster) {
			salt, ok := secret.Annotations["kubectl-gather.nirs.github.com/sanitized"]
			if !ok {
				t.Fatalf("secret %s: sanitized annotation not found", secret.Name)
			}
			if salt != saltB64 {
				t.Errorf("secret %s: expected salt %q, got %q", secret.Name, saltB64, salt)
			}
		}
	}
}

func TestSecretSanitizationRandomSalt(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-secret-sanitization-random"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Fatalf("kubectl-gather failed: %s", err)
	}

	salts := map[string]struct{}{}
	for _, cluster := range clusters.Names {
		for _, secret := range gatheredSecrets(t, outputDir, cluster) {
			salt, ok := secret.Annotations["kubectl-gather.nirs.github.com/sanitized"]
			if !ok {
				t.Fatalf("secret %s: sanitized annotation not found", secret.Name)
			}
			salts[salt] = struct{}{}
		}
	}
	if len(salts) != 1 {
		t.Fatalf("secrets hashed with multiple salts: %v", salts)
	}
}

// Test helpers

// gatheredSecrets returns all secrets gathered for a cluster.
func gatheredSecrets(t *test.T, outputDir, cluster string) []core.Secret {
	t.Helper()
	pattern := filepath.Join(outputDir, cluster, "namespaces", "*", "secrets", "*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no secrets found for cluster %s", cluster)
	}
	var secrets []core.Secret
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var secret core.Secret
		if err := yaml.Unmarshal(data, &secret); err != nil {
			t.Fatal(err)
		}
		secrets = append(secrets, secret)
	}
	return secrets
}
