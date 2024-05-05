// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"sync"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
	"github.com/spf13/cobra"
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
	log := gather.NewLogger(verbose)

	if namespace != "" {
		log.Printf("Gathering namespace %s", namespace)
	} else {
		log.Print("Gathering all namespaces")
	}

	log.Printf("Using kubeconfig %s", kubeconfig)
	config, err := loadConfig(kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(contexts) == 0 {
		if config.CurrentContext == "" {
			log.Fatal("No context specified and current context not set")
		}

		log.Printf("Using current context %q", config.CurrentContext)
		contexts = append(contexts, config.CurrentContext)
	}

	wg := sync.WaitGroup{}
	errors := make(chan error, len(contexts))

	for _, context := range contexts {
		options := gather.Options{
			Kubeconfig: kubeconfig,
			Context:    context,
			Namespace:  namespace,
			Verbose:    verbose,
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
			log.Printf("Gathered cluster %s in %.3f seconds", options.Context, time.Since(start).Seconds())
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		log.Fatal(err)
	}

	log.Printf("Gathered %d clusters in %.3f seconds", len(contexts), time.Since(start).Seconds())
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
