// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
)

func remoteGather(ctx context.Context, clusterConfigs []*clusterConfig) {
	start := time.Now()

	wg := sync.WaitGroup{}
	errors := make(chan error, len(clusterConfigs))

	for i := range clusterConfigs {
		clusterConfig := clusterConfigs[i]
		directory := filepath.Join(directory, clusterConfig.Context)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runMustGather(ctx, clusterConfig.Context, directory); err != nil {
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

func runMustGather(ctx context.Context, clusterContext string, directory string) error {
	log.Infof("Gathering on remote cluster %q", clusterContext)
	start := time.Now()

	logfile, err := createMustGatherLog(directory)
	if err != nil {
		return err
	}

	defer logfile.Close()

	var stderr bytes.Buffer

	cmd := mustGatherCommand(ctx, clusterContext, directory)
	cmd.Stdout = logfile
	cmd.Stderr = &stderr

	log.Debugf("Running command: %s", cmd)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("oc adm must-gather error: %s: %s", err, stderr.String())
	}

	elapsed := time.Since(start).Seconds()
	log.Infof("Gathered on remote cluster %q in %.3f seconds",
		clusterContext, elapsed)

	return nil
}

func createMustGatherLog(directory string) (*os.File, error) {
	if err := os.MkdirAll(directory, 0750); err != nil {
		return nil, err
	}

	return os.Create(filepath.Join(directory, "must-gather.log"))
}

func mustGatherCommand(ctx context.Context, clusterContext string, directory string) *exec.Cmd {
	args := []string{
		"adm",
		"must-gather",
		"--image=" + gather.Image,
		"--context=" + clusterContext,
		"--dest-dir=" + directory,
	}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig="+kubeconfig)
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

	if len(remoteArgs) > 0 {
		args = append(args, "--", "/usr/bin/gather")
		args = append(args, remoteArgs...)
	}

	cmd := exec.CommandContext(ctx, "oc", args...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 60 * time.Second
	return cmd
}
