// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

// Package kubeconfig loads cluster configurations for kubectl-gather.
//
// The returned [Config] values vary by mode:
//
//	Mode                     Name                  Context
//	In-cluster               ""                    ""
//	Current context          see below             ""
//	Explicit --contexts      sanitized context     context
//	Multiple kubeconfigs     sanitized filename    ""
//
// Name is used for the output directory and logging. It is always
// sanitized for cross-platform filesystem compatibility (no ":", "\",
// "/", ".", or leading/trailing "-"). Empty Name means in-cluster mode
// (data goes directly to the gather directory).
//
// Context is passed to child processes as --context. Empty Context means
// use the file's current-context implicitly — no --context flag is passed.
//
// # Naming for current context (no --contexts specified)
//
// When gathering the current context, Name is chosen by checking whether
// the context name is already filesystem-safe. If sanitizeName(context)
// equals the original context, the context name is used — this gives
// meaningful names for minikube/kind clusters (e.g. "dr1", "hub"). If
// the context name requires sanitization (common with OpenShift contexts
// like "admin/api-cluster.example.com:6443/system:admin"), the kubeconfig
// filename is used instead as a shorter, predictable alternative.
//
// Examples:
//
//	kubectl gather --kubeconfig hub.yaml
//	  (current-context: "admin/api-hub:6443/system:admin")
//	  → Name="hub" (context is not clean, use filename), Context=""
//
//	kubectl gather
//	  (default ~/.kube/config, current-context: "dr1")
//	  → Name="dr1" (context is clean, use it), Context=""
//
//	kubectl gather --contexts dr1,dr2
//	  → Name="dr1", Context="dr1"
//	  → Name="dr2", Context="dr2"
//
//	kubectl gather --kubeconfig hub.yaml,c1.yaml,c2.yaml
//	  → Name="hub", Context=""
//	  → Name="c1", Context=""
//	  → Name="c2", Context=""
package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config holds the connection information and metadata for a single cluster.
type Config struct {
	// Config is the Kubernetes REST client configuration for API access.
	Config *rest.Config

	// Name is the cluster name for output directory and logging (empty for in-cluster).
	Name string

	// Context is the context name for child processes (empty to use current-context).
	Context string

	// Kubeconfig is the file path for child processes.
	Kubeconfig string
}

// Load loads cluster configurations based on the provided flags. It routes to
// the appropriate loading strategy:
//   - contexts specified: load the given contexts from a single kubeconfig
//   - kubeconfigs specified: load current-context from each file
//   - neither: try in-cluster config, fallback to default kubeconfig
func Load(contexts, kubeconfigs []string) ([]*Config, error) {
	if len(contexts) > 0 && len(kubeconfigs) > 1 {
		return nil, fmt.Errorf("contexts cannot be used with multiple kubeconfig files")
	}

	if len(contexts) > 0 {
		var kc string
		if len(kubeconfigs) == 1 {
			kc = kubeconfigs[0]
		}
		return loadContexts(contexts, kc)
	}

	if len(kubeconfigs) > 0 {
		return loadKubeconfigs(kubeconfigs)
	}

	configs, err := loadInCluster()
	if err == nil {
		return configs, nil
	}
	if err != rest.ErrNotInCluster {
		return nil, err
	}

	return loadCurrentContext()
}

func loadInCluster() ([]*Config, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return []*Config{{Config: restConfig}}, nil
}

// loadContexts loads explicit contexts from a single kubeconfig file.
func loadContexts(contexts []string, kubeconfig string) ([]*Config, error) {
	if kubeconfig == "" {
		kubeconfig = defaultKubeconfig()
	}

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}

	var configs []*Config
	hosts := make(map[string]string) // host -> context
	for _, context := range contexts {
		restConfig, err := clientcmd.NewNonInteractiveClientConfig(
			*config, context, nil, nil).ClientConfig()
		if err != nil {
			return nil, err
		}
		if prev, ok := hosts[restConfig.Host]; ok {
			return nil, fmt.Errorf("duplicate cluster %q from contexts %q and %q", restConfig.Host, prev, context)
		}
		hosts[restConfig.Host] = context
		configs = append(configs, &Config{
			Config:     restConfig,
			Name:       sanitizeName(context),
			Context:    context,
			Kubeconfig: kubeconfig,
		})
	}
	return configs, nil
}

// loadCurrentContext loads the current-context from the default kubeconfig.
func loadCurrentContext() ([]*Config, error) {
	kubeconfig := defaultKubeconfig()

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	if config.CurrentContext == "" {
		return nil, fmt.Errorf("no context specified and current context not set")
	}

	name := config.CurrentContext
	if sanitizeName(name) != name {
		name = nameFromFilename(kubeconfig)
	}

	restConfig, err := clientcmd.NewNonInteractiveClientConfig(
		*config, config.CurrentContext, nil, nil).ClientConfig()
	if err != nil {
		return nil, err
	}
	return []*Config{{
		Config:     restConfig,
		Name:       name,
		Kubeconfig: kubeconfig,
	}}, nil
}

// loadKubeconfigs loads each kubeconfig file independently, using current-context
// from each file. The cluster name is derived from the filename (without
// directory and extension). Context is left empty since each file's
// current-context is used implicitly. No merging or renaming is done.
func loadKubeconfigs(kubeconfigs []string) ([]*Config, error) {
	names := make(map[string]string) // derived name -> file path
	hosts := make(map[string]string) // host -> file path
	var configs []*Config

	for _, path := range kubeconfigs {
		name := nameFromFilename(path)

		if prev, ok := names[name]; ok {
			return nil, fmt.Errorf("duplicate cluster name %q from files %q and %q", name, prev, path)
		}
		names[name] = path

		config, err := clientcmd.LoadFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading %q: %w", path, err)
		}

		context := config.CurrentContext
		if context == "" {
			return nil, fmt.Errorf("file %q: current-context not set", path)
		}

		restConfig, err := clientcmd.NewNonInteractiveClientConfig(
			*config, context, nil, nil).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("file %q: %w", path, err)
		}

		if prev, ok := hosts[restConfig.Host]; ok {
			return nil, fmt.Errorf("duplicate cluster %q from files %q and %q", restConfig.Host, prev, path)
		}
		hosts[restConfig.Host] = path

		configs = append(configs, &Config{
			Config:     restConfig,
			Name:       name,
			Kubeconfig: path,
		})
	}

	return configs, nil
}

// defaultKubeconfig returns the kubeconfig path from the KUBECONFIG environment
// variable or the default location.
func defaultKubeconfig() string {
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		return env
	}
	return clientcmd.RecommendedHomeFile
}

// nameFromFilename extracts a cluster name from a kubeconfig file path by taking the
// filename without directory and extension, and sanitizing for cross-platform use.
func nameFromFilename(path string) string {
	basename := filepath.Base(path)
	name := strings.TrimSuffix(basename, filepath.Ext(basename))
	return sanitizeName(name)
}

// sanitizeName replaces characters that are problematic in file paths to
// ensure the name works as a directory component on any platform.
func sanitizeName(name string) string {
	name = strings.NewReplacer(
		":", "-", // invalid on Windows
		"\\", "-", // path separator on Windows
		"/", "-", // path separator on Unix
		".", "-", // hidden directory prefix, confusing in paths
	).Replace(name)
	return strings.Trim(name, "-")
}
