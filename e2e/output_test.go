package e2e

import (
	"os/exec"
	"path/filepath"
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/e2e/commands"
	"github.com/nirs/kubectl-gather/pkg/gather"
)

func TestOutput(t *testing.T) {
	outputDir := "out/test-output"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", clusters.C1,
		"--kubeconfig", clusters.Kubeconfig(),
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	reader := gather.NewOutputReader(filepath.Join(outputDir, clusters.C1))

	t.Run("deployment", func(t *testing.T) {
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
		t.Logf("Read deployment %q", deployment.Name)
	})

	t.Run("pods", func(t *testing.T) {
		pods, err := reader.ListResources("test-common", "pods")
		if err != nil {
			t.Fatal(err)
		}
		if len(pods) == 0 {
			t.Fatalf("no pod found")
		}
		t.Logf("Listed pods %q", pods)

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
			t.Logf("Read pod %q", pod.Name)
		}
	})

	t.Run("cluster scope", func(t *testing.T) {
		namespaces, err := reader.ListResources("", "namespaces")
		if err != nil {
			t.Fatal(err)
		}
		if len(namespaces) == 0 {
			t.Fatalf("no namespaces found")
		}
		t.Logf("Listed namespaces %q", namespaces)

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
			t.Logf("Read namespace %q", namespace.Name)
		}
	})

	t.Run("list non existent directory", func(t *testing.T) {
		namespaceResources, err := reader.ListResources("test-common", "non-existent")
		if err != nil {
			t.Fatal(err)
		}
		if len(namespaceResources) != 0 {
			t.Errorf("expected empty slice, got %d resources: %v",
				len(namespaceResources), namespaceResources)
		}

		clusterResources, err := reader.ListResources("", "non-existent")
		if err != nil {
			t.Fatal(err)
		}
		if len(clusterResources) != 0 {
			t.Errorf("expected empty slice, got %d resources: %v",
				len(clusterResources), clusterResources)
		}
	})
}
