// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"net/http"
	"slices"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type GatherOptions struct {
	Context   string
	Namespace string
	Verbose   bool
}

type Gatherer struct {
	restConfig *rest.Config
	httpClient *http.Client
	directory  string
	opts       *GatherOptions
}

type resource struct {
	GV          *schema.GroupVersion
	APIResource *v1.APIResource
}

// Name returns the full name of the reosurce, used as the directory name in the
// gather directory.
func (r *resource) Name() string {
	if r.GV.Group == "" {
		return r.APIResource.Name
	}
	return r.APIResource.Name + "." + r.GV.Group
}

func NewGatherer(config *api.Config, directory string, opts GatherOptions) (*Gatherer, error) {
	if opts.Context == "" {
		opts.Context = config.CurrentContext
	}

	restConfig, err := clientcmd.NewNonInteractiveClientConfig(*config, opts.Context, nil, nil).ClientConfig()
	if err != nil {
		return nil, err
	}

	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	return &Gatherer{
		restConfig: restConfig,
		httpClient: httpClient,
		directory:  directory,
		opts:       &opts,
	}, nil
}

func (g *Gatherer) Gather() error {
	resources, err := g.listAPIResources()
	if err != nil {
		return err
	}

	// TODO: Gather the resources instead of printing.
	for _, res := range resources {
		fmt.Printf("%s\n", res.Name())
	}

	return nil
}

func (g *Gatherer) listAPIResources() ([]resource, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(g.restConfig, g.httpClient)
	if err != nil {
		return nil, err
	}

	items, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	resources := []resource{}

	for _, list := range items {
		// Some resources have empty Group, and the resource list have only
		// GroupVersion. Seems that the only way to get the Group is to parse
		// it (what kubectl api-resources does).
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for i := range list.APIResources {
			res := &list.APIResources[i]

			// We cannot gather resources we cannot list.
			if !slices.Contains(res.Verbs, "list") {
				continue
			}

			// If we gather specific namespace, we must use only namespaced resources.
			if g.opts.Namespace != "" && !res.Namespaced {
				continue
			}

			resources = append(resources, resource{GV: &gv, APIResource: res})
		}
	}

	return resources, nil
}
