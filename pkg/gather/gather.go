// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Options struct {
	Context   string
	Namespace string
	Verbose   bool
}

type Gatherer struct {
	restConfig      *rest.Config
	httpClient      *http.Client
	resourcesClient *dynamic.DynamicClient
	logsClient      *rest.RESTClient
	output          OutputDirectory
	opts            *Options
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

type containerInfo struct {
	Namespace string
	Pod       string
	Name      string
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

	resourcesClient, err := dynamic.NewForConfigAndClient(restConfig, httpClient)
	if err != nil {
		return nil, err
	}

	logsClient, err := createLogsClient(restConfig, httpClient)
	if err != nil {
		return nil, err
	}

	return &Gatherer{
		restConfig:      restConfig,
		httpClient:      httpClient,
		resourcesClient: resourcesClient,
		logsClient:      logsClient,
		output:          OutputDirectory{base: filepath.Join(directory, opts.Context)},
		opts:            &opts,
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

func (g *Gatherer) listAPIResources() ([]resourceInfo, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(g.restConfig, g.httpClient)
	if err != nil {
		return nil, err
	}

	items, err := discoveryClient.ServerPreferredResources()
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

			// We cannot gather resources we cannot list.
			if !slices.Contains(res.Verbs, "list") {
				continue
			}

			// If we gather specific namespace, we must use only namespaced resources.
			if g.opts.Namespace != "" && !res.Namespaced {
				continue
			}

			resources = append(resources, resourceInfo{GroupVersion: &gv, APIResource: res})
		}
	}

	return resources, nil
}

func (g *Gatherer) gatherResources(r *resourceInfo) error {
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

		if r.Name() == "pods" {
			err := g.gatherPod(item)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Gatherer) listResources(r *resourceInfo) (*unstructured.UnstructuredList, error) {
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

func createLogsClient(config *rest.Config, httpClient *http.Client) (*rest.RESTClient, error) {
	logsConfig := rest.CopyConfig(config)

	logsConfig.APIPath = "api"
	logsConfig.GroupVersion = &corev1.SchemeGroupVersion
	logsConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	return rest.RESTClientForConfigAndClient(logsConfig, httpClient)
}

func (g *Gatherer) gatherPod(pod *unstructured.Unstructured) error {
	containers, err := listContainers(pod)
	if err != nil {
		return err
	}

	for _, container := range containers {
		if err := g.gatherContainerLog(container, false); err != nil {
			return err
		}

		// This always fails with "previous terminated container not found",
		// same as kubectl logs --previous. Ignoring errors since there is no
		// way to detect if previous log exists or detect the specific error.
		g.gatherContainerLog(container, true)
	}

	return nil
}

func listContainers(pod *unstructured.Unstructured) ([]containerInfo, error) {
	containers, found, err := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil
	}

	var result []containerInfo

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			return nil, nil
		}

		name, found, err := unstructured.NestedString(container, "name")
		if err != nil {
			return nil, err
		}

		if !found {
			continue
		}

		result = append(result, containerInfo{
			Namespace: pod.GetNamespace(),
			Pod:       pod.GetName(),
			Name:      name,
		})
	}

	return result, nil
}

func (g *Gatherer) gatherContainerLog(container containerInfo, previous bool) error {
	var which string
	if previous {
		which = "previous"
	} else {
		which = "current"
	}

	req := g.logsClient.Get().
		Namespace(container.Namespace).
		Resource("pods").
		Name(container.Pod).
		SubResource("log").
		Param("container", container.Name)
	if previous {
		req.Param("previous", "true")
	}

	src, err := req.Stream(context.TODO())
	if err != nil {
		return err
	}

	defer src.Close()

	dst, err := g.output.CreateContainerLog(container.Namespace, container.Pod, container.Name, which)
	if err != nil {
		return err
	}

	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
