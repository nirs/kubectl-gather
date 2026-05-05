// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// In-cluster config is tested via e2e since rest.InClusterConfig() requires
// service account files on disk.

type expectedConfig struct {
	Name       string
	Context    string
	Kubeconfig string
	Host       string
}

// kubectl gather (KUBECONFIG=hub.yaml, no --kubeconfig flag)
// Name derived from filename since current context is not filesystem-safe.
func TestLoadDefaultKubeconfigUnsafeContext(t *testing.T) {
	dir := t.TempDir()
	hubPath := writeKubeconfig(t, dir, "hub.yaml", `
apiVersion: v1
kind: Config
current-context: admin/api-hub-example-com:6443/system:admin
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub-cluster
contexts:
- context:
    cluster: hub-cluster
    user: admin
  name: admin/api-hub-example-com:6443/system:admin
users:
- name: admin
  user:
    token: token1
`)
	t.Setenv("KUBECONFIG", hubPath)

	configs, err := Load(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkConfigs(t, configs, []expectedConfig{
		{
			Name:       "hub",
			Host:       "https://hub.example.com:6443",
			Kubeconfig: hubPath,
		},
	})
}

// Name is the current context since it is already filesystem-safe.
// Context is empty to use the current context.
func TestLoadDefaultKubeconfigSafeContext(t *testing.T) {
	dir := t.TempDir()
	configPath := writeKubeconfig(t, dir, "config", `
apiVersion: v1
kind: Config
current-context: minikube
clusters:
- cluster:
    server: https://192.168.49.2:8443
  name: minikube
contexts:
- context:
    cluster: minikube
    user: minikube
  name: minikube
users:
- name: minikube
  user:
    token: token1
`)
	t.Setenv("KUBECONFIG", configPath)

	configs, err := Load(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkConfigs(t, configs, []expectedConfig{
		{
			Name:       "minikube",
			Host:       "https://192.168.49.2:8443",
			Kubeconfig: configPath,
		},
	})
}

// kubectl gather --contexts ...
// Name is the sanitized context name. Context is the raw value for child processes.
func TestLoadFromContexts(t *testing.T) {
	// OCP clusters: context names are auto-generated and need sanitization.
	t.Run("openshift", func(t *testing.T) {
		dir := t.TempDir()
		configPath := writeKubeconfig(t, dir, "config", `
apiVersion: v1
kind: Config
current-context: admin/api-hub-example-com:6443/system:admin
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub-cluster
- cluster:
    server: https://dr1.example.com:6443
  name: dr1-cluster
- cluster:
    server: https://dr2.example.com:6443
  name: dr2-cluster
contexts:
- context:
    cluster: hub-cluster
    user: admin
  name: admin/api-hub-example-com:6443/system:admin
- context:
    cluster: dr1-cluster
    user: admin
  name: admin/api-dr1-example-com:6443/system:admin
- context:
    cluster: dr2-cluster
    user: admin
  name: admin/api-dr2-example-com:6443/system:admin
users:
- name: admin
  user:
    token: token1
`)
		t.Setenv("KUBECONFIG", configPath)

		contexts := []string{
			"admin/api-hub-example-com:6443/system:admin",
			"admin/api-dr1-example-com:6443/system:admin",
			"admin/api-dr2-example-com:6443/system:admin",
		}
		configs, err := Load(contexts, nil)
		if err != nil {
			t.Fatal(err)
		}
		checkConfigs(t, configs, []expectedConfig{
			{
				Name:       "admin-api-hub-example-com-6443-system-admin",
				Context:    "admin/api-hub-example-com:6443/system:admin",
				Host:       "https://hub.example.com:6443",
				Kubeconfig: configPath,
			},
			{
				Name:       "admin-api-dr1-example-com-6443-system-admin",
				Context:    "admin/api-dr1-example-com:6443/system:admin",
				Host:       "https://dr1.example.com:6443",
				Kubeconfig: configPath,
			},
			{
				Name:       "admin-api-dr2-example-com-6443-system-admin",
				Context:    "admin/api-dr2-example-com:6443/system:admin",
				Host:       "https://dr2.example.com:6443",
				Kubeconfig: configPath,
			},
		})
	})

	// Minikube/drenv clusters: context names are clean and used as-is.
	t.Run("drenv", func(t *testing.T) {
		dir := t.TempDir()
		configPath := writeKubeconfig(t, dir, "config", `
apiVersion: v1
kind: Config
current-context: hub
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub-cluster
- cluster:
    server: https://dr1.example.com:6443
  name: dr1-cluster
- cluster:
    server: https://dr2.example.com:6443
  name: dr2-cluster
contexts:
- context:
    cluster: hub-cluster
    user: admin
  name: hub
- context:
    cluster: dr1-cluster
    user: admin
  name: dr1
- context:
    cluster: dr2-cluster
    user: admin
  name: dr2
users:
- name: admin
  user:
    token: token1
`)

		configs, err := Load([]string{"hub", "dr1", "dr2"}, []string{configPath})
		if err != nil {
			t.Fatal(err)
		}
		checkConfigs(t, configs, []expectedConfig{
			{
				Name:       "hub",
				Context:    "hub",
				Host:       "https://hub.example.com:6443",
				Kubeconfig: configPath,
			},
			{
				Name:       "dr1",
				Context:    "dr1",
				Host:       "https://dr1.example.com:6443",
				Kubeconfig: configPath,
			},
			{
				Name:       "dr2",
				Context:    "dr2",
				Host:       "https://dr2.example.com:6443",
				Kubeconfig: configPath,
			},
		})
	})
}

// kubectl gather --kubeconfig hub.yaml,spoke1.yaml
// Name derived from filename. Context is empty to use each file's current context.
func TestLoadFromMultipleKubeconfigs(t *testing.T) {
	dir := t.TempDir()

	hubPath := writeKubeconfig(t, dir, "hub.yaml", `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub-cluster
contexts:
- context:
    cluster: hub-cluster
    user: admin
  name: admin
users:
- name: admin
  user:
    token: token1
`)

	spoke1Path := writeKubeconfig(t, dir, "spoke1.yaml", `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://spoke1.example.com:6443
  name: spoke-cluster
contexts:
- context:
    cluster: spoke-cluster
    user: admin
  name: admin
users:
- name: admin
  user:
    token: token2
`)

	configs, err := Load(nil, []string{hubPath, spoke1Path})
	if err != nil {
		t.Fatal(err)
	}
	checkConfigs(t, configs, []expectedConfig{
		{
			Name:       "hub",
			Host:       "https://hub.example.com:6443",
			Kubeconfig: hubPath,
		},
		{
			Name:       "spoke1",
			Host:       "https://spoke1.example.com:6443",
			Kubeconfig: spoke1Path,
		},
	})
}

// kubectl gather --kubeconfig cluster1.yaml
// Name derived from filename. Uses current-context to select the right cluster
// from a file with stale entries.
func TestLoadFromKubeconfigSafeContext(t *testing.T) {
	dir := t.TempDir()

	cluster1Path := writeKubeconfig(t, dir, "cluster1.yaml", `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://current.example.com:6443
  name: current-cluster
- cluster:
    server: https://old.example.com:6443
  name: old-cluster
contexts:
- context:
    cluster: current-cluster
    user: admin
  name: admin
- context:
    cluster: old-cluster
    user: old-admin
  name: old-admin
users:
- name: admin
  user:
    token: current-token
- name: old-admin
  user:
    token: old-token
`)

	configs, err := Load(nil, []string{cluster1Path})
	if err != nil {
		t.Fatal(err)
	}
	checkConfigs(t, configs, []expectedConfig{
		{
			Name:       "cluster1",
			Host:       "https://current.example.com:6443",
			Kubeconfig: cluster1Path,
		},
	})
}

// kubectl gather --kubeconfig hub.yaml
// Name derived from filename since current context is not filesystem-safe.
func TestLoadFromKubeconfigUnsafeContext(t *testing.T) {
	dir := t.TempDir()
	hubPath := writeKubeconfig(t, dir, "hub.yaml", `
apiVersion: v1
kind: Config
current-context: admin/api-hub-example-com:6443/system:admin
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub-cluster
contexts:
- context:
    cluster: hub-cluster
    user: admin
  name: admin/api-hub-example-com:6443/system:admin
users:
- name: admin
  user:
    token: token1
`)

	configs, err := Load(nil, []string{hubPath})
	if err != nil {
		t.Fatal(err)
	}
	checkConfigs(t, configs, []expectedConfig{
		{
			Name:       "hub",
			Host:       "https://hub.example.com:6443",
			Kubeconfig: hubPath,
		},
	})
}

// TestDeriveName verifies that names derived from filenames are sanitized to safe directory names.
func TestDeriveNameFromFilename(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"ocp/hub.yaml", "hub"},
		{"ocp/hub.yml", "hub"},
		{"/home/user/configs/my-cluster.kubeconfig", "my-cluster"},
		{"cluster:6443.yaml", "cluster-6443"},
		{"plain", "plain"},
		{".hidden.yaml", "hidden"},
		{"my.cluster.yaml", "my-cluster"},
	}
	for _, tt := range tests {
		got := nameFromFilename(tt.path)
		if got != tt.want {
			t.Errorf("nameFromFilename(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// --- negative cases ---

// kubectl gather --contexts admin,viewer
// Error: both contexts point to the same cluster.
func TestLoadFromContextsDuplicateHost(t *testing.T) {
	dir := t.TempDir()
	configPath := writeKubeconfig(t, dir, "config", `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub
contexts:
- context:
    cluster: hub
    user: admin
  name: admin
- context:
    cluster: hub
    user: viewer
  name: viewer
users:
- name: admin
  user:
    token: admin-token
- name: viewer
  user:
    token: viewer-token
`)

	_, err := Load([]string{"admin", "viewer"}, []string{configPath})
	if err == nil {
		t.Fatal("expected error for duplicate host, got nil")
	}
}

// kubectl gather --kubeconfig my-kubeconfig
// Error: current-context not set and no --contexts specified.
func TestLoadFromKubconfigsNoCurrentContext(t *testing.T) {
	dir := t.TempDir()
	configPath := writeKubeconfig(t, dir, "config", `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com:6443
  name: cluster
contexts:
- context:
    cluster: cluster
    user: admin
  name: ctx
users:
- name: admin
  user:
    token: token
`)

	_, err := Load(nil, []string{configPath})
	if err == nil {
		t.Fatal("expected error for missing current-context, got nil")
	}
}

// kubectl gather --kubeconfig dir1/cluster.yaml,dir2/cluster.yaml
// Error: duplicate basenames.
func TestLoadFromKubeconfigsDuplicateBasenames(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	content := `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://example.com:6443
  name: cluster
contexts:
- context:
    cluster: cluster
    user: admin
  name: admin
users:
- name: admin
  user:
    token: token
`
	cluster1Path := writeKubeconfig(t, dir1, "cluster.yaml", content)
	cluster2Path := writeKubeconfig(t, dir2, "cluster.yaml", content)

	_, err := Load(nil, []string{cluster1Path, cluster2Path})
	if err == nil {
		t.Fatal("expected error for duplicate basenames, got nil")
	}
}

// kubectl gather --kubeconfig .hidden.yaml,hidden.yaml
// Error: names collide after sanitization.
func TestLoadFromKubeconfigsSanitizedCollision(t *testing.T) {
	dir := t.TempDir()

	content := `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://example.com:6443
  name: cluster
contexts:
- context:
    cluster: cluster
    user: admin
  name: admin
users:
- name: admin
  user:
    token: token
`
	hiddenPath := writeKubeconfig(t, dir, ".hidden.yaml", content)
	plainPath := writeKubeconfig(t, dir, "hidden.yaml", content)

	_, err := Load(nil, []string{hiddenPath, plainPath})
	if err == nil {
		t.Fatal("expected error for sanitized name collision, got nil")
	}
}

// kubectl gather --kubeconfig hub.yaml,hub-backup.yaml
// Error: both files point to the same cluster.
func TestLoadFromKubeconfigsDuplicateHost(t *testing.T) {
	dir := t.TempDir()

	hub1Path := writeKubeconfig(t, dir, "hub.yaml", `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub
contexts:
- context:
    cluster: hub
    user: admin
  name: admin
users:
- name: admin
  user:
    token: token1
`)

	hub2Path := writeKubeconfig(t, dir, "hub-backup.yaml", `
apiVersion: v1
kind: Config
current-context: admin
clusters:
- cluster:
    server: https://hub.example.com:6443
  name: hub
contexts:
- context:
    cluster: hub
    user: admin
  name: admin
users:
- name: admin
  user:
    token: token2
`)

	_, err := Load(nil, []string{hub1Path, hub2Path})
	if err == nil {
		t.Fatal("expected error for duplicate host, got nil")
	}
}

// --- helpers ---

func checkConfigs(t *testing.T, got []*Config, want []expectedConfig) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d configs, got %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i].Name != w.Name {
			t.Errorf("configs[%d].Name = %q, want %q", i, got[i].Name, w.Name)
		}
		if got[i].Context != w.Context {
			t.Errorf("configs[%d].Context = %q, want %q", i, got[i].Context, w.Context)
		}
		if got[i].Kubeconfig != w.Kubeconfig {
			t.Errorf("configs[%d].Kubeconfig = %q, want %q", i, got[i].Kubeconfig, w.Kubeconfig)
		}
		if got[i].Config.Host != w.Host {
			t.Errorf("configs[%d].Config.Host = %q, want %q", i, got[i].Config.Host, w.Host)
		}
	}
}

func writeKubeconfig(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
