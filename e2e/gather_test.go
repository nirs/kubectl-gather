//go:build e2e

package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/e2e/commands"
	"github.com/nirs/kubectl-gather/e2e/test"
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
		"namespaces/test-common/secrets/common-secret1.yaml",
	}

	commonLogResources = []string{
		"namespaces/test-common/pods/common-busybox-*/busybox/current.log",
	}

	commonPVCResources = []string{
		"cluster/persistentvolumes/common-pv1.yaml",
	}

	c1ClusterNodes = []string{
		"cluster/nodes/c1.yaml",
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
		"namespaces/test-c1/secrets/c1-secret1.yaml",
	}

	c1LogResources = []string{
		"namespaces/test-c1/pods/c1-busybox-*/busybox/current.log",
	}

	c1PVCResources = []string{
		"cluster/persistentvolumes/c1-pv1.yaml",
	}

	c2ClusterNodes = []string{
		"cluster/nodes/c2.yaml",
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
		"namespaces/test-c2/secrets/c2-secret1.yaml",
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

func TestGatherLocal(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-local"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateGatherAll(t, validate.New(outputDir))
}

func TestGatherClusterTrue(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-cluster-true"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--cluster=true",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateGatherAll(t, validate.New(outputDir))
}

func TestGatherRemote(dt *testing.T) {
	t := test.WithLog(dt)
	if _, err := exec.LookPath("oc"); err != nil {
		t.Skip("oc not found, skipping remote test")
	}

	outputDir := "out/test-gather-remote"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--remote",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Fatalf("kubectl-gather --remote failed: %s", err)
	}

	dataRoot := findDataRoot(t, filepath.Join(outputDir, clusters.C1))
	t.Debugf("Data root: %s", dataRoot)

	validateGatherAll(t, validate.New(outputDir).WithDataRoot(dataRoot))
}

func TestGatherClusterFalse(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-cluster-false"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--cluster=false",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	v := validate.New(outputDir)

	v.Exists(t, clusters.Names,
		defaultPVCResources,
		commonPVCResources,
		commonNamespacedResources,
		commonLogResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)

	v.Exists(t, []string{clusters.C2},
		c2PVCResources,
		c2NamespaceResources,
		c2LogResources,
	)

	v.Missing(t, clusters.Names,
		commonClusterResources,
	)

	v.Missing(t, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
	)

	v.Missing(t, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
	)
}

func TestGatherEmptyNamespaces(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-empty-namespaces"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces=", "",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err == nil {
		t.Errorf("kubectl-gather should fail, but it succeeded")
	}

	validateNoClusterDir(t, outputDir)
}

func TestGatherEmptyNamespacesClusterFalse(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-empty-namespaces-cluster-false"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces=", "",
		"--cluster=false",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err == nil {
		t.Errorf("kubectl-gather should fail, but it succeeded")
	}

	validateNoClusterDir(t, outputDir)
}

func TestGatherEmptyNamespacesClusterTrue(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-empty-namespaces-cluster-true"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces=", "",
		"--cluster=true",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	v := validate.New(outputDir)

	v.Exists(t, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
	)

	v.Exists(t, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
	)

	v.Missing(t, clusters.Names,
		commonNamespacedResources,
		commonLogResources,
	)

	v.Missing(t, []string{clusters.C1},
		c1NamespaceResources,
		c1LogResources,
	)

	v.Missing(t, []string{clusters.C2},
		c2NamespaceResources,
		c2LogResources,
	)
}

func TestGatherSpecificNamespaces(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-specific-namespaces"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces", "test-common,test-c1",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateSpecificNamespaces(t, validate.New(outputDir))
}

func TestGatherSpecificNamespacesClusterFalse(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-specific-namespaces-cluster-false"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces", "test-common,test-c1",
		"--cluster=false",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validateSpecificNamespaces(t, validate.New(outputDir))
}

func TestGatherSpecificNamespacesClusterTrue(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-specific-namespaces-cluster-true"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces", "test-common,test-c1",
		"--cluster=true",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	v := validate.New(outputDir)

	v.Exists(t, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
		commonNamespacedResources,
		commonLogResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)

	v.Exists(t, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
	)

	v.Missing(t, []string{clusters.C2},
		c2NamespaceResources,
		c2LogResources,
	)
}

func TestGatherAddonsLogs(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-addons-logs"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons", "logs",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	v := validate.New(outputDir)

	v.Exists(t, clusters.Names,
		commonLogResources,
		commonClusterResources,
		commonNamespacedResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1LogResources,
		c1ClusterResources,
		c1NamespaceResources,
	)

	v.Exists(t, []string{clusters.C2},
		c2LogResources,
		c2ClusterResources,
		c2NamespaceResources,
	)

	v.Missing(t, clusters.Names,
		defaultPVCResources,
		commonPVCResources,
	)

	v.Missing(t, []string{clusters.C1},
		c1PVCResources,
		c2PVCResources,
	)
}

func TestGatherAddonsPVCs(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-addons-pvcs"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons", "pvcs",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	v := validate.New(outputDir)

	v.Exists(t, clusters.Names,
		defaultPVCResources,
		commonPVCResources,
		commonClusterResources,
		commonNamespacedResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1PVCResources,
		c1ClusterResources,
		c1NamespaceResources,
	)

	v.Exists(t, []string{clusters.C2},
		c2PVCResources,
		c2ClusterResources,
		c2NamespaceResources,
	)

	v.Missing(t, clusters.Names,
		commonLogResources,
		c1LogResources,
		c2LogResources,
	)
}

func TestGatherAddonsEmpty(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-gather-addons-empty"

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--namespaces", "test-common,test-c1,test-c2",
		"--addons=",
		"--directory", outputDir,
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	v := validate.New(outputDir)

	v.Exists(t, clusters.Names,
		commonClusterResources,
		commonNamespacedResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1ClusterResources,
		c1NamespaceResources,
	)

	v.Exists(t, []string{clusters.C2},
		c2ClusterResources,
		c2NamespaceResources,
	)

	v.Missing(t, clusters.Names,
		defaultPVCResources,
		commonLogResources,
		commonPVCResources,
	)

	v.Missing(t, []string{clusters.C1},
		c1LogResources,
		c1PVCResources,
	)

	v.Missing(t, []string{clusters.C2},
		c2LogResources,
		c2PVCResources,
	)
}

func TestJSONLogs(dt *testing.T) {
	t := test.WithLog(dt)
	outputDir := "out/test-json-logs"
	logPath := filepath.Join(outputDir, "gather.log")

	cmd := exec.Command(
		kubectlGather,
		"--contexts", strings.Join(clusters.Names, ","),
		"--directory", outputDir,
		"--log-format", "json",
	)
	if err := commands.Run(cmd, t.Log); err != nil {
		t.Errorf("kubectl-gather failed: %s", err)
	}

	validate.JSONLog(t, logPath)
}

// Test helpers

func validateGatherAll(t *test.T, v *validate.Validator) {
	t.Helper()

	v.Exists(t, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
		commonNamespacedResources,
		commonLogResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)

	v.Exists(t, []string{clusters.C2},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
		c2NamespaceResources,
		c2LogResources,
	)

	v.Missing(t, []string{clusters.C1},
		c2ClusterNodes,
		c2ClusterResources,
		c2PVCResources,
		c2NamespaceResources,
		c2LogResources,
	)

	v.Missing(t, []string{clusters.C2},
		c1ClusterNodes,
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
		c1LogResources,
	)
}

func validateSpecificNamespaces(t *test.T, v *validate.Validator) {
	t.Helper()

	v.Exists(t, clusters.Names,
		defaultPVCResources,
		commonClusterResources,
		commonPVCResources,
		commonNamespacedResources,
	)

	v.Exists(t, []string{clusters.C1},
		c1ClusterResources,
		c1PVCResources,
		c1NamespaceResources,
	)

	v.Missing(t, []string{clusters.C2},
		c2ClusterResources,
		c2PVCResources,
		c2NamespaceResources,
	)
}

func validateNoClusterDir(t *test.T, outputDir string) {
	for _, cluster := range clusters.Names {
		clusterDir := filepath.Join(outputDir, cluster)
		if validate.PathExists(t, clusterDir) {
			t.Errorf("cluster directory %q should not be created", clusterDir)
		}
	}
}
