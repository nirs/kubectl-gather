// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

const (
	logsName = "logs"

	// Number of attempts to copy a container log when the stream is
	// interrupted by a graceful shutdown of the connection.
	logStreamAttempts = 3
)

type LogsAddon struct {
	addonMeta
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

// logStreamer opens a new container log stream. Implemented by *rest.Request.
type logStreamer interface {
	Stream(ctx context.Context) (io.ReadCloser, error)
}

func init() {
	registerAddon(logsName, addonInfo{
		Resources: []string{"pods"},
		AddonFunc: NewLogsAddon,
	})
}

func NewLogsAddon(backend AddonBackend) (Addon, error) {
	client, err := kubernetes.NewForConfigAndClient(backend.Config(), backend.HTTPClient())
	if err != nil {
		return nil, err
	}

	return &LogsAddon{
		addonMeta:    addonMeta{name: logsName},
		AddonBackend: backend,
		client:       client,
		log:          backend.Options().Log.Named(logsName),
	}, nil
}

func (a *LogsAddon) Inspect(pod *unstructured.Unstructured, _ *time.Time) error {
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
	which := "current"
	if opts.Previous {
		which = "previous"
	}

	req := a.client.CoreV1().Pods(container.Namespace).GetLogs(container.Pod, opts)

	a.copyContainerLog(container, which, req)
}

func (a *LogsAddon) copyContainerLog(container *containerInfo, which string, req logStreamer) {
	start := time.Now()

	var n int64

	for attempt := 1; attempt <= logStreamAttempts; attempt++ {
		src, err := req.Stream(context.TODO())
		if err != nil {
			if attempt > 1 {
				// We already copied some data, so the container was running and
				// failing to stream it again is not expected.
				a.log.Warnf("Cannot get log for \"%s/%s\": %s", container, which, err)
				return
			}

			// Getting the log is possible only if a container is running, but
			// checking the container state before the call is racy. We get a
			// BadRequest error like: "container ... in pod ... is waiting to start:
			// PodInitializing" so there is no way to detect the actual problem.
			// Since this is expected situation, and getting logs is best effort, we
			// log this in debug level.
			a.log.Debugf("Cannot get log for \"%s/%s\": %v", container, which, err)
			return
		}

		// Creating the log truncates data copied by a previous attempt. The log
		// API cannot resume an interrupted transfer, so we must copy it again.
		dst, err := a.Output().CreateContainerLog(
			container.Namespace, container.Pod, container.Name, which)
		if err != nil {
			src.Close()
			a.log.Warnf("Cannot create \"%s/%s.log\": %s", container, which, err)
			return
		}

		n, err = io.Copy(dst, src)

		src.Close()
		dst.Close()

		if err == nil {
			break
		}

		if attempt < logStreamAttempts && isGoAwayError(err) {
			a.log.Warnf("Retrying \"%s/%s.log\": %s", container, which, err)
			continue
		}

		a.log.Warnf("Cannot copy \"%s/%s.log\": %s", container, which, err)

		break
	}

	elapsed := time.Since(start).Seconds()
	rate := float64(n) / float64(1024*1024) / elapsed
	a.log.Debugf("Gathered \"%s/%s.log\" in %.3f seconds (%.2f MiB/s)",
		container, which, elapsed, rate)
}

// isGoAwayError tells if the stream was interrupted by a GOAWAY frame, sent
// when the server closes the connection gracefully (RFC 9113 section 6.8). We
// cannot use errors.As since net/http bundles a private copy of the http2
// package using an unexported error type, see
// https://github.com/golang/go/issues/28930.
func isGoAwayError(err error) bool {
	return strings.Contains(err.Error(), "http2: server sent GOAWAY")
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
	containerID, found, err := unstructured.NestedString(
		status,
		"lastState",
		"terminated",
		"containerID",
	)
	if err != nil || !found {
		return false
	}

	return containerID != ""
}
