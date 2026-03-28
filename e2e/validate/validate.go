package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Validator checks that gathered resources exist or are missing in the output
// directory. For remote gather, the data root (image digest directory) is
// inserted between the cluster directory and the resource patterns.
type Validator struct {
	outputDir string
	dataRoot  string
}

func New(outputDir string) *Validator {
	return &Validator{outputDir: outputDir}
}

// WithDataRoot returns a new Validator with the given data root for remote
// gather output where resources are nested under an image digest directory.
func (v *Validator) WithDataRoot(dataRoot string) *Validator {
	return &Validator{outputDir: v.outputDir, dataRoot: dataRoot}
}

func (v *Validator) Exists(t *testing.T, clusterNames []string, resources ...[]string) {
	t.Helper()

	if !PathExists(t, v.outputDir) {
		t.Fatalf("output directory %q does not exist", v.outputDir)
	}

	for _, cluster := range clusterNames {
		clusterDir := filepath.Join(v.outputDir, cluster, v.dataRoot)
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

func (v *Validator) Missing(t *testing.T, clusterNames []string, resources ...[]string) {
	t.Helper()

	if !PathExists(t, v.outputDir) {
		t.Fatalf("output directory %q does not exist", v.outputDir)
	}

	for _, cluster := range clusterNames {
		clusterDir := filepath.Join(v.outputDir, cluster, v.dataRoot)
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
