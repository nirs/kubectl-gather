// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

// Package kubeconfig loads cluster configurations for kubectl-gather.
//
// The returned [Config] values vary by mode:
//
//	Mode                  Name              Context
//	In-cluster            ""                ""
//	Single kubeconfig     context name      context name
//	Multiple kubeconfigs  filename          ""
//
// Name is used for the output directory and logging. Empty Name means
// in-cluster mode (data goes directly to the gather directory).
// Context is passed to child processes as --context. Empty Context means
// use the file's current-context implicitly.
package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Config struct {
	Config     *rest.Config
	Name       string // cluster name for output directory and logging (empty for in-cluster)
	Context    string // context name for child processes (empty to use current-context)
	Kubeconfig string // kubeconfig file path for child processes
}

// Load loads cluster configurations from a single kubeconfig file. If contexts
// is empty and running in-cluster, the in-cluster config is returned. If
// contexts is empty and not in-cluster, current-context from the file is used.
func Load(kubeconfig string, contexts []string) ([]*Config, error) {
	if len(contexts) == 0 {
		restConfig, err := rest.InClusterConfig()
		if err != rest.ErrNotInCluster {
			if err != nil {
				return nil, err
			}
			return []*Config{{Config: restConfig, Kubeconfig: kubeconfig}}, nil
		}
	}

	if kubeconfig == "" {
		kubeconfig = DefaultKubeconfig()
	}

	config, err := loadFile(kubeconfig)
	if err != nil {
		return nil, err
	}

	if len(contexts) == 0 {
		if config.CurrentContext == "" {
			return nil, fmt.Errorf("no context specified and current context not set")
		}
		contexts = []string{config.CurrentContext}
	}

	var configs []*Config

	for _, context := range contexts {
		restConfig, err := clientcmd.NewNonInteractiveClientConfig(
			*config, context, nil, nil).ClientConfig()
		if err != nil {
			return nil, err
		}
		configs = append(configs, &Config{
			Config:     restConfig,
			Name:       context,
			Context:    context,
			Kubeconfig: kubeconfig,
		})
	}

	return configs, nil
}

// LoadMultiple loads each kubeconfig file independently, using current-context
// from each file. The cluster name is derived from the filename (without
// directory and extension). Context is left empty since each file's
// current-context is used implicitly. No merging or renaming is done.
func LoadMultiple(kubeconfigs []string) ([]*Config, error) {
	seen := make(map[string]string) // derived name -> file path
	var configs []*Config

	for _, path := range kubeconfigs {
		name := deriveName(path)

		if prev, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate cluster name %q from files %q and %q", name, prev, path)
		}
		seen[name] = path

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

		configs = append(configs, &Config{
			Config:     restConfig,
			Name:       name,
			Kubeconfig: path,
		})
	}

	return configs, nil
}

// DefaultKubeconfig returns the kubeconfig path from the KUBECONFIG environment
// variable or the default location.
func DefaultKubeconfig() string {
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		return env
	}
	return clientcmd.RecommendedHomeFile
}

func loadFile(kubeconfig string) (*api.Config, error) {
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// deriveName extracts a cluster name from a kubeconfig file path by taking the
// filename without directory and extension, and sanitizing for cross-platform use.
func deriveName(path string) string {
	basename := filepath.Base(path)
	name := strings.TrimSuffix(basename, filepath.Ext(basename))
	return sanitizeName(name)
}

// sanitizeName replaces characters that are problematic in file paths on
// other platforms. The name comes from filepath.Base so it cannot contain
// path separators.
func sanitizeName(name string) string {
	return strings.NewReplacer(
		":", "-", // invalid on Windows
		"\\", "-", // valid on Linux but path separator on Windows
	).Replace(name)
}
