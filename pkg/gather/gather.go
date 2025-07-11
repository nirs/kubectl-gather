// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Based on stats from OCP cluster, this value keeps payload size under 4 MiB in
// most cases. Higher values decrease the number of requests and increase CPU
// time and memory usage. TODO: Needs more testing to find the optimal value.
const listResourcesLimit = 100

// Number of workers serving a work queue.
const workQueueSize = 6

// Replaced during build with actual values.
var Version = "latest"
var Image = "quay.io/nirsof/gather:latest"

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
	config       *rest.Config
	httpClient   *http.Client
	client       *dynamic.DynamicClient
	addons       map[string]Addon
	output       OutputDirectory
	opts         *Options
	gatherQueue  *WorkQueue
	inspectQueue *WorkQueue
	log          *zap.SugaredLogger
	mutex        sync.Mutex
	resources    map[string]struct{}
}

type resourceInfo struct {
	schema.GroupVersionResource
	Namespaced bool
}

// Name returns the full name of the resource, used as the directory name in the
// gather directory. Resources with an empty group are gathered in the cluster
// or namespace directory. Resources with non-empty group are gathered in a
// group directory.
func (r *resourceInfo) Name() string {
	if r.Group == "" {
		return r.Resource
	}
	return r.Group + "/" + r.Resource
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

	client, err := dynamic.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, err
	}

	g := &Gatherer{
		config:       config,
		httpClient:   httpClient,
		client:       client,
		output:       OutputDirectory{base: directory},
		opts:         &opts,
		gatherQueue:  NewWorkQueue(workQueueSize),
		inspectQueue: NewWorkQueue(workQueueSize),
		log:          opts.Log,
		resources:    make(map[string]struct{}),
	}

	backend := &gatherBackend{
		g:  g,
		wq: g.inspectQueue,
	}

	addons, err := createAddons(backend)
	if err != nil {
		return nil, err
	}

	g.addons = addons
	return g, nil
}

func (g *Gatherer) Gather() error {
	start := time.Now()
	g.gatherQueue.Start()
	g.inspectQueue.Start()

	defer func() {
		// Safe close even if some work was queued in prepare before it failed.
		g.gatherQueue.Close()
		_ = g.gatherQueue.Wait()
		g.inspectQueue.Close()
		_ = g.inspectQueue.Wait()
	}()

	// Start the prepare step, looking up namespaces and API resources and
	// queuing work on the gather workqueue.
	if err := g.prepare(); err != nil {
		return err
	}
	g.log.Debugf("Prepare step finished in %.2f seconds", time.Since(start).Seconds())

	// No more work can be queued on the gather queue so we can close it.
	g.gatherQueue.Close()
	gatherErr := g.gatherQueue.Wait()
	g.log.Debugf("Gather step finished in %.2f seconds", time.Since(start).Seconds())

	// No more work can be queued on the inspect queue so we can close it.
	g.inspectQueue.Close()
	inspectErr := g.inspectQueue.Wait()
	g.log.Debugf("Inspect step finished in %.2f seconds", time.Since(start).Seconds())

	// All work completed. Report fatal errors to caller.
	if gatherErr != nil || inspectErr != nil {
		return fmt.Errorf("failed to gather (gather: %w, inspect: %w)", gatherErr, inspectErr)
	}

	return nil
}

func (g *Gatherer) Count() int {
	return len(g.resources)
}

func (g *Gatherer) prepare() error {
	var namespaces []string

	if len(g.opts.Namespaces) > 0 {
		var err error

		namespaces, err = g.gatherNamespaces()
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
			g.gatherQueue.Queue(func() error {
				g.gatherResources(r, namespace)
				return nil
			})
		}
	}

	return nil
}

func (g *Gatherer) listAPIResources() ([]resourceInfo, error) {
	start := time.Now()

	client, err := discovery.NewDiscoveryClientForConfigAndClient(g.config, g.httpClient)
	if err != nil {
		return nil, err
	}

	items, err := client.ServerPreferredResources()
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

			resources = append(resources, resourceInfo{
				GroupVersionResource: gv.WithResource(res.Name),
				Namespaced:           res.Namespaced,
			})
		}
	}

	g.log.Debugf("Listed %d api resources in %.3f seconds", len(resources), time.Since(start).Seconds())

	return resources, nil
}

// gatherNamespaces gathers the requested namespaces and return a list of
// available namespaces on this cluster.
func (g *Gatherer) gatherNamespaces() ([]string, error) {
	gvr := corev1.SchemeGroupVersion.WithResource("namespaces")
	var found []string

	for _, namespace := range g.opts.Namespaces {
		ns, err := g.client.Resource(gvr).
			Get(context.TODO(), namespace, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("cannot get namespace %q: %s", namespace, err)
			}

			// Expected condition when gathering multiple clusters.
			g.log.Debugf("Skipping missing namespace %q", namespace)
			continue
		}

		r := resourceInfo{GroupVersionResource: gvr}
		key := g.keyFromResource(&r, ns)
		if g.addResource(key) {
			if err := g.dumpResource(&r, ns); err != nil {
				g.log.Warnf("Cannot dump %q: %s", key, err)
			}
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
		}

		addon := g.addons[r.Name()]

		for i := range list.Items {
			item := &list.Items[i]
			key := g.keyFromResource(r, item)

			if !g.addResource(key) {
				continue
			}

			count += 1

			if err := g.dumpResource(r, item); err != nil {
				g.log.Warnf("Cannot dump %q: %s", key, err)
			}

			if addon != nil {
				if err := addon.Inspect(item); err != nil {
					g.log.Warnf("Cannot inspect %q: %s", key, err)
				}
			}
		}

		opts.Continue = list.GetContinue()
		if opts.Continue == "" {
			break
		}
	}

	g.log.Debugf("Gathered %d %q in %.3f seconds", count, r.Name(), time.Since(start).Seconds())
}

func (g *Gatherer) listResources(r *resourceInfo, namespace string, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	start := time.Now()

	ctx := context.TODO()
	var list *unstructured.UnstructuredList
	var err error

	if r.Namespaced {
		list, err = g.client.Resource(r.GroupVersionResource).
			Namespace(namespace).
			List(ctx, opts)
	} else {
		list, err = g.client.Resource(r.GroupVersionResource).
			List(ctx, opts)
	}

	if err != nil {
		return nil, err
	}

	g.log.Debugf("Listed %d %q in %.3f seconds", len(list.Items), r.Name(), time.Since(start).Seconds())

	return list, nil
}

func (g *Gatherer) gatherResource(gvr schema.GroupVersionResource, name types.NamespacedName) {
	start := time.Now()

	r := resourceInfo{GroupVersionResource: gvr, Namespaced: name.Namespace != ""}

	key := g.keyFromName(&r, name)
	if !g.addResource(key) {
		return
	}

	item, err := g.getResource(&r, name)
	if err != nil {
		g.log.Warnf("Cannot get %q: %s", key, err)
		return
	}

	if err := g.dumpResource(&r, item); err != nil {
		g.log.Warnf("Cannot dump %q: %s", key, err)
		return
	}

	g.log.Debugf("Gathered %q in %.3f seconds", key, time.Since(start).Seconds())
}

func (g *Gatherer) getResource(r *resourceInfo, name types.NamespacedName) (*unstructured.Unstructured, error) {
	ctx := context.TODO()
	var opts metav1.GetOptions

	if r.Namespaced {
		return g.client.Resource(r.GroupVersionResource).
			Namespace(name.Namespace).
			Get(ctx, name.Name, opts)
	} else {
		return g.client.Resource(r.GroupVersionResource).
			Get(ctx, name.Name, opts)
	}
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
	if r.Namespaced {
		return g.output.CreateNamespacedResource(item.GetNamespace(), r.Name(), item.GetName())
	} else {
		return g.output.CreateClusterResource(r.Name(), item.GetName())
	}
}

func (g *Gatherer) keyFromResource(r *resourceInfo, item *unstructured.Unstructured) string {
	name := types.NamespacedName{Namespace: item.GetNamespace(), Name: item.GetName()}
	return g.keyFromName(r, name)
}

func (g *Gatherer) keyFromName(r *resourceInfo, name types.NamespacedName) string {
	if r.Namespaced {
		return fmt.Sprintf("namespaces/%s/%s/%s", name.Namespace, r.Name(), name.Name)
	} else {
		return fmt.Sprintf("cluster/%s/%s", r.Name(), name.Name)
	}
}

func (g *Gatherer) addResource(key string) bool {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if _, ok := g.resources[key]; ok {
		return false
	}

	g.resources[key] = struct{}{}
	return true
}
