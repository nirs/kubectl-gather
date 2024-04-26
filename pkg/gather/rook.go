// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type RookAddon struct {
	name   string
	out    *OutputDirectory
	opts   *Options
	client *kubernetes.Clientset
	log    *log.Logger
}

func NewRookCephAddon(config *rest.Config, client *http.Client, out *OutputDirectory, opts *Options) (*RookAddon, error) {
	clientSet, err := kubernetes.NewForConfigAndClient(config, client)
	if err != nil {
		return nil, err
	}

	return &RookAddon{
		name:   "rook-ceph",
		out:    out,
		opts:   opts,
		client: clientSet,
		log:    createLogger("rook", opts),
	}, nil
}

func (a *RookAddon) Gather(cephcluster *unstructured.Unstructured) error {
	start := time.Now()

	namespace := cephcluster.GetNamespace()
	a.log.Printf("Gathering data for cephcluster %s/%s", namespace, cephcluster.GetName())

	a.gatherCommands(namespace)
	a.gatherLogs(namespace)

	a.log.Printf("Gathered %s data in %.3f seconds", a.name, time.Since(start).Seconds())

	return nil
}

func (a *RookAddon) gatherCommands(namespace string) {
	start := time.Now()

	tools, err := a.findPod(namespace, "app=rook-ceph-tools")
	if err != nil {
		a.log.Printf("Cannot find rook-ceph-tools pod: %s", err)
		return
	}

	a.log.Printf("Using pod %s", tools.Name)

	commands, err := a.out.CreateAddonDir(a.name, "commands")
	if err != nil {
		a.log.Printf("Cannot create %s commnads directory: %s", a.name, err)
		return
	}

	a.log.Printf("Storing commands output in %s", commands)

	rc := a.remoteCommand(tools, commands)

	err = rc.Gather("ceph", "status")
	if err != nil {
		a.log.Printf("Error running 'ceph status': %s", err)
	}

	err = rc.Gather("ceph", "osd", "blocklist", "ls")
	if err != nil {
		a.log.Printf("Error running 'ceph osd blocklist ls': %s", err)
	}

	a.log.Printf("Gathered %s commands in %.3f seconds", a.name, time.Since(start).Seconds())
}

func (a *RookAddon) gatherLogs(namespace string) {
	start := time.Now()

	a.log.Printf("Gathering ceph logs")

	mgr, err := a.findPod(namespace, "app=rook-ceph-mgr")
	if err != nil {
		a.log.Printf("Cannot find rook-ceph-mgr pod: %s", err)
		return
	}

	a.log.Printf("Using pod %s", mgr.Name)

	logs, err := a.out.CreateAddonDir(a.name, "logs")
	if err != nil {
		a.log.Printf("Cannot create %s logs directory: %s", a.name, err)
		return
	}

	a.log.Printf("Copying logs to %s", logs)

	rd := a.remoteDirectory(mgr)

	if err := rd.Gather("/var/log/ceph", logs); err != nil {
		a.log.Printf("Cannot copy /var/log/ceph in pod %s: %s", mgr.Name, err)
	}

	a.log.Printf("Gathered %s logs in %.3f seconds", a.name, time.Since(start).Seconds())
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
		return nil, fmt.Errorf("no pod matches %s in namespace %s", labelSelector, namespace)
	}

	return &pods.Items[0], nil
}

func (a *RookAddon) remoteCommand(pod *corev1.Pod, dir string) *RemoteCommand {
	return &RemoteCommand{
		Kubeconfig: a.opts.Kubeconfig,
		Context:    a.opts.Context,
		Namespace:  pod.Namespace,
		Pod:        pod.Name,
		Container:  pod.Spec.Containers[0].Name,
		Directory:  dir,
	}
}

func (a *RookAddon) remoteDirectory(pod *corev1.Pod) *RemoteDirectory {
	return &RemoteDirectory{
		Kubeconfig: a.opts.Kubeconfig,
		Context:    a.opts.Context,
		Namespace:  pod.Namespace,
		Pod:        pod.Name,
		Container:  pod.Spec.Containers[0].Name,
		Log:        a.log,
	}
}
