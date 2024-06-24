// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
)

type result struct {
	Count int
	Err   error
}

func localGather(clusters []*clusterConfig) {
	start := time.Now()

	wg := sync.WaitGroup{}
	results := make(chan result, len(clusters))

	for i := range clusters {
		cluster := clusters[i]

		if cluster.Context != "" {
			log.Infof("Gathering from cluster %q", cluster.Context)
		} else {
			log.Info("Gathering on cluster")
		}
		start := time.Now()

		directory := filepath.Join(directory, cluster.Context)

		options := gather.Options{
			Kubeconfig: kubeconfig,
			Context:    cluster.Context,
			Namespaces: namespaces,
			Addons:     addons,
			Log:        log.Named(cluster.Context),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			g, err := gather.New(cluster.Config, directory, options)
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
			if cluster.Context != "" {
				log.Infof("Gathered %d resources from cluster %q in %.3f seconds",
					g.Count(), cluster.Context, elapsed)
			} else {
				log.Infof("Gathered %d resources on cluster in %.3f seconds",
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
		count, len(clusters), time.Since(start).Seconds())
}
