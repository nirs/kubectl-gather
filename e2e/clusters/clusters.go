package clusters

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

const (
	C1 = "c1"
	C2 = "c2"
)

var Names = []string{C1, C2}

func Create() error {
	log.Print("Creating clusters")
	profiles, err := profilesStatus()
	if err != nil {
		return err
	}
	var start []string
	for _, name := range Names {
		status := profiles[name]
		switch status {
		case "OK", "Running":
			log.Printf("Using existing cluster %q", name)
		case "", "Stopped":
			start = append(start, name)
		default:
			return fmt.Errorf("cluster %q status is %q", name, status)
		}
	}
	if err := execute(createCluster, start); err != nil {
		return err
	}
	log.Print("Clusters created")
	return nil
}

func Delete() error {
	log.Print("Deleting clusters")
	if err := execute(deleteCluster, Names); err != nil {
		return err
	}
	log.Print("Clusters deleted")
	return nil
}

func Load(archive string) error {
	log.Printf("Loading image %q", archive)
	if err := execute(func(name string) error {
		return runMinikube(name, "image", "load", archive)
	}, Names); err != nil {
		return err
	}
	log.Print("Image loaded")
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
	if err := startCluster(name); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		if err := configureDNS(name); err != nil {
			return err
		}
		if err := verifyDNS(name); err != nil {
			return err
		}
	}
	return nil
}

func deleteCluster(name string) error {
	log.Printf("Deleting cluster %q", name)
	return runMinikube(name, "delete")
}

// startCluster start the minikube cluster, creating it if it does not exist.
func startCluster(name string) error {
	log.Printf("Start cluster %q", name)
	args := []string{"start", "--memory", "3g"}
	switch runtime.GOOS {
	case "darwin":
		args = append(args, "--driver", "vfkit", "--network", "vmnet-shared")
	case "linux":
		args = append(args, "--driver", "docker")
	}
	return runMinikube(name, args...)
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
func configureDNS(name string) error {
	log.Printf("Configuring DNS in cluster %q", name)
	script := `\
resolvectl dns eth0 8.8.8.8 1.1.1.1
resolvectl domain eth0 "~."
resolvectl flush-caches`
	command := fmt.Sprintf("sudo bash -o errexit -c '%s'", script)
	return runMinikube(name, "ssh", "--", command)
}

// verifyDNS ensures that DNS works in the cluster, failing if not. Required on
// managed macOS machines to verify that our configuration works. Does not work
// on non-vm drivers on Linux.
func verifyDNS(name string) error {
	log.Printf("Verifying DNS in cluster %q", name)
	return runMinikube(name, "ssh", "--", "resolvectl", "query", "google.com")
}

type profileInfo struct {
	Name   string
	Status string
}

type profileList struct {
	Valid   []profileInfo `json:"valid"`
	Invalid []profileInfo `json:"invalid"`
}

func profilesStatus() (map[string]string, error) {
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

func runMinikube(name string, args ...string) error {
	cmd_args := []string{"--profile", name}
	cmd_args = append(cmd_args, args...)
	cmd := exec.Command("minikube", cmd_args...)
	log.Printf("[%s] Running %s", name, cmd)
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
				log.Printf("[%s] Failed to read from command stdout: %s", name, err)
			}
			break
		}
		log.Printf("[%s] %s", name, string(line))
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}
