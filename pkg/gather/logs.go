// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
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
	log    *zap.SugaredLogger
}

type containerInfo struct {
	Namespace      string
	Pod            string
	Name           string
	HasPreviousLog bool
}

func (c containerInfo) String() string {
	return c.Namespace + "/" + c.Pod + "/" + c.Name
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
		log:    opts.Log.Named("logs"),
	}, nil
}

func (g *LogsAddon) Gather(pod *unstructured.Unstructured) error {
	containers, err := listContainers(pod)
	if err != nil {
		return fmt.Errorf("cannnot find containers in pod %s/%s: %s",
			pod.GetNamespace(), pod.GetName(), err)
	}

	for i := range containers {
		container := containers[i]

		g.q.Queue(func() error {
			g.gatherContainerLog(container, current)
			return nil
		})

		if container.HasPreviousLog {
			g.q.Queue(func() error {
				g.gatherContainerLog(container, previous)
				return nil
			})
		}
	}

	return nil
}

func (g *LogsAddon) gatherContainerLog(container *containerInfo, which logType) {
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
		// Getting the log is possible only if a container is running, but
		// checking the container state before the call is racy. We get a
		// BadRequest error like: "container ... in pod ... is waiting to start:
		// PodInitializing" so there is no way to detect the actul problem.
		// Since this is expected situation, and getting logs is best effort, we
		// log this in debug level.
		g.log.Debugf("Cannot get log for %s/%s: %+v", container, which, err)
		return
	}

	defer src.Close()

	dst, err := g.output.CreateContainerLog(
		container.Namespace, container.Pod, container.Name, string(which))
	if err != nil {
		g.log.Warnf("Cannot create %s/%s.log: %s", container, which, err)
		return
	}

	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		g.log.Warnf("Cannot copy %s/%s.log: %s", container, which, err)
	}

	elapsed := time.Since(start).Seconds()
	rate := float64(n) / float64(1024*1024) / elapsed
	g.log.Debugf("Gathered %s/%s.log in %.3f seconds (%.2f MiB/s)",
		container, which, elapsed, rate)
}

func listContainers(pod *unstructured.Unstructured) ([]*containerInfo, error) {
	statuses, found, err := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil
	}

	var result []*containerInfo

	for _, c := range statuses {
		status, ok := c.(map[string]interface{})
		if !ok {
			return nil, nil
		}

		name, found, err := unstructured.NestedString(status, "name")
		if err != nil {
			return nil, err
		}

		if !found {
			continue
		}

		result = append(result, &containerInfo{
			Namespace:      pod.GetNamespace(),
			Pod:            pod.GetName(),
			Name:           name,
			HasPreviousLog: containerHasPreviousLog(status),
		})
	}

	return result, nil
}

// containerHasPreviousLog returns true if we can get a previous log for a
// container, based on container status.
//
//	lastState:
//	  terminated:
//	    containerID: containerd://...
//
// See also https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/kubelet_pods.go#L1453
func containerHasPreviousLog(status map[string]interface{}) bool {
	containerID, found, err := unstructured.NestedString(status, "lastState", "terminated", "containerID")
	if err != nil || !found {
		return false
	}

	return containerID != ""
}
