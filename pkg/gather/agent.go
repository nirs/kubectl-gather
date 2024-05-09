// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	agentPodTimeoutSeconds = 60
)

type AgentPod struct {
	Client *kubernetes.Clientset
	Log    *zap.SugaredLogger
	Pod    *corev1.Pod
}

func NewAgentPod(name string, client *kubernetes.Clientset, log *zap.SugaredLogger) *AgentPod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gather-agent-" + name,

			// TODO: Use a tempoary random gather namespace so we don't leave
			// leftovers in real namespaces, and if we leave leftovers is it
			// easy to clean up.
			Namespace: corev1.NamespaceDefault,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "agent",

					// TODO: make configurable
					Image: "quay.io/nirsof/busybox:stable-musl",

					// The agent should stop automatically so if we fail to
					// delete it, so we don't waste resources on the target
					// cluster. We trap SIGTERM so it terminates immediatly when
					// deleted.
					Command: []string{"sh", "-c", "trap exit TERM; sleep 900"},
				},
			},
		},
	}

	return &AgentPod{Pod: &pod, Client: client, Log: log}
}

func (a *AgentPod) Create() error {
	a.Log.Debugf("Starting agent pod %s/%s", a.Pod.Namespace, a.Pod.Name)
	pod, err := a.Client.CoreV1().Pods(a.Pod.Namespace).
		Create(context.TODO(), a.Pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	a.Pod = pod
	return nil
}

func (a *AgentPod) WaitUntilRunning() error {
	// TODO: use tools/watch for handling watch errors?

	timeout := int64(agentPodTimeoutSeconds)
	selector := fields.OneTermEqualSelector(metav1.ObjectNameField, a.Pod.Name).String()

	watcher, err := a.Client.CoreV1().Pods(a.Pod.Namespace).
		Watch(context.TODO(), metav1.ListOptions{
			TimeoutSeconds: &timeout,
			FieldSelector:  selector,
		})
	if err != nil {
		return err
	}

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Modified:
			obj := event.Object.(*corev1.Pod)
			switch obj.Status.Phase {
			case corev1.PodRunning:
				return nil
			case corev1.PodFailed:
				return fmt.Errorf("agent pod failed")
			case corev1.PodSucceeded:
				return fmt.Errorf("agent pod terminated")
			}
		case watch.Error:
			a.Log.Warnf("Watch error: %s", event)
		case watch.Deleted:
			return fmt.Errorf("agent pod was deleted")
		}
	}

	return fmt.Errorf("timeout waiting for agent pod running phase")
}

func (a *AgentPod) Delete() {
	a.Log.Debugf("Deleting agent pod %s/%s", a.Pod.Namespace, a.Pod.Name)
	err := a.Client.CoreV1().Pods(a.Pod.Namespace).
		Delete(context.TODO(), a.Pod.Name, metav1.DeleteOptions{})
	if err != nil {
		a.Log.Warnf("Cannot delete agent pod %s/%s", a.Pod.Namespace, a.Pod.Name)
	}
}