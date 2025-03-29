// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

const (
	logsName = "logs"
)

type LogsAddon struct {
	AddonBackend
	client *kubernetes.Clientset
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

func init() {
	registerAddon(logsName, addonInfo{
		Resource:  "pods",
		AddonFunc: NewLogsAddon,
	})
}

func NewLogsAddon(backend AddonBackend) (Addon, error) {
	client, err := kubernetes.NewForConfigAndClient(backend.Config(), backend.HTTPClient())
	if err != nil {
		return nil, err
	}

	return &LogsAddon{
		AddonBackend: backend,
		client:       client,
		log:          backend.Options().Log.Named(logsName),
	}, nil
}

func (a *LogsAddon) Inspect(pod *unstructured.Unstructured) error {
	a.log.Debugf("Inspecting pod \"%s/%s\"", pod.GetNamespace(), pod.GetName())

	containers, err := a.listContainers(pod)
	if err != nil {
		return fmt.Errorf("cannot find containers in pod \"%s/%s\": %s",
			pod.GetNamespace(), pod.GetName(), err)
	}

	for i := range containers {
		container := containers[i]

		a.Queue(func() error {
			opts := corev1.PodLogOptions{Container: container.Name}
			a.gatherContainerLog(container, &opts)
			return nil
		})

		if container.HasPreviousLog {
			a.Queue(func() error {
				opts := corev1.PodLogOptions{Container: container.Name, Previous: true}
				a.gatherContainerLog(container, &opts)
				return nil
			})
		}
	}

	return nil
}

func (a *LogsAddon) gatherContainerLog(container *containerInfo, opts *corev1.PodLogOptions) {
	start := time.Now()

	which := "current"
	if opts.Previous {
		which = "previous"
	}

	req := a.client.CoreV1().Pods(container.Namespace).GetLogs(container.Pod, opts)

	src, err := req.Stream(context.TODO())
	if err != nil {
		// Getting the log is possible only if a container is running, but
		// checking the container state before the call is racy. We get a
		// BadRequest error like: "container ... in pod ... is waiting to start:
		// PodInitializing" so there is no way to detect the actual problem.
		// Since this is expected situation, and getting logs is best effort, we
		// log this in debug level.
		a.log.Debugf("Cannot get log for \"%s/%s\": %v", container, which, err)
		return
	}

	defer src.Close()

	dst, err := a.Output().CreateContainerLog(
		container.Namespace, container.Pod, container.Name, string(which))
	if err != nil {
		a.log.Warnf("Cannot create \"%s/%s.log\": %s", container, which, err)
		return
	}

	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		a.log.Warnf("Cannot copy \"%s/%s.log\": %s", container, which, err)
	}

	elapsed := time.Since(start).Seconds()
	rate := float64(n) / float64(1024*1024) / elapsed
	a.log.Debugf("Gathered \"%s/%s.log\" in %.3f seconds (%.2f MiB/s)",
		container, which, elapsed, rate)
}

func (a *LogsAddon) listContainers(pod *unstructured.Unstructured) ([]*containerInfo, error) {
	var result []*containerInfo

	for _, key := range []string{"containerStatuses", "initContainerStatuses"} {
		statuses, found, err := unstructured.NestedSlice(pod.Object, "status", key)
		if err != nil {
			a.log.Warnf("Cannot get %q for pod \"%s/%s\": %s",
				key, pod.GetNamespace(), pod.GetName(), err)
			continue
		}

		if !found {
			continue
		}

		for _, c := range statuses {
			status, ok := c.(map[string]interface{})
			if !ok {
				a.log.Warnf("Invalid container status for pod \"%s/%s\": %s",
					pod.GetNamespace(), pod.GetName(), status)
				continue
			}

			name, found, err := unstructured.NestedString(status, "name")
			if err != nil || !found {
				a.log.Warnf("No container status name for pod \"%s/%s\": %s",
					pod.GetNamespace(), pod.GetName(), status)
				continue
			}

			result = append(result, &containerInfo{
				Namespace:      pod.GetNamespace(),
				Pod:            pod.GetName(),
				Name:           name,
				HasPreviousLog: containerHasPreviousLog(status),
			})
		}
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
