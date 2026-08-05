// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
	"github.com/nirs/kubectl-gather/pkg/kubeconfig"
)

func remoteGather(clusterConfigs []*kubeconfig.Config) {
	start := time.Now()

	wg := sync.WaitGroup{}
	errors := make(chan error, len(clusterConfigs))

	for i := range clusterConfigs {
		clusterConfig := clusterConfigs[i]
		clusterDir := filepath.Join(directory, clusterConfig.Name)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runMustGather(clusterConfig, clusterDir); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		log.Fatal(err)
	}

	log.Infof("Gathered %d clusters in %.3f seconds",
		len(clusterConfigs), time.Since(start).Seconds())
}

func runMustGather(config *kubeconfig.Config, directory string) error {
	log.Infof("Gathering on remote cluster %q", config.Name)
	start := time.Now()

	var stderr bytes.Buffer

	cmd := mustGatherCommand(config, directory)
	cmd.Stderr = &stderr

	log.Debugf("Running command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("oc adm must-gather error: %s: %s", err, stderr.String())
	}

	elapsed := time.Since(start).Seconds()
	log.Infof("Gathered on remote cluster %q in %.3f seconds",
		config.Name, elapsed)

	return nil
}

func mustGatherCommand(config *kubeconfig.Config, directory string) *exec.Cmd {
	args := []string{
		"adm",
		"must-gather",
		"--image=" + gather.Image,
		"--dest-dir=" + directory,
	}
	if config.Context != "" {
		args = append(args, "--context="+config.Context)
	}
	if config.Kubeconfig != "" {
		args = append(args, "--kubeconfig="+config.Kubeconfig)
	}

	var remoteArgs []string

	if namespaces != nil {
		remoteArgs = append(remoteArgs, "--namespaces="+strings.Join(namespaces, ","))
	}

	// --namespaces not set, --cluster not set -> cluster=true
	// --namespaces set, --cluster not set -> cluster=false
	if cluster {
		remoteArgs = append(remoteArgs, "--cluster=true")
	} else {
		remoteArgs = append(remoteArgs, "--cluster=false")
	}

	if addons != nil {
		remoteArgs = append(remoteArgs, "--addons="+strings.Join(addons, ","))
	}

	// Always pass the salt so all remote clusters use the same salt value,
	// ensuring consistent hashes for comparing secrets across clusters.
	remoteArgs = append(remoteArgs, "--salt="+salt)

	if workers > 0 {
		remoteArgs = append(remoteArgs, fmt.Sprintf("--workers=%d", workers))
	}

	if len(remoteArgs) > 0 {
		args = append(args, "--", "/usr/bin/gather")
		args = append(args, remoteArgs...)
	}

	return exec.Command("oc", args...)
}
