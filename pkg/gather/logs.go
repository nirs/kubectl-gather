// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type LogsAddon struct {
	client *rest.RESTClient
	output *OutputDirectory
}

type containerInfo struct {
	Namespace string
	Pod       string
	Name      string
}

func NewLogsAddon(config *rest.Config, httpClient *http.Client, output *OutputDirectory) (*LogsAddon, error) {
	logsConfig := rest.CopyConfig(config)

	logsConfig.APIPath = "api"
	logsConfig.GroupVersion = &corev1.SchemeGroupVersion
	logsConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	client, err := rest.RESTClientForConfigAndClient(logsConfig, httpClient)
	if err != nil {
		return nil, err
	}

	return &LogsAddon{client: client, output: output}, nil
}

func (g *LogsAddon) Gather(pod *unstructured.Unstructured) error {
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

func (g *LogsAddon) gatherContainerLog(container containerInfo, previous bool) error {
	var which string
	if previous {
		which = "previous"
	} else {
		which = "current"
	}

	req := g.client.Get().
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
