package e2e

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/pkg/gather"
	"go.uber.org/zap"
)

func TestAgentPodCleanupOnCancel(t *testing.T) {
	client := newClientset(t, clusters.C1)
	log := zap.NewNop().Sugar()
	agent := gather.NewAgentPod("cancel-test", client, log)

	ctx, cancel := context.WithCancel(context.Background())

	if err := agent.Create(ctx); err != nil {
		t.Fatal(err)
	}
	defer agent.Delete()

	time.Sleep(200 * time.Millisecond)
	cancel()

	if err := agent.WaitUntilRunning(ctx); err == nil {
		t.Fatal("WaitUntilRunning should fail with cancelled context")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %s", err)
	}

	agent.Delete()
	assertPodDeleted(t, client, agent)
}

func TestAgentPodCleanupOnTimeout(t *testing.T) {
	client := newClientset(t, clusters.C1)
	log := zap.NewNop().Sugar()
	agent := gather.NewAgentPod("timeout-test", client, log)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := agent.Create(ctx); err != nil {
		t.Fatal(err)
	}
	defer agent.Delete()

	if err := agent.WaitUntilRunning(ctx); err == nil {
		t.Fatal("WaitUntilRunning should fail with deadline exceeded")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %s", err)
	}

	agent.Delete()
	assertPodDeleted(t, client, agent)
}

func assertPodDeleted(t *testing.T, client *kubernetes.Clientset, agent *gather.AgentPod) {
	t.Helper()
	pod, err := client.CoreV1().Pods(agent.Pod.Namespace).
		Get(context.Background(), agent.Pod.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("unexpected error checking agent pod: %s", err)
		}
		log.Printf("agent pod %q already removed", agent.Pod.Name)
		return
	}
	if pod.DeletionTimestamp == nil {
		t.Fatalf("agent pod %q exists and is not marked for deletion", agent.Pod.Name)
	}
}
