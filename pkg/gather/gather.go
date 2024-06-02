// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"slices"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Based on stats from OpenShift ODF cluster, this value keeps payload size
// under 4 MiB in most cases. Higher values decrease the number of requests and
// increase CPU time and memory usage.
// TODO: Needs more testing to find the optimal value.
const listResourcesLimit = 100

type Options struct {
	Kubeconfig string
	Context    string
	Namespaces []string
	Addons     []string
	Log        *zap.SugaredLogger
}

type Addon interface {
	// Inspect a resource and gather related data.
	Inspect(*unstructured.Unstructured) error
}

type Gatherer struct {
	discoveryClient *discovery.DiscoveryClient
	resourcesClient *dynamic.DynamicClient
	addons          map[string]Addon
	output          OutputDirectory
	opts            *Options
	wq              *WorkQueue
	log             *zap.SugaredLogger
	count           atomic.Int32
}

type resourceInfo struct {
	GroupVersion *schema.GroupVersion
	APIResource  *metav1.APIResource
}

// Name returns the full name of the reosurce, used as the directory name in the
// gather directory. Resources with an empty group are gathered in the cluster
// or namespace direcotry. Reosurces with non-empty group are gathered in a
// group directory.
func (r *resourceInfo) Name() string {
	if r.GroupVersion.Group == "" {
		return r.APIResource.Name
	}
	return r.GroupVersion.Group + "/" + r.APIResource.Name
}

func New(config *rest.Config, directory string, opts Options) (*Gatherer, error) {
	// We want list all api resources (~80) quickly, gather logs from all pods,
	// and run various commands on the nodes. This change makes gathering 60
	// times faster than the defaults. (9.6 seconds -> 0.15 seconds).
	config.QPS = 50
	config.Burst = 100

	// Disable the useless deprecated warnings.
	// TODO: Make this configurable to allow arnings during development.
	config.WarningHandler = rest.NoWarnings{}

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, err
	}

	resourcesClient, err := dynamic.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, err
	}

	output := OutputDirectory{base: directory}

	// TODO: make configurable
	wq := NewWorkQueue(6, 500)

	addons, err := createAddons(config, httpClient, &output, &opts, wq)
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
		log:             opts.Log,
	}, nil
}

func (g *Gatherer) Gather() error {
	g.wq.Start()
	g.wq.Queue(func() error {
		return g.gatherAPIResources()
	})
	return g.wq.Wait()
}

func (g *Gatherer) Count() int32 {
	return g.count.Load()
}

func (g *Gatherer) gatherAPIResources() error {
	var namespaces []string

	if len(g.opts.Namespaces) > 0 {
		var err error

		namespaces, err = g.lookupNamespaces()
		if err != nil {
			// We cannot gather anything.
			return err
		}

		if len(namespaces) == 0 {
			// Nothing to gather - expected conditions when gathering namespace
			// from multiple cluster when namespace exists only on some.
			g.log.Debug("No namespace to gather")
			return nil
		}
	}

	if len(namespaces) == 0 {
		namespaces = []string{metav1.NamespaceAll}
	}

	resources, err := g.listAPIResources()
	if err != nil {
		// We cannot gather anything.
		return fmt.Errorf("cannot list api resources: %s", err)
	}

	for i := range resources {
		r := &resources[i]
		for j := range namespaces {
			namespace := namespaces[j]
			g.wq.Queue(func() error {
				g.gatherResources(r, namespace)
				return nil
			})
		}
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

	g.log.Debugf("Listed %d api resources in %.3f seconds", len(resources), time.Since(start).Seconds())

	return resources, nil
}

// lookupNamespaces looks up the requested namespaces and return a list of
// available namespaces on this cluster.
func (g *Gatherer) lookupNamespaces() ([]string, error) {
	gvr := corev1.SchemeGroupVersion.WithResource("namespaces")
	var found []string

	for _, namespace := range g.opts.Namespaces {
		_, err := g.resourcesClient.Resource(gvr).
			Get(context.TODO(), namespace, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("cannot get namespace %q: %s", namespace, err)
			}

			// Expected condition when gathering multiple clusters.
			g.log.Debugf("Skipping missing namespace %q", namespace)
			continue
		}

		found = append(found, namespace)
	}

	return found, nil
}

func (g *Gatherer) shouldGather(gv schema.GroupVersion, res *metav1.APIResource) bool {
	// We cannot gather resources we cannot list.
	if !slices.Contains(res.Verbs, "list") {
		return false
	}

	if len(g.opts.Namespaces) != 0 {
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

func (g *Gatherer) gatherResources(r *resourceInfo, namespace string) {
	start := time.Now()

	opts := metav1.ListOptions{Limit: listResourcesLimit}
	count := 0

	for {
		list, err := g.listResources(r, namespace, opts)
		if err != nil {
			// Fall back to full list only if this was an attempt to get the next
			// page and the resource expired.
			if opts.Continue == "" || !errors.IsResourceExpired(err) {
				g.log.Warnf("Cannot list %q: %s", r.Name(), err)
				break
			}

			g.log.Debugf("Falling back to full list for %q: %s", r.Name(), err)

			opts.Limit = 0
			opts.Continue = ""

			list, err = g.listResources(r, namespace, opts)
			if err != nil {
				g.log.Warnf("Cannot list %q: %s", r.Name(), err)
				break
			}

			// If we got a full list, don't count twice what we aleady gathered.
			count = 0
		}

		count += len(list.Items)

		addon := g.addons[r.Name()]

		for i := range list.Items {
			item := &list.Items[i]

			err := g.dumpResource(r, item)
			if err != nil {
				g.log.Warnf("Cannot dump \"%s/%s\": %s", r.Name(), item.GetName(), err)
			}

			if addon != nil {
				if err := addon.Inspect(item); err != nil {
					g.log.Warnf("Cannot inspect \"%s/%s\": %s", r.Name(), item.GetName(), err)
				}
			}
		}

		opts.Continue = list.GetContinue()
		if opts.Continue == "" {
			break
		}
	}

	g.count.Add(int32(count))
	g.log.Debugf("Gathered %d %q in %.3f seconds", count, r.Name(), time.Since(start).Seconds())
}

func (g *Gatherer) listResources(r *resourceInfo, namespace string, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	start := time.Now()

	var gvr = schema.GroupVersionResource{
		Group:    r.GroupVersion.Group,
		Version:  r.GroupVersion.Version,
		Resource: r.APIResource.Name,
	}

	ctx := context.TODO()
	var list *unstructured.UnstructuredList
	var err error

	if r.APIResource.Namespaced {
		list, err = g.resourcesClient.Resource(gvr).Namespace(namespace).List(ctx, opts)
	} else {
		list, err = g.resourcesClient.Resource(gvr).List(ctx, opts)
	}

	if err != nil {
		return nil, err
	}

	g.log.Debugf("Listed %d %q in %.3f seconds", len(list.Items), r.Name(), time.Since(start).Seconds())

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
