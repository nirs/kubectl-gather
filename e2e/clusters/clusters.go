package clusters

import (
	"fmt"
	"log"
	"sync"

	"github.com/nirs/kubectl-gather/e2e/clusters/minikube"
)

const (
	C1 = "c1"
	C2 = "c2"
)

var Names = []string{C1, C2}

func Create() error {
	log.Print("Creating clusters")
	profiles, err := minikube.ClustersStatus()
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
	if err := execute(func(name string) error {
		return minikube.New(name).Create()
	}, start); err != nil {
		return err
	}
	log.Print("Clusters created")
	return nil
}

func Delete() error {
	log.Print("Deleting clusters")
	if err := execute(func(name string) error {
		return minikube.New(name).Delete()
	}, Names); err != nil {
		return err
	}
	log.Print("Clusters deleted")
	return nil
}

func Load(archive string) error {
	log.Printf("Loading image %q", archive)
	if err := execute(func(name string) error {
		return minikube.New(name).Load(archive)
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
