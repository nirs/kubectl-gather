// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// In-cluster config (Name: "in-cluster") is tested via e2e since
// rest.InClusterConfig() requires service account files on disk.

// kubectl gather --kubeconfig my-kubeconfig
func TestLoadCurrentContext(t *testing.T) {
	dir := t.TempDir()

	f := writeKubeconfig(t, dir, "config", `
apiVersion: v1
kind: Config
current-context: my-cluster
clusters:
- cluster:
    server: https://my-cluster.example.com:6443
  name: my-cluster
contexts:
- context:
    cluster: my-cluster
    user: admin
  name: my-cluster
users:
- name: admin
  user:
    token: token1
`)

	configs, err := Load(f, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	if configs[0].Name != "my-cluster" {
		t.Errorf("expected name %q, got %q", "my-cluster", configs[0].Name)
	}
	if configs[0].Context != "my-cluster" {
		t.Errorf("expected context %q, got %q", "my-cluster", configs[0].Context)
	}
	if configs[0].Kubeconfig != f {
		t.Errorf("expected kubeconfig %q, got %q", f, configs[0].Kubeconfig)
	}
	if configs[0].Config.Host != "https://my-cluster.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://my-cluster.example.com:6443", configs[0].Config.Host)
	}
}

// kubectl gather
func TestLoadDefaultKubeconfig(t *testing.T) {
	dir := t.TempDir()

	writeKubeconfig(t, dir, "config", `
apiVersion: v1
kind: Config
current-context: default-ctx
clusters:
- cluster:
    server: https://default.example.com:6443
  name: default-cluster
contexts:
- context:
    cluster: default-cluster
    user: admin
  name: default-ctx
users:
- name: admin
  user:
    token: token1
`)

	t.Setenv("KUBECONFIG", filepath.Join(dir, "config"))

	configs, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	if configs[0].Name != "default-ctx" {
		t.Errorf("expected name %q, got %q", "default-ctx", configs[0].Name)
	}
	if configs[0].Context != "default-ctx" {
		t.Errorf("expected context %q, got %q", "default-ctx", configs[0].Context)
	}
	if configs[0].Kubeconfig != filepath.Join(dir, "config") {
		t.Errorf("expected kubeconfig %q, got %q", filepath.Join(dir, "config"), configs[0].Kubeconfig)
	}
	if configs[0].Config.Host != "https://default.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://default.example.com:6443", configs[0].Config.Host)
	}
}

// kubectl gather --contexts dr1,dr2
func TestLoadDefaultKubeconfigWithContexts(t *testing.T) {
	dir := t.TempDir()

	writeKubeconfig(t, dir, "config", `
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

	t.Setenv("KUBECONFIG", filepath.Join(dir, "config"))

	configs, err := Load("", []string{"dr1", "dr2"})
	if err != nil {
		t.Fatal(err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	if configs[0].Name != "dr1" {
		t.Errorf("expected name %q, got %q", "dr1", configs[0].Name)
	}
	if configs[0].Context != "dr1" {
		t.Errorf("expected context %q, got %q", "dr1", configs[0].Context)
	}
	if configs[0].Kubeconfig != filepath.Join(dir, "config") {
		t.Errorf("expected kubeconfig %q, got %q", filepath.Join(dir, "config"), configs[0].Kubeconfig)
	}
	if configs[0].Config.Host != "https://dr1.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://dr1.example.com:6443", configs[0].Config.Host)
	}

	if configs[1].Name != "dr2" {
		t.Errorf("expected name %q, got %q", "dr2", configs[1].Name)
	}
	if configs[1].Context != "dr2" {
		t.Errorf("expected context %q, got %q", "dr2", configs[1].Context)
	}
	if configs[1].Kubeconfig != filepath.Join(dir, "config") {
		t.Errorf("expected kubeconfig %q, got %q", filepath.Join(dir, "config"), configs[1].Kubeconfig)
	}
	if configs[1].Config.Host != "https://dr2.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://dr2.example.com:6443", configs[1].Config.Host)
	}
}

// kubectl gather --kubeconfig my-kubeconfig --contexts hub,dr1,dr2
func TestLoadWithContexts(t *testing.T) {
	dir := t.TempDir()

	f := writeKubeconfig(t, dir, "config", `
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

	configs, err := Load(f, []string{"hub", "dr1", "dr2"})
	if err != nil {
		t.Fatal(err)
	}

	if len(configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(configs))
	}

	if configs[0].Name != "hub" {
		t.Errorf("expected name %q, got %q", "hub", configs[0].Name)
	}
	if configs[0].Context != "hub" {
		t.Errorf("expected context %q, got %q", "hub", configs[0].Context)
	}
	if configs[0].Kubeconfig != f {
		t.Errorf("expected kubeconfig %q, got %q", f, configs[0].Kubeconfig)
	}
	if configs[0].Config.Host != "https://hub.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://hub.example.com:6443", configs[0].Config.Host)
	}

	if configs[1].Name != "dr1" {
		t.Errorf("expected name %q, got %q", "dr1", configs[1].Name)
	}
	if configs[1].Config.Host != "https://dr1.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://dr1.example.com:6443", configs[1].Config.Host)
	}

	if configs[2].Name != "dr2" {
		t.Errorf("expected name %q, got %q", "dr2", configs[2].Name)
	}
	if configs[2].Config.Host != "https://dr2.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://dr2.example.com:6443", configs[2].Config.Host)
	}
}

// kubectl gather --kubeconfig my-kubeconfig
// Error: current-context not set and no --contexts specified.
func TestLoadNoCurrentContext(t *testing.T) {
	dir := t.TempDir()

	f := writeKubeconfig(t, dir, "config", `
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

	_, err := Load(f, nil)
	if err == nil {
		t.Fatal("expected error for missing current-context, got nil")
	}
}

// kubectl gather --kubeconfig hub.yaml,spoke1.yaml
func TestLoadMultipleTwoFiles(t *testing.T) {
	dir := t.TempDir()

	f1 := writeKubeconfig(t, dir, "hub.yaml", `
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

	f2 := writeKubeconfig(t, dir, "spoke1.yaml", `
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

	configs, err := LoadMultiple([]string{f1, f2})
	if err != nil {
		t.Fatal(err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	if configs[0].Name != "hub" {
		t.Errorf("expected name %q, got %q", "hub", configs[0].Name)
	}
	if configs[0].Context != "" {
		t.Errorf("expected empty context, got %q", configs[0].Context)
	}
	if configs[0].Kubeconfig != f1 {
		t.Errorf("expected kubeconfig %q, got %q", f1, configs[0].Kubeconfig)
	}
	if configs[0].Config.Host != "https://hub.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://hub.example.com:6443", configs[0].Config.Host)
	}

	if configs[1].Name != "spoke1" {
		t.Errorf("expected name %q, got %q", "spoke1", configs[1].Name)
	}
	if configs[1].Context != "" {
		t.Errorf("expected empty context, got %q", configs[1].Context)
	}
	if configs[1].Kubeconfig != f2 {
		t.Errorf("expected kubeconfig %q, got %q", f2, configs[1].Kubeconfig)
	}
	if configs[1].Config.Host != "https://spoke1.example.com:6443" {
		t.Errorf("expected host %q, got %q", "https://spoke1.example.com:6443", configs[1].Config.Host)
	}
}

// kubectl gather --kubeconfig cluster1.yaml
// File has duplicate entries from multiple oc login sessions.
func TestLoadMultipleDuplicateEntries(t *testing.T) {
	dir := t.TempDir()

	// File with duplicate context names — current-context selects the right one.
	f1 := writeKubeconfig(t, dir, "cluster1.yaml", `
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

	configs, err := LoadMultiple([]string{f1})
	if err != nil {
		t.Fatal(err)
	}

	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	if configs[0].Config.Host != "https://current.example.com:6443" {
		t.Errorf("expected host from current-context, got %q", configs[0].Config.Host)
	}
}

// kubectl gather --kubeconfig dir1/cluster.yaml,dir2/cluster.yaml
// Error: duplicate basenames.
func TestLoadMultipleDuplicateBasenames(t *testing.T) {
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
	f1 := writeKubeconfig(t, dir1, "cluster.yaml", content)
	f2 := writeKubeconfig(t, dir2, "cluster.yaml", content)

	_, err := LoadMultiple([]string{f1, f2})
	if err == nil {
		t.Fatal("expected error for duplicate basenames, got nil")
	}
}

// kubectl gather --kubeconfig multi.yaml
// Error: current-context not set.
func TestLoadMultipleMissingCurrentContext(t *testing.T) {
	dir := t.TempDir()

	f1 := writeKubeconfig(t, dir, "multi.yaml", `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://a.example.com:6443
  name: cluster-a
- cluster:
    server: https://b.example.com:6443
  name: cluster-b
contexts:
- context:
    cluster: cluster-a
    user: admin-a
  name: ctx-a
- context:
    cluster: cluster-b
    user: admin-b
  name: ctx-b
users:
- name: admin-a
  user:
    token: token-a
- name: admin-b
  user:
    token: token-b
`)

	_, err := LoadMultiple([]string{f1})
	if err == nil {
		t.Fatal("expected error for missing current-context with multiple contexts, got nil")
	}
}

func TestDeriveName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"ocp/hub.yaml", "hub"},
		{"ocp/hub.yml", "hub"},
		{"/home/user/configs/my-cluster.kubeconfig", "my-cluster"},
		{"cluster:6443.yaml", "cluster-6443"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		got := deriveName(tt.path)
		if got != tt.want {
			t.Errorf("deriveName(%q) = %q, want %q", tt.path, got, tt.want)
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
