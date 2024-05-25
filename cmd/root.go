// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var directory string
var kubeconfig string
var contexts []string
var namespaces []string
var verbose bool
var log *zap.SugaredLogger

var example = `  # Gather data from all namespaces in current context in my-kubeconfig and
  # store it in gather-{timestamp}.
  kubectl gather --kubeconfig my-kubeconfig

  # Gather data from all namespaces in clusters "dr1", "dr2" and "hub" and store
  # it in "gather/", using default kubeconfig (~/.kube/config).
  kubectl gather --directory gather --contexts dr1,dr2,hub

  # Gather data from namespaces "my-ns" and "other-ns" in cluster "dr1" and
  # store it in gather/, using default kubeconfig (~/.kube/config).
  kubectl gather --directory gather --contexts dr1 --namespaces my-ns,other-ns`

var rootCmd = &cobra.Command{
	Use:     "kubectl-gather",
	Short:   "Gather data from clusters",
	Example: example,
	Annotations: map[string]string{
		cobra.CommandDisplayNameAnnotation: "kubectl gather",
	},
	Run: gatherAll,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&directory, "directory", "d", "",
		"directory for storing gathered data (default \"gather.{timestamp}\")")

	// Don't set default kubeconfig, so kubeconfig is empty unless the user
	// specified the option. This is required to allow running remote commands
	// using in-cluster config.
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "",
		"the kubeconfig file to use")

	rootCmd.Flags().StringSliceVar(&contexts, "contexts", nil,
		"comma separated list of contexts to gather data from")
	rootCmd.Flags().StringSliceVarP(&namespaces, "namespaces", "n", nil,
		"comma separated list of namespaces to gather data from")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"be more verbose")
}

type result struct {
	Count int32
	Err   error
}

func gatherAll(cmd *cobra.Command, args []string) {
	start := time.Now()

	if directory == "" {
		directory = defaultGatherDirectory()
	}

	log = createLogger(directory, verbose)
	defer log.Sync()

	clusters, err := loadClusterConfigs(contexts, kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(namespaces) != 0 {
		log.Infof("Gathering from namespaces %v", namespaces)
	} else {
		log.Infof("Gathering from all namespaces")
	}

	if !cmd.Flags().Changed("directory") {
		log.Infof("Storing data in %q", directory)
	}

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

	count := int32(0)

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

func createLogger(directory string, verbose bool) *zap.SugaredLogger {
	if err := os.MkdirAll(directory, 0750); err != nil {
		stdlog.Fatalf("Cannot create directory: %s", err)
	}

	logfile, err := os.Create(filepath.Join(directory, "gather.log"))
	if err != nil {
		stdlog.Fatalf("Cannot create log file: %s", err)
	}

	config := zap.NewDevelopmentEncoderConfig()
	config.CallerKey = ""
	encoder := zapcore.NewConsoleEncoder(config)

	consoleLevel := zapcore.InfoLevel
	if verbose {
		consoleLevel = zapcore.DebugLevel
	}

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.Lock(logfile), zapcore.DebugLevel),
		zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), consoleLevel),
	)

	return zap.New(core).Named("gather").Sugar()
}

func defaultGatherDirectory() string {
	return time.Now().Format("gather.20060102150405")
}
