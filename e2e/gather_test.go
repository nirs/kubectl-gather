package e2e_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/e2e/commands"
	"github.com/nirs/kubectl-gather/e2e/validate"
)

const executable = "../kubectl-gather"

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
		executable,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names, defaultPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonClusterResources)
	validate.Exists(t, outputDir, clusters.Names, commonPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonNamespacedResources)
	validate.Exists(t, outputDir, clusters.Names, commonLogResources)

	validate.Exists(t, outputDir, []string{clusters.C1}, c1ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1PVCResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1NamespaceResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1LogResources)

	validate.Exists(t, outputDir, []string{clusters.C2}, c2ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2PVCResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2NamespaceResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2LogResources)

	validate.Missing(t, outputDir, []string{clusters.C1}, c2ClusterResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c2PVCResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c2NamespaceResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c2LogResources)

	validate.Missing(t, outputDir, []string{clusters.C2}, c1ClusterResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c1PVCResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c1NamespaceResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c1LogResources)
}

func TestGatherEmptyNamespaces(t *testing.T) {
	outputDir := "out/test-gather-empty-namespaces"

	cmd := exec.Command(
		executable,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces=", "",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	// TODO: Empty namespace should result in gathering no resources.
	validate.Exists(t, outputDir, clusters.Names, defaultPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonClusterResources)
	validate.Exists(t, outputDir, clusters.Names, commonPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonNamespacedResources)
	validate.Exists(t, outputDir, clusters.Names, commonLogResources)

	validate.Exists(t, outputDir, []string{clusters.C1}, c1ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1PVCResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1NamespaceResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1LogResources)

	validate.Exists(t, outputDir, []string{clusters.C2}, c2ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2PVCResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2NamespaceResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2LogResources)

	validate.Missing(t, outputDir, []string{clusters.C1}, c2ClusterResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c2PVCResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c2NamespaceResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c2LogResources)

	validate.Missing(t, outputDir, []string{clusters.C2}, c1ClusterResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c1PVCResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c1NamespaceResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c1LogResources)
}

func TestGatherSpecificNamespaces(t *testing.T) {
	outputDir := "out/test-gather-specific-namespaces"

	cmd := exec.Command(
		executable,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names, defaultPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonClusterResources)
	validate.Exists(t, outputDir, clusters.Names, commonPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonNamespacedResources)

	validate.Exists(t, outputDir, []string{clusters.C1}, c1ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1PVCResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1NamespaceResources)

	validate.Missing(t, outputDir, []string{clusters.C2}, c2ClusterResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c2PVCResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c2NamespaceResources)
}

func TestGatherAddonsLogs(t *testing.T) {
	outputDir := "out/test-gather-addons-logs"

	cmd := exec.Command(
		executable,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons", "logs",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names, commonLogResources)
	validate.Exists(t, outputDir, clusters.Names, commonClusterResources)
	validate.Exists(t, outputDir, clusters.Names, commonNamespacedResources)

	validate.Exists(t, outputDir, []string{clusters.C1}, c1LogResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1NamespaceResources)

	validate.Exists(t, outputDir, []string{clusters.C2}, c2LogResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2NamespaceResources)

	validate.Missing(t, outputDir, clusters.Names, defaultPVCResources)
	validate.Missing(t, outputDir, clusters.Names, commonPVCResources)

	validate.Missing(t, outputDir, []string{clusters.C1}, c1PVCResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c2PVCResources)
}

func TestGatherAddonsPVCs(t *testing.T) {
	outputDir := "out/test-gather-addons-pvcs"

	cmd := exec.Command(
		executable,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons", "pvcs",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Exists(t, outputDir, clusters.Names, defaultPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonPVCResources)
	validate.Exists(t, outputDir, clusters.Names, commonClusterResources)
	validate.Exists(t, outputDir, clusters.Names, commonNamespacedResources)

	validate.Exists(t, outputDir, []string{clusters.C1}, c1PVCResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1NamespaceResources)

	validate.Exists(t, outputDir, []string{clusters.C2}, c2PVCResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2NamespaceResources)

	validate.Missing(t, outputDir, clusters.Names, commonLogResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c1LogResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c2LogResources)
}

func TestGatherAddonsEmpty(t *testing.T) {
	outputDir := "out/test-gather-addons-empty"

	cmd := exec.Command(
		executable,
		"--contexts", strings.Join(clusters.Names, ","),
		"--kubeconfig", clusters.Kubeconfig(),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons=",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.Missing(t, outputDir, clusters.Names, defaultPVCResources)
	validate.Missing(t, outputDir, clusters.Names, commonLogResources)
	validate.Missing(t, outputDir, clusters.Names, commonPVCResources)

	validate.Missing(t, outputDir, []string{clusters.C1}, c1LogResources)
	validate.Missing(t, outputDir, []string{clusters.C1}, c1PVCResources)

	validate.Missing(t, outputDir, []string{clusters.C2}, c2LogResources)
	validate.Missing(t, outputDir, []string{clusters.C2}, c2PVCResources)

	validate.Exists(t, outputDir, clusters.Names, commonClusterResources)
	validate.Exists(t, outputDir, clusters.Names, commonNamespacedResources)

	validate.Exists(t, outputDir, []string{clusters.C1}, c1ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C1}, c1NamespaceResources)

	validate.Exists(t, outputDir, []string{clusters.C2}, c2ClusterResources)
	validate.Exists(t, outputDir, []string{clusters.C2}, c2NamespaceResources)
}

func TestJSONLogs(t *testing.T) {
	outputDir := "out/test-json-logs"
	logPath := filepath.Join(outputDir, "gather.log")

	cmd := exec.Command(
		executable,
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
