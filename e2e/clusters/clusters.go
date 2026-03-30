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
	log.Printf("Creating cluster %q", name)
	args := []string{"start", "--memory", "3g"}
	switch runtime.GOOS {
	case "darwin":
		args = append(args, "--driver", "vfkit", "--network", "vmnet-shared")
	case "linux":
		args = append(args, "--driver", "podman")
	}
	return runMinikube(name, args...)
}

func deleteCluster(name string) error {
	log.Printf("Deleting cluster %q", name)
	return runMinikube(name, "delete")
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
