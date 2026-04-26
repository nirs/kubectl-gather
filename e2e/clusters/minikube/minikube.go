package minikube

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"
	"sync"

	"github.com/nirs/kubectl-gather/e2e/commands"
)

type Cluster struct {
	Name string
}

type profileInfo struct {
	Name   string
	Status string
}

type profileList struct {
	Valid   []profileInfo `json:"valid"`
	Invalid []profileInfo `json:"invalid"`
}

func ClustersStatus() (map[string]string, error) {
	status := map[string]string{}
	cmd := exec.Command("minikube", "profile", "list", "--output", "json")
	log.Printf("Running %v", cmd)
	out, err := cmd.Output()
	if err != nil {
		return status, fmt.Errorf("failed to list profiles: %w: %s", err, commands.Stderr(err))
	}
	profiles := profileList{}
	if err := json.Unmarshal(out, &profiles); err != nil {
		return status, fmt.Errorf("failed to unmarshal profile list: %w", err)
	}
	for _, profile := range profiles.Valid {
		status[profile.Name] = profile.Status
	}
	for _, profile := range profiles.Invalid {
		status[profile.Name] = profile.Status
	}
	return status, nil
}

func New(name string) *Cluster {
	return &Cluster{name}
}

func (c *Cluster) Create() error {
	log.Printf("Creating cluster %q", c.Name)
	if err := c.startCluster(); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		if err := c.configureDNS(); err != nil {
			return err
		}
		if err := c.verifyDNS(); err != nil {
			return err
		}
	}
	return nil
}

// deleteMutex serializes delete operations to work around minikube not locking
// ~/.kube/config during cleanup. Without this, concurrent deletes race and
// one leaves stale kubeconfig entries.
var deleteMutex sync.Mutex

func (c *Cluster) Delete() error {
	deleteMutex.Lock()
	defer deleteMutex.Unlock()
	log.Printf("Deleting cluster %q", c.Name)
	return c.run("delete")
}

func (c *Cluster) Load(archive string) error {
	log.Printf("Loading image %q in cluster %q", archive, c.Name)
	return c.run("image", "load", archive)
}

// startCluster start the minikube cluster, creating it if it does not exist.
func (c *Cluster) startCluster() error {
	log.Printf("Start cluster %q", c.Name)
	args := []string{"start", "--memory", "3g"}
	switch runtime.GOOS {
	case "darwin":
		args = append(args, "--driver", "vfkit", "--network", "vmnet-shared")
	case "linux":
		args = append(args, "--driver", "docker")
	}
	return c.run(args...)
}

// configureDNS configure the cluster to use public DNS server. Required only on
// managed macOS machines, but harmless on unmanaged machines.
//
// On managed Macs, corporate security agents install network extensions that
// silently discard DNS traffic from the vmnet bridge (192.168.105.0/24).
// However, DNS traffic to public servers (e.g., 8.8.8.8) is forwarded via NAT
// normally.
//
// We configure systemd-resolved in the minikube VM with two settings:
//
//  1. Public DNS servers that are reachable from the VM, bypassing the host's
//     broken DNS path.
//
//  2. Routing domain (eth0 "~.") The "~." syntax tells systemd-resolved to
//     route ALL DNS queries through eth0's DNS servers.  Without this,
//     systemd-resolved might still try other interfaces' DNS servers.
func (c *Cluster) configureDNS() error {
	log.Printf("Configuring DNS in cluster %q", c.Name)
	script := `\
resolvectl dns eth0 8.8.8.8 1.1.1.1
resolvectl domain eth0 "~."
resolvectl flush-caches`
	command := fmt.Sprintf("sudo bash -o errexit -c '%s'", script)
	return c.run("ssh", "--", command)
}

// verifyDNS ensures that DNS works in the cluster, failing if not. Required on
// managed macOS machines to verify that our configuration works. Does not work
// on non-vm drivers on Linux.
func (c *Cluster) verifyDNS() error {
	log.Printf("Verifying DNS in cluster %q", c.Name)
	return c.run("ssh", "--", "resolvectl", "query", "google.com")
}

// run executes a minikube command, streaming stdout to the log.
func (c *Cluster) run(args ...string) error {
	args = append([]string{"--profile", c.Name}, args...)
	cmd := exec.Command("minikube", args...)
	log.Printf("[%s] Running %s", c.Name, cmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	reader := bufio.NewReader(pipe)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Printf("[%s] Failed to read from command stdout: %s", c.Name, err)
			}
			break
		}
		log.Printf("[%s] %s", c.Name, string(line))
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}
