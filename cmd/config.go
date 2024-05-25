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

func clientConfig(config *api.Config, context string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveClientConfig(*config, context, nil, nil).ClientConfig()
}

func loadConfig(kubeconfig string) (*api.Config, error) {
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

func validateContexts(config *api.Config, contexts []string) error {
	for _, context := range contexts {
		ctx, ok := config.Contexts[context]
		if !ok {
			return fmt.Errorf("context %q does not exist", context)
		}

		if _, ok := config.Clusters[ctx.Cluster]; !ok {
			return fmt.Errorf("context %q does not have a cluster", context)
		}

		if _, ok := config.AuthInfos[ctx.AuthInfo]; !ok {
			return fmt.Errorf("context %q does not have a auth info", context)
		}
	}

	return nil
}
