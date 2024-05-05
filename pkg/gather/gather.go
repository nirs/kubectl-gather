// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"bufio"
	"context"
	"io"
	"log"
	"path/filepath"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Options struct {
	Kubeconfig string
	Context    string
	Namespace  string
	Verbose    bool
}

type Addon interface {
	Gather(*unstructured.Unstructured) error
}

type Gatherer struct {
	discoveryClient *discovery.DiscoveryClient
	resourcesClient *dynamic.DynamicClient
	addons          map[string]Addon
	output          OutputDirectory
	opts            *Options
	wq              *WorkQueue
	log             *log.Logger
}

type resourceInfo struct {
	GroupVersion *schema.GroupVersion
	APIResource  *metav1.APIResource
}

// Name returns the full name of the reosurce, used as the directory name in the
// gather directory.
func (r *resourceInfo) Name() string {
	if r.GroupVersion.Group == "" {
		return r.APIResource.Name
	}
	return r.APIResource.Name + "." + r.GroupVersion.Group
}

func New(config *api.Config, directory string, opts Options) (*Gatherer, error) {
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

	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(restConfig, httpClient)
	if err != nil {
		return nil, err
	}

	resourcesClient, err := dynamic.NewForConfigAndClient(restConfig, httpClient)
	if err != nil {
		return nil, err
	}

	output := OutputDirectory{base: filepath.Join(directory, opts.Context)}

	// TODO: make configurable
	wq := NewWorkQueue(6, 500)

	addons, err := createAddons(restConfig, httpClient, &output, &opts, wq)
	if err != nil {
		return nil, err
	}

	return &Gatherer{
		discoveryClient: discoveryClient,
		resourcesClient: resourcesClient,
		addons:          addons,
		output:          output,
		opts:            &opts,
		wq:              wq,
		log:             NewLogger("main", &opts),
	}, nil
}

func (g *Gatherer) Gather() error {
	g.wq.Start()
	g.wq.Queue(func() error {
		return g.gatherAPIResources()
	})
	return g.wq.Wait()
}

func (g *Gatherer) gatherAPIResources() error {
	resources, err := g.listAPIResources()
	if err != nil {
		return err
	}

	for i := range resources {
		r := &resources[i]
		g.wq.Queue(func() error {
			return g.gatherResources(r)
		})
	}

	return nil
}

func (g *Gatherer) listAPIResources() ([]resourceInfo, error) {
	start := time.Now()

	items, err := g.discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	resources := []resourceInfo{}

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
			if !g.shouldGather(gv, res) {
				continue
			}

			resources = append(resources, resourceInfo{GroupVersion: &gv, APIResource: res})
		}
	}

	g.log.Printf("Listed %d api resources in %.3f seconds", len(resources), time.Since(start).Seconds())

	return resources, nil
}

func (g *Gatherer) shouldGather(gv schema.GroupVersion, res *metav1.APIResource) bool {
	// We cannot gather resources we cannot list.
	if !slices.Contains(res.Verbs, "list") {
		return false
	}

	if g.opts.Namespace != "" {
		// If we gather specific namespace, we must use only namespaced resources.
		if !res.Namespaced {
			return false
		}

		// olm bug? - returned for *every namespace* when listing by namespace.
		// https://github.com/operator-framework/operator-lifecycle-manager/issues/2932
		if res.Name == "packagemanifests" && gv.Group == "packages.operators.coreos.com" {
			return false
		}
	}

	// Skip "events", replaced by "events.events.k8s.io".  Otherwise we
	// get all events twice, as "events" and as "events.events.k8s.io",
	// both resources contain the same content.
	if res.Name == "events" && gv.Group == "" {
		return false
	}

	// Avoid warning: "v1 ComponentStatus is deprecated in v1.19+"
	if res.Name == "componentstatuses" && gv.Group == "" {
		return false
	}

	return true
}

func (g *Gatherer) gatherResources(r *resourceInfo) error {
	start := time.Now()

	list, err := g.listResources(r)
	if err != nil {
		return err
	}

	addon := g.addons[r.Name()]

	for i := range list.Items {
		item := &list.Items[i]

		err := g.dumpResource(r, item)
		if err != nil {
			return err
		}

		if addon != nil {
			if err := addon.Gather(item); err != nil {
				return err
			}
		}
	}

	g.log.Printf("Gathered %d %s in %.3f seconds", len(list.Items), r.Name(), time.Since(start).Seconds())

	return nil
}

func (g *Gatherer) listResources(r *resourceInfo) (*unstructured.UnstructuredList, error) {
	start := time.Now()

	var gvr = schema.GroupVersionResource{
		Group:    r.GroupVersion.Group,
		Version:  r.GroupVersion.Version,
		Resource: r.APIResource.Name,
	}

	ctx := context.TODO()
	var opts metav1.ListOptions
	var list *unstructured.UnstructuredList
	var err error

	if r.APIResource.Namespaced {
		list, err = g.resourcesClient.Resource(gvr).Namespace(g.opts.Namespace).List(ctx, opts)
	} else {
		list, err = g.resourcesClient.Resource(gvr).List(ctx, opts)
	}

	if err != nil {
		return nil, err
	}

	g.log.Printf("Listed %d %s in %.3f seconds", len(list.Items), r.Name(), time.Since(start).Seconds())

	return list, nil
}

func (g *Gatherer) dumpResource(r *resourceInfo, item *unstructured.Unstructured) error {
	dst, err := g.createResource(r, item)
	if err != nil {
		return err
	}

	defer dst.Close()
	writer := bufio.NewWriter(dst)
	printer := printers.YAMLPrinter{}
	if err := printer.PrintObj(item, writer); err != nil {
		return err
	}

	return writer.Flush()
}

func (g *Gatherer) createResource(r *resourceInfo, item *unstructured.Unstructured) (io.WriteCloser, error) {
	if r.APIResource.Namespaced {
		return g.output.CreateNamespacedResource(item.GetNamespace(), r.Name(), item.GetName())
	} else {
		return g.output.CreateClusterResource(r.Name(), item.GetName())
	}
}
