// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	stdlog "log"
	"os"
	"sync"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var directory string
var kubeconfig string
var contexts []string
var namespace string
var verbose bool

var example = `  # Gather data from clusters "dr1", "dr2" and "hub" and store it
  # in directory "gather/".
  kubectl gather --directory gather --contexts dr1,dr2,hub

  # Gather data from namespace "rook-ceph" in cluster "dr1"
  kubectl gather --directory gather --contexts dr1 --namespace rook-ceph`

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
	rootCmd.Flags().StringVarP(&directory, "directory", "d", defaultGatherDirectory(),
		"directory for storing gathered data")
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig(),
		"the kubeconfig file to use")
	rootCmd.Flags().StringSliceVar(&contexts, "contexts", nil,
		"command separate list of contexts to gather data from")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "",
		"namespace to gather data from")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"be more verbose")
}

func gatherAll(cmd *cobra.Command, args []string) {
	start := time.Now()
	log := createLogger()
	defer log.Sync()

	if namespace != "" {
		log.Infof("Gathering namespace %q", namespace)
	} else {
		log.Infof("Gathering all namespaces")
	}

	log.Infof("Using kubeconfig %q", kubeconfig)
	config, err := loadConfig(kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(contexts) == 0 {
		if config.CurrentContext == "" {
			log.Fatal("No context specified and current context not set")
		}

		log.Infof("Using current context %q", config.CurrentContext)
		contexts = append(contexts, config.CurrentContext)
	}

	wg := sync.WaitGroup{}
	errors := make(chan error, len(contexts))

	for _, context := range contexts {
		log.Infof("Gathering cluster %q", context)
		options := gather.Options{
			Kubeconfig: kubeconfig,
			Context:    context,
			Namespace:  namespace,
			Log:        log.Named(context),
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			g, err := gather.New(config, directory, options)
			if err != nil {
				errors <- err
				return
			}

			if err := g.Gather(); err != nil {
				errors <- err
			}
			log.Infof("Gathered cluster %q in %.3f seconds", options.Context, time.Since(start).Seconds())
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		log.Fatal(err)
	}

	log.Infof("Gathered %d clusters in %.3f seconds", len(contexts), time.Since(start).Seconds())
}

func createLogger() *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()

	// Disable file:line annotation, not helpful in a tiny application.
	config.DisableCaller = true

	if !verbose {
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := config.Build()
	if err != nil {
		stdlog.Fatalf("Cannot create logger: %s", err)
	}

	return logger.Named("gather").Sugar()
}

func defaultKubeconfig() string {
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		return env
	}
	return clientcmd.RecommendedHomeFile
}

func loadConfig(kubeconfig string) (*api.Config, error) {
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func defaultGatherDirectory() string {
	return time.Now().Format("gather-20060102150405")
}
