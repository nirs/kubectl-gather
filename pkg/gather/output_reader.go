// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"os"
	"path/filepath"
	"strings"
)

type OutputReader struct {
	base string
}

// NewOutputReader creates a new OutputReader instance.
func NewOutputReader(path string) *OutputReader {
	return &OutputReader{base: path}
}

// ListResources lists resource names in namespace.
func (r *OutputReader) ListResources(namespace, resource string) ([]string, error) {
	var resourceDir string
	if namespace == "" {
		resourceDir = filepath.Join(r.base, clusterDir, resource)
	} else {
		resourceDir = filepath.Join(r.base, namespacesDir, namespace, resource)
	}

	entries, err := os.ReadDir(resourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		// Skip pod directory.
		if e.IsDir() {
			continue
		}
		resourceName := strings.TrimSuffix(e.Name(), resourceSuffix)
		names = append(names, resourceName)
	}

	return names, nil
}

// ReadResource reads named resource data.
func (r *OutputReader) ReadResource(namespace, resource, name string) ([]byte, error) {
	var resourcePath string
	if namespace == "" {
		resourcePath = filepath.Join(r.base, clusterDir, resource, name+resourceSuffix)
	} else {
		resourcePath = filepath.Join(r.base, namespacesDir, namespace, resource, name+resourceSuffix)
	}
	return os.ReadFile(resourcePath)
}
