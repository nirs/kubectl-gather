package e2e

import (
	"context"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/nirs/kubectl-gather/e2e/clusters"
)

// deleteMustGatherNamespaces removes any openshift-must-gather-* namespaces
// left behind by oc adm must-gather, which does not clean up on signal.
// Cleans up all contexts in parallel and waits for the namespaces to be
// fully deleted.
func deleteMustGatherNamespaces(t *testing.T) {
	t.Helper()
	var wg sync.WaitGroup
	for _, ctx := range clusters.Names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deleteMustGatherNamespacesFromCluster(t, ctx)
		}()
	}
	wg.Wait()
}

func deleteMustGatherNamespacesFromCluster(t *testing.T, clusterContext string) {
	t.Helper()
	client := newClientset(t, clusterContext)

	deleting, err := mustGatherNamespaces(client)
	if err != nil {
		log.Printf("failed to list namespaces on %q: %s", clusterContext, err)
		return
	}
	for name := range deleting {
		err := client.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("failed to delete namespace %q on %q: %s", name, clusterContext, err)
			delete(deleting, name)
			continue
		}
		log.Printf("deleting leftover namespace %q on %q", name, clusterContext)
	}

	for len(deleting) > 0 {
		time.Sleep(time.Second)

		remaining, err := mustGatherNamespaces(client)
		if err != nil {
			log.Printf("failed to list namespaces on %q: %s", clusterContext, err)
			return
		}
		for name := range deleting {
			if _, ok := remaining[name]; !ok {
				log.Printf("namespace %q on %q was deleted", name, clusterContext)
				delete(deleting, name)
			}
		}
	}
}

func mustGatherNamespaces(client *kubernetes.Clientset) (map[string]struct{}, error) {
	namespaces, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{})
	for _, ns := range namespaces.Items {
		if strings.HasPrefix(ns.Name, "openshift-must-gather-") {
			names[ns.Name] = struct{}{}
		}
	}
	return names, nil
}

