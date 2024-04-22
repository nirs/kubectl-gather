// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
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
	restConfig   *rest.Config
	httpClient   *http.Client
	gatherClient *dynamic.DynamicClient
	directory    string
	opts         *GatherOptions
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

	// We want list all api resources (~80) quickly, gather logs from all pods,
	// and run various commands on the nodes. This change makes gathering 60
	// times faster than the defaults. (9.6 seconds -> 0.15 seconds).
	restConfig.QPS = 50
	restConfig.Burst = 100

	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	gatherClient, err := dynamic.NewForConfigAndClient(restConfig, httpClient)
	if err != nil {
		return nil, err
	}

	return &Gatherer{
		restConfig:   restConfig,
		httpClient:   httpClient,
		gatherClient: gatherClient,
		directory:    directory,
		opts:         &opts,
	}, nil
}

func (g *Gatherer) Gather() error {
	resources, err := g.listAPIResources()
	if err != nil {
		return err
	}

	for i := range resources {
		r := &resources[i]
		if err := g.gatherResources(r); err != nil {
			return err
		}
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

func (g *Gatherer) gatherResources(r *resource) error {
	list, err := g.listResources(r)
	if err != nil {
		return err
	}

	for i := range list.Items {
		item := &list.Items[i]
		err := g.dumpResource(r, item)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Gatherer) listResources(r *resource) (*unstructured.UnstructuredList, error) {
	var gvr = schema.GroupVersionResource{
		Group:    r.GV.Group,
		Version:  r.GV.Version,
		Resource: r.APIResource.Name,
	}

	if r.APIResource.Namespaced {
		list, err := g.gatherClient.Resource(gvr).Namespace(g.opts.Namespace).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return list, nil
	} else {
		list, err := g.gatherClient.Resource(gvr).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return list, nil
	}
}

func (g *Gatherer) dumpResource(r *resource, item *unstructured.Unstructured) error {
	// TODO: dump yaml to output directory.
	if r.APIResource.Namespaced {
		fmt.Printf("namespaces/%s/%s/%s\n", item.GetNamespace(), r.Name(), item.GetName())
	} else {
		fmt.Printf("cluster/%s/%s\n", r.Name(), item.GetName())
	}

	return nil
}
