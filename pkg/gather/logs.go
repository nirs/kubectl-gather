// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type logType string

const (
	current  = logType("current")
	previous = logType("previous")
)

type LogsAddon struct {
	client *rest.RESTClient
	output *OutputDirectory
	opts   *Options
	q      Queuer
	log    *log.Logger
}

type containerInfo struct {
	Namespace string
	Pod       string
	Name      string
}

func NewLogsAddon(config *rest.Config, httpClient *http.Client, out *OutputDirectory, opts *Options, q Queuer) (*LogsAddon, error) {
	logsConfig := rest.CopyConfig(config)

	logsConfig.APIPath = "api"
	logsConfig.GroupVersion = &corev1.SchemeGroupVersion
	logsConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	client, err := rest.RESTClientForConfigAndClient(logsConfig, httpClient)
	if err != nil {
		return nil, err
	}

	return &LogsAddon{
		client: client,
		output: out,
		opts:   opts,
		q:      q,
		log:    createLogger("logs", opts),
	}, nil
}

func (g *LogsAddon) Gather(pod *unstructured.Unstructured) error {
	containers, err := listContainers(pod)
	if err != nil {
		return err
	}

	for _, container := range containers {
		g.q.Queue(func() error {
			return g.gatherContainerLog(container, current)
		})

		g.q.Queue(func() error {
			// This typically fails with "previous terminated container not
			// found", same as kubectl logs --previous. Ignoring errors since
			// there is no way to detect if previous log exists or detect the
			// specific error.
			g.gatherContainerLog(container, previous)
			return nil
		})
	}

	return nil
}

func (g *LogsAddon) gatherContainerLog(container containerInfo, which logType) error {
	start := time.Now()

	req := g.client.Get().
		Namespace(container.Namespace).
		Resource("pods").
		Name(container.Pod).
		SubResource("log").
		Param("container", container.Name)
	if which == previous {
		req.Param("previous", "true")
	}

	src, err := req.Stream(context.TODO())
	if err != nil {
		return err
	}

	defer src.Close()

	dst, err := g.output.CreateContainerLog(container.Namespace, container.Pod, container.Name, string(which))
	if err != nil {
		return err
	}

	defer dst.Close()

	n, err := io.Copy(dst, src)

	elapsed := time.Since(start).Seconds()
	rate := float64(n) / float64(1024*1024) / elapsed
	g.log.Printf("Gathered %s logs %s/%s/%s in %.3f seconds (%.2f MiB/s)",
		which, container.Namespace, container.Pod, container.Name, elapsed, rate)

	return err
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
