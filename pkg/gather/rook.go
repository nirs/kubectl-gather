// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type RookAddon struct {
	name   string
	out    *OutputDirectory
	opts   *Options
	q      Queuer
	client *kubernetes.Clientset
	log    *zap.SugaredLogger
}

func NewRookCephAddon(config *rest.Config, client *http.Client, out *OutputDirectory, opts *Options, q Queuer) (Addon, error) {
	clientSet, err := kubernetes.NewForConfigAndClient(config, client)
	if err != nil {
		return nil, err
	}

	return &RookAddon{
		name:   "rook",
		out:    out,
		opts:   opts,
		q:      q,
		client: clientSet,
		log:    opts.Log.Named("rook"),
	}, nil
}

func (a *RookAddon) Inspect(cephcluster *unstructured.Unstructured) error {
	namespace := cephcluster.GetNamespace()
	a.log.Debugf("Inspecting cephcluster \"%s/%s\"", namespace, cephcluster.GetName())

	a.q.Queue(func() error {
		a.gatherCommands(namespace)
		return nil
	})

	if a.logCollectorEnabled(cephcluster) {
		dataDir, err := a.dataDirHostPath(cephcluster)
		if err != nil {
			a.log.Warnf("Cannot get cephcluster dataDirHostPath: %s", err)
			return nil
		}

		a.q.Queue(func() error {
			a.gatherLogs(namespace, dataDir)
			return nil
		})
	}

	return nil
}

func (a *RookAddon) gatherCommands(namespace string) {
	tools, err := a.findPod(namespace, "app=rook-ceph-tools")
	if err != nil {
		a.log.Warnf("Cannot find tools pod: %s", err)
		return
	}

	a.log.Debugf("Using pod %q", tools.Name)

	commands, err := a.out.CreateAddonDir(a.name, "commands")
	if err != nil {
		a.log.Warnf("Cannot create commnads directory: %s", err)
		return
	}

	a.log.Debugf("Storing commands output in %q", commands)

	rc := NewRemoteCommand(tools, a.opts, a.log, commands)

	// Running remote ceph commands in parallel is much faster.

	a.q.Queue(func() error {
		a.gatherCommand(rc, "ceph", "osd", "blocklist", "ls")
		return nil
	})

	a.gatherCommand(rc, "ceph", "status")
}

func (a *RookAddon) gatherCommand(rc *RemoteCommand, command ...string) {
	if err := rc.Gather(command...); err != nil {
		a.log.Warnf("Error running %q: %s", strings.Join(command, "-"), err)
	}
}

func (a *RookAddon) logCollectorEnabled(cephcluster *unstructured.Unstructured) bool {
	enabled, found, err := unstructured.NestedBool(cephcluster.Object, "spec", "logCollector", "enabled")
	if err != nil {
		a.log.Warnf("Cannot get cephcluster .spec.logCollector.enabled: %s", err)
	}
	return found && enabled
}

func (a *RookAddon) dataDirHostPath(cephcluster *unstructured.Unstructured) (string, error) {
	path, found, err := unstructured.NestedString(cephcluster.Object, "spec", "dataDirHostPath")
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("cannot find .spec.dataDirHostPath")
	}
	return path, nil
}

func (a *RookAddon) gatherLogs(namespace string, dataDir string) {
	nodes, err := a.findNodesToGather(namespace)
	if err != nil {
		a.log.Warnf("Cannot find nodes: %s", err)
		return
	}

	for i := range nodes {
		nodeName := nodes[i]
		a.q.Queue(func() error {
			a.gatherNodeLogs(namespace, nodeName, dataDir)
			return nil
		})
	}
}

func (a *RookAddon) findNodesToGather(namespace string) ([]string, error) {
	pods, err := a.client.CoreV1().
		Pods(namespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := sets.New[string]()

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Spec.NodeName != "" {
			names.Insert(pod.Spec.NodeName)
		}
	}

	return names.UnsortedList(), nil
}

func (a *RookAddon) gatherNodeLogs(namespace string, nodeName string, dataDir string) {
	a.log.Debugf("Gathering ceph logs from node %q dataDir %q", nodeName, dataDir)
	start := time.Now()

	agent, err := a.createAgentPod(nodeName, dataDir)
	if err != nil {
		a.log.Warnf("Cannot create agent pod: %s", err)
		return
	}
	defer agent.Delete()

	if err := agent.WaitUntilRunning(); err != nil {
		a.log.Warnf("Error waiting for agent pod: %s", agent, err)
		return
	}

	a.log.Debugf("Agent pod %q running in %.3f seconds", agent, time.Since(start).Seconds())

	logs, err := a.out.CreateAddonDir(a.name, "logs", nodeName)
	if err != nil {
		a.log.Warnf("Cannot create logs directory: %s", err)
		return
	}

	rd := NewRemoteDirectory(agent.Pod, a.opts, a.log)
	src := filepath.Join(dataDir, namespace, "log")

	if err := rd.Gather(src, logs); err != nil {
		a.log.Warnf("Cannot copy %q from agent pod %q: %s", src, agent.Pod.Name, err)
	}

	a.log.Debugf("Gathered node %q logs in %.3f seconds", nodeName, time.Since(start).Seconds())
}

func (a *RookAddon) createAgentPod(nodeName string, dataDir string) (*AgentPod, error) {
	agent := NewAgentPod(a.name+"-"+nodeName, a.client, a.log)
	agent.Pod.Spec.NodeName = nodeName
	agent.Pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "rook-data",
			MountPath: dataDir,
			ReadOnly:  true,
		},
	}
	agent.Pod.Spec.Volumes = []corev1.Volume{
		{
			Name: "rook-data",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: dataDir},
			},
		},
	}

	if err := agent.Create(); err != nil {
		return nil, err
	}

	return agent, nil
}

func (a *RookAddon) findPod(namespace string, labelSelector string) (*corev1.Pod, error) {
	pods, err := a.client.CoreV1().
		Pods(namespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod matches %q in namespace %q", labelSelector, namespace)
	}

	return &pods.Items[0], nil
}
