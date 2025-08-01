package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/e2e/commands"
	"github.com/nirs/kubectl-gather/e2e/validate"
)

var (
	commonClusterResources = []string{
		"cluster/namespaces/test-common.yaml",
	}

	commonNamespacedResources = []string{
		"namespaces/test-common/persistentvolumeclaims/common-pvc1.yaml",
		"namespaces/test-common/pods/common-busybox-*.yaml",
		"namespaces/test-common/apps/deployments/common-busybox.yaml",
		"namespaces/test-common/apps/replicasets/common-busybox-*.yaml",
		"namespaces/test-common/configmaps/kube-root-ca.crt.yaml",
		"namespaces/test-common/serviceaccounts/default.yaml",
	}

	commonLogResources = []string{
		"namespaces/test-common/pods/common-busybox-*/busybox/current.log",
	}

	commonPVCResources = []string{
		"cluster/persistentvolumes/common-pv1.yaml",
	}

	c1ClusterNodes = []string{
		"cluster/nodes/c1-control-plane.yaml",
	}

	c1ClusterResources = []string{
		"cluster/namespaces/test-c1.yaml",
	}

	c1NamespaceResources = []string{
		"namespaces/test-c1/persistentvolumeclaims/c1-pvc1.yaml",
		"namespaces/test-c1/pods/c1-busybox-*.yaml",
		"namespaces/test-c1/apps/deployments/c1-busybox.yaml",
		"namespaces/test-c1/apps/replicasets/c1-busybox-*.yaml",
		"namespaces/test-c1/configmaps/kube-root-ca.crt.yaml",
		"namespaces/test-c1/serviceaccounts/default.yaml",
	}

	c1LogResources = []string{
		"namespaces/test-c1/pods/c1-busybox-*/busybox/current.log",
	}

	c1PVCResources = []string{
		"cluster/persistentvolumes/c1-pv1.yaml",
	}

	c2ClusterNodes = []string{
		"cluster/nodes/c2-control-plane.yaml",
	}

	c2ClusterResources = []string{
		"cluster/namespaces/test-c2.yaml",
	}

	c2NamespaceResources = []string{
		"namespaces/test-c2/persistentvolumeclaims/c2-pvc1.yaml",
		"namespaces/test-c2/pods/c2-busybox-*.yaml",
		"namespaces/test-c2/apps/deployments/c2-busybox.yaml",
		"namespaces/test-c2/apps/replicasets/c2-busybox-*.yaml",
		"namespaces/test-c2/configmaps/kube-root-ca.crt.yaml",
		"namespaces/test-c2/serviceaccounts/default.yaml",
	}

	c2LogResources = []string{
		"namespaces/test-c2/pods/c2-busybox-*/busybox/current.log",
	}

	c2PVCResources = []string{
		"cluster/persistentvolumes/c2-pv1.yaml",
	}

	defaultPVCResources = []string{
		"cluster/storage.k8s.io/storageclasses/standard.yaml",
	}
)

func TestGather(t *testing.T) {
	outputDir := "out/test-gather"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateGatherAll(t, outputDir)
}

func TestGatherClusterTrue(t *testing.T) {
	outputDir := "out/test-gather-cluster-true"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--cluster=true",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateGatherAll(t, outputDir)
}

func TestGatherClusterFalse(t *testing.T) {
	outputDir := "out/test-gather-cluster-false"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--cluster=false",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonPVCResources,
		commonNamespacedResources,
		commonLogResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C2},
		c2PVCResources,
		c2NamespaceResources,
		c2LogResources,
	)

	validate.Missing(t, outputDir, clusters.Names,
		commonClusterResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
	)
}

func TestGatherEmptyNamespaces(t *testing.T) {
	outputDir := "out/test-gather-empty-namespaces"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces=", "",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err == nil {
		t.Errorf("kubectl-gather should fail, but it succeeded")
	}

	validateNoClusterDir(t, outputDir)
}

func TestGatherEmptyNamespacesClusterFalse(t *testing.T) {
	outputDir := "out/test-gather-empty-namespaces-cluster-false"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces=", "",
		"--cluster=false",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err == nil {
		t.Errorf("kubectl-gather should fail, but it succeeded")
	}

	validateNoClusterDir(t, outputDir)
}

func TestGatherEmptyNamespacesClusterTrue(t *testing.T) {
	outputDir := "out/test-gather-empty-namespaces-cluster-true"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces=", "",
		"--cluster=true",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
	)

	validate.Missing(t, outputDir, clusters.Names,
		commonNamespacedResources,
		commonLogResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C1},
		c1NamespaceResources,
		c1LogResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C2},
		c2NamespaceResources,
		c2LogResources,
	)
}

func TestGatherSpecificNamespaces(t *testing.T) {
	outputDir := "out/test-gather-specific-namespaces"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateSpecificNamespaces(t, outputDir)
}

func TestGatherSpecificNamespacesClusterFalse(t *testing.T) {
	outputDir := "out/test-gather-specific-namespaces-cluster-false"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1",
		"--cluster=false",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateSpecificNamespaces(t, outputDir)
}

func TestGatherSpecificNamespacesClusterTrue(t *testing.T) {
	outputDir := "out/test-gather-specific-namespaces-cluster-true"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1",
		"--cluster=true",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
		commonNamespacedResources,
		commonLogResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C2},
		c2NamespaceResources,
		c2LogResources,
	)
}

func TestGatherAddonsLogs(t *testing.T) {
	outputDir := "out/test-gather-addons-logs"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons", "logs",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names,
		commonLogResources,
		commonClusterResources,
		commonNamespacedResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1LogResources,
		c1ClusterResources,
		c1NamespaceResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C2},
		c2LogResources,
		c2ClusterResources,
		c2NamespaceResources,
	)

	validate.Missing(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonPVCResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C1},
		c1PVCResources,
		c2PVCResources,
	)
}

func TestGatherAddonsPVCs(t *testing.T) {
	outputDir := "out/test-gather-addons-pvcs"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons", "pvcs",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonPVCResources,
		commonClusterResources,
		commonNamespacedResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1PVCResources,
		c1ClusterResources,
		c1NamespaceResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C2},
		c2PVCResources,
		c2ClusterResources,
		c2NamespaceResources,
	)

	validate.Missing(t, outputDir, clusters.Names,
		commonLogResources,
		c1LogResources,
		c2LogResources,
	)
}

func TestGatherAddonsEmpty(t *testing.T) {
	outputDir := "out/test-gather-addons-empty"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons=",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names,
		commonClusterResources,
		commonNamespacedResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1ClusterResources,
		c1NamespaceResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C2},
		c2ClusterResources,
		c2NamespaceResources,
	)

	validate.Missing(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonLogResources,
		commonPVCResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C1},
		c1LogResources,
		c1PVCResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C2},
		c2LogResources,
		c2PVCResources,
	)
}

func TestJSONLogs(t *testing.T) {
	outputDir := "out/test-json-logs"
	logPath := filepath.Join(outputDir, "gather.log")

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--directory", outputDir,
		"--log-format", "json",
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.JSONLog(t, logPath)
}

// Test helpers

func validateGatherAll(t *testing.T, outputDir string) {
	validate.Exists(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
		commonNamespacedResources,
		commonLogResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
		c2NamespaceResources,
		c2LogResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C1},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
		c2NamespaceResources,
		c2LogResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C2},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)
}

func validateSpecificNamespaces(t *testing.T, outputDir string) {
	validate.Exists(t, outputDir, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
		commonNamespacedResources,
	)

	validate.Exists(t, outputDir, []string{clusters.C1},
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
	)

	validate.Missing(t, outputDir, []string{clusters.C2},
		c2ClusterResources,
		c2PVCResources,
		c2NamespaceResources,
	)
}

func validateNoClusterDir(t *testing.T, outputDir string) {
	for _, cluster := range clusters.Names {
		clusterDir := filepath.Join(outputDir, cluster)
		if validate.PathExists(t, clusterDir) {
			t.Errorf("cluster directory %q should not be created", clusterDir)
		}
	}
}
