// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
	"github.com/nirs/kubectl-gather/pkg/kubeconfig"
)

type result struct {
	Count int
	Err   error
}

func localGather(clusterConfigs []*kubeconfig.Config) {
	start := time.Now()

	wg := sync.WaitGroup{}
	results := make(chan result, len(clusterConfigs))

	for i := range clusterConfigs {
		clusterConfig := clusterConfigs[i]

		if clusterConfig.Name != "" {
			log.Infof("Gathering from cluster %q", clusterConfig.Name)
		} else {
			log.Info("Gathering in cluster")
		}
		start := time.Now()

		clusterDir := filepath.Join(directory, clusterConfig.Name)

		options := gather.Options{
			Kubeconfig: clusterConfig.Kubeconfig,
			Context:    clusterConfig.Context,
			Namespaces: namespaces,
			Addons:     addons,
			Cluster:    cluster,
			Salt:       parsedSalt,
			Log:        log.Named(clusterConfig.Name),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			g, err := gather.New(clusterConfig.Config, clusterDir, options)
			if err != nil {
				results <- result{Err: err}
				return
			}

			err = g.Gather()
			results <- result{Count: g.Count(), Err: err}
			if err != nil {
				return
			}

			elapsed := time.Since(start).Seconds()
			if clusterConfig.Name != "" {
				log.Infof("Gathered %d resources from cluster %q in %.3f seconds",
					g.Count(), clusterConfig.Name, elapsed)
			} else {
				log.Infof("Gathered %d resources in %.3f seconds",
					g.Count(), elapsed)
			}
		}()
	}

	wg.Wait()
	close(results)

	count := 0

	for r := range results {
		if r.Err != nil {
			log.Fatal(r.Err)
		}
		count += r.Count
	}

	if len(namespaces) != 0 && count == 0 {
		// Likely a user error like a wrong namespace.
		log.Warnf("No resource gathered from namespaces %v", namespaces)
	}

	log.Infof("Gathered %d resources from %d clusters in %.3f seconds",
		count, len(clusterConfigs), time.Since(start).Seconds())
}
