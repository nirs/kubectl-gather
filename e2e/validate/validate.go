package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func Exists(t *testing.T, outputDir string, clusterNames []string, resources []string) {
	if !pathExists(t, outputDir) {
		t.Fatalf("output directory %q does not exist", outputDir)
	}

	for _, cluster := range clusterNames {
		clusterDir := filepath.Join(outputDir, cluster)
		if !pathExists(t, clusterDir) {
			t.Fatalf("cluster directory %q does not exist", clusterDir)
		}
		for _, expectedFile := range resources {
			resource := filepath.Join(clusterDir, expectedFile)
			matches, err := filepath.Glob(resource)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) == 0 {
				t.Errorf("expected resource %q does not exist", resource)
			}
		}
	}
}

func Missing(t *testing.T, outputDir string, clusterNames []string, resources []string) {
	if !pathExists(t, outputDir) {
		t.Fatalf("output directory %q does not exist", outputDir)
	}

	for _, cluster := range clusterNames {
		for _, expectedFile := range resources {
			resource := filepath.Join(outputDir, cluster, expectedFile)
			matches, err := filepath.Glob(resource)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) > 0 {
				t.Errorf("expected resource %q should not exist: %q", expectedFile, matches)
			}
		}
	}
}

func JSONLog(t *testing.T, logPath string) {
	if !pathExists(t, logPath) {
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

func pathExists(t *testing.T, path string) bool {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("error checking path %q: %v", path, err)
		}
		return false
	}
	return true
}
