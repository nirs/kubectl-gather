// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"io"
	"os"
	"path/filepath"
)

const (
	namespacesDir  = "namespaces"
	clusterDir     = "cluster"
	addonsDir      = "addons"
	resourceSuffix = ".yaml"
	logSuffix      = ".log"
)

type OutputDirectory struct {
	base string
}

func (o *OutputDirectory) CreateContainerLog(namespace string, pod string, container string, name string) (io.WriteCloser, error) {
	dir, err := createDirectory(o.base, namespacesDir, namespace, "pods", pod, container)
	if err != nil {
		return nil, err
	}
	return createFile(dir, name+logSuffix)
}

func (o *OutputDirectory) CreateNamespacedResource(namespace string, resource string, name string) (io.WriteCloser, error) {
	dir, err := createDirectory(o.base, namespacesDir, namespace, resource)
	if err != nil {
		return nil, err
	}
	return createFile(dir, name+resourceSuffix)
}

func (o *OutputDirectory) CreateClusterResource(resource string, name string) (io.WriteCloser, error) {
	dir, err := createDirectory(o.base, clusterDir, resource)
	if err != nil {
		return nil, err
	}
	return createFile(dir, name+resourceSuffix)
}

func (o *OutputDirectory) CreateAddonDir(name string, more ...string) (string, error) {
	args := append([]string{o.base, addonsDir, name}, more...)
	return createDirectory(args...)
}

func createDirectory(args ...string) (string, error) {
	dir := filepath.Join(args...)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}

	return dir, nil
}

func createFile(dir string, name string) (io.WriteCloser, error) {
	filename := filepath.Join(dir, name)
	return os.Create(filename)
}
