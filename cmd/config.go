// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type clusterConfig struct {
	Config  *rest.Config
	Context string
}

func loadClusterConfigs(contexts []string, kubeconfig string) ([]*clusterConfig, error) {
	log.Infof("Using kubeconfig %q", kubeconfig)
	config, err := loadKubeconfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	if len(contexts) == 0 {
		if config.CurrentContext == "" {
			return nil, fmt.Errorf("no context specified and current context not set")
		}

		log.Infof("Using current context %q", config.CurrentContext)
		contexts = []string{config.CurrentContext}
	}

	var configs []*clusterConfig

	for _, context := range contexts {
		restConfig, err := clientcmd.NewNonInteractiveClientConfig(
			*config, context, nil, nil).ClientConfig()
		if err != nil {
			return nil, err
		}

		configs = append(configs, &clusterConfig{Config: restConfig, Context: context})
	}

	return configs, nil
}

func loadKubeconfig(kubeconfig string) (*api.Config, error) {
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func defaultKubeconfig() string {
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		return env
	}
	return clientcmd.RecommendedHomeFile
}
