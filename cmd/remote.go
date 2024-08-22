// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const remoteDefaultImage = "quay.io/nirsof/gather:0.5.1"

func remoteGather(clusters []*clusterConfig) {
	start := time.Now()

	wg := sync.WaitGroup{}
	errors := make(chan error, len(clusters))

	for i := range clusters {
		cluster := clusters[i]
		directory := filepath.Join(directory, cluster.Context)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runMustGather(cluster.Context, directory); err != nil {
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
		len(clusters), time.Since(start).Seconds())
}

func runMustGather(context string, directory string) error {
	log.Infof("Gathering on remote cluster %q", context)
	start := time.Now()

	logfile, err := createMustGatherLog(directory)
	if err != nil {
		return err
	}

	defer logfile.Close()

	var stderr bytes.Buffer

	cmd := mustGatherCommand(context, directory)
	cmd.Stdout = logfile
	cmd.Stderr = &stderr

	log.Debugf("Running command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("oc adm must-gather error: %s: %s", err, stderr.String())
	}

	elapsed := time.Since(start).Seconds()
	log.Infof("Gathered on remote cluster %q in %.3f seconds",
		context, elapsed)

	return nil
}

func createMustGatherLog(directory string) (*os.File, error) {
	if err := os.MkdirAll(directory, 0750); err != nil {
		return nil, err
	}

	return os.Create(filepath.Join(directory, "must-gather.log"))
}

func mustGatherCommand(context string, directory string) *exec.Cmd {
	args := []string{
		"adm",
		"must-gather",
		"--image=" + remoteDefaultImage,
		"--context=" + context,
		"--dest-dir=" + directory,
	}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig="+kubeconfig)
	}

	var remoteArgs []string

	if len(namespaces) > 0 {
		remoteArgs = append(remoteArgs, "--namespaces="+strings.Join(namespaces, ","))
	}

	if addons != nil {
		remoteArgs = append(remoteArgs, "--addons="+strings.Join(addons, ","))
	}

	if len(remoteArgs) > 0 {
		args = append(args, "--", "/usr/bin/gather")
		args = append(args, remoteArgs...)
	}

	return exec.Command("oc", args...)
}
