package clusters

import (
	"encoding/json"
	"fmt"
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
		cmd := exec.Command("minikube", "image", "load", archive, "--profile", name)
		return commands.Run(cmd)
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
	args := []string{"start", "--profile", name, "--memory", "3g"}
	switch runtime.GOOS {
	case "darwin":
		args = append(args, "--driver", "vfkit", "--network", "vmnet-shared")
	case "linux":
		args = append(args, "--driver", "podman")
	}
	cmd := exec.Command("minikube", args...)
	return commands.Run(cmd)
}

func deleteCluster(name string) error {
	log.Printf("Deleting cluster %q", name)
	cmd := exec.Command("minikube", "delete", "--profile", name)
	return commands.Run(cmd)
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
