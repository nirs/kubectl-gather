package clusters

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/nirs/kubectl-gather/e2e/commands"
)

const kubeconfig = "clusters.yaml"

var names = []string{"kind-c1", "kind-c2"}

func Names() []string {
	return names
}

func Kubeconfig() string {
	return kubeconfig
}

func Create() error {
	log.Print("Creating clusters")
	if err := execute(createCluster, names); err != nil {
		return err
	}
	if err := createKubeconfig(); err != nil {
		return err
	}
	log.Print("Clusters created")
	return nil
}

func Delete() error {
	log.Print("Deleting clusters")
	if err := execute(deleteCluster, names); err != nil {
		return err
	}
	_ = os.Remove(kubeconfig)
	log.Print("Clusters deleted")
	return nil
}

func execute(fn func(name string) error, names []string) error {
	errors := make(chan error, len(names))
	wg := sync.WaitGroup{}
	for _, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fn(name)
			if err != nil {
				errors <- err
			}
		}()
	}
	wg.Wait()
	close(errors)
	for e := range errors {
		return e
	}
	return nil
}

func createCluster(name string) error {
	log.Printf("Creating cluster %q", name)
	exists, err := clusterExists(name)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("Using existing cluster: %q", name)
		return nil
	}
	cmd := exec.Command(
		"kind", "create", "cluster",
		"--name", kindName(name),
		"--kubeconfig", name+".yaml",
		"--wait", "60s",
	)
	return commands.LogStderr(cmd)
}

func deleteCluster(name string) error {
	log.Printf("Deleting cluster %q", name)
	config := name + ".yaml"
	cmd := exec.Command(
		"kind", "delete", "cluster",
		"--name", kindName(name),
		"--kubeconfig", config,
	)
	if err := commands.LogStderr(cmd); err != nil {
		return err
	}
	_ = os.Remove(config)
	return nil
}

func createKubeconfig() error {
	log.Printf("Creating kubconfigs %q", kubeconfig)
	var configs []string
	for _, name := range names {
		configs = append(configs, name+".yaml")
	}
	cmd := exec.Command("kubectl", "config", "view", "--flatten")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+strings.Join(configs, ":"))
	log.Printf("Running %v", cmd)
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to merge configs: %s: %s", err, commands.Stderr(err))
	}
	return os.WriteFile(kubeconfig, data, 0640)
}

func clusterExists(name string) (bool, error) {
	cmd := exec.Command("kind", "get", "clusters")
	log.Printf("Running %v", cmd)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("Failed to get clusters: %s: %s", err, commands.Stderr(err))
	}
	trimmed := strings.TrimSpace(string(out))
	existing := strings.Split(trimmed, "\n")
	return slices.Contains(existing, kindName(name)), nil
}

func kindName(name string) string {
	return strings.TrimPrefix(name, "kind-")
}
