// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"
	"os"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var directory string
var kubeconfig string
var context string
var namespace string
var verbose bool

var example = `  # Gather data from cluster 'my-cluster' to directory
  # 'gather/my-cluster':
  kubectl gather --context my-cluster

  # Gather data from namespace 'my-namespace' in cluster 'my-cluster'
  # and store in directroy 'gather/my-cluster/namespaces/my-namespace':
  kubectl gather --context foo --namespace my-namespace`

var rootCmd = &cobra.Command{
	Use:     "kubectl-gather",
	Short:   "Gather data from a cluster",
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
	rootCmd.Flags().StringVar(&context, "context", "",
		"the kubeconfig context of the cluster to gather data from")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "",
		"namespace to gather data from")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"be more verbose")
}

func gatherAll(cmd *cobra.Command, args []string) {
	config, err := loadConfig(kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	options := gather.Options{
		Kubeconfig: kubeconfig,
		Context:    context,
		Namespace:  namespace,
		Verbose:    verbose,
	}
	g, err := gather.New(config, directory, options)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Gather(); err != nil {
		log.Fatal(err)
	}
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
