package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func Exists(t *testing.T, outputDir string, clusterNames []string, resources ...[]string) {
	if !PathExists(t, outputDir) {
		t.Fatalf("output directory %q does not exist", outputDir)
	}

	for _, cluster := range clusterNames {
		clusterDir := filepath.Join(outputDir, cluster)
		if !PathExists(t, clusterDir) {
			t.Fatalf("cluster directory %q does not exist", clusterDir)
		}
		for _, pattern := range slices.Concat(resources...) {
			resource := filepath.Join(clusterDir, pattern)
			matches, err := filepath.Glob(resource)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) == 0 {
				t.Errorf("resource %q does not exist", resource)
			}
		}
	}
}

func Missing(t *testing.T, outputDir string, clusterNames []string, resources ...[]string) {
	if !PathExists(t, outputDir) {
		t.Fatalf("output directory %q does not exist", outputDir)
	}

	for _, cluster := range clusterNames {
		clusterDir := filepath.Join(outputDir, cluster)
		for _, pattern := range slices.Concat(resources...) {
			resource := filepath.Join(clusterDir, pattern)
			matches, err := filepath.Glob(resource)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) > 0 {
				t.Errorf("resource %q should not exist: %q", resource, matches)
			}
		}
	}
}

func JSONLog(t *testing.T, logPath string) {
	if !PathExists(t, logPath) {
		t.Fatalf("log %q does not exist", logPath)
	}

	file, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	lineNum := 0
	for decoder.More() {
		lineNum++
		var jsonData map[string]interface{}
		if err := decoder.Decode(&jsonData); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", lineNum, err)
		}
	}
}

func PathExists(t *testing.T, path string) bool {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("error checking path %q: %v", path, err)
		}
		return false
	}
	return true
}
