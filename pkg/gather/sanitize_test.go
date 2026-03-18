// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aymanbagabas/go-udiff"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

var testSalt = Salt{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

func TestSanitizeResource(t *testing.T) {
	tests := []string{
		"secret-with-data",
		"secret-with-annotations",
		"secret-empty",
		"configmap",
	}

	g := newTestGatherer(testSalt)

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			obj := loadTestdata(t, name+".yaml")
			g.sanitizeResource(obj)
			actual := marshalYAML(t, obj.Object)

			golden := filepath.Join("testdata", "sanitize", name+".golden.yaml")
			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if actual != string(expected) {
				t.Errorf("mismatch:\n%s", unifiedDiff(t, string(expected), actual))
			}
		})
	}
}

func TestSanitizeSecretIdempotent(t *testing.T) {
	g := newTestGatherer(testSalt)

	obj := loadTestdata(t, "secret-with-data.yaml")
	g.sanitizeResource(obj)
	expected := marshalYAML(t, obj.Object)

	g.sanitizeResource(obj)
	actual := marshalYAML(t, obj.Object)

	if expected != actual {
		t.Fatalf("sanitizing twice changed the secret:\n%s", unifiedDiff(t, expected, actual))
	}
}

func TestSanitizeSecretDifferentSalts(t *testing.T) {
	g1 := newTestGatherer(RandomSalt())
	g2 := newTestGatherer(RandomSalt())

	s1 := loadTestdata(t, "secret-with-data.yaml")
	s2 := loadTestdata(t, "secret-with-data.yaml")

	g1.sanitizeResource(s1)
	g2.sanitizeResource(s2)

	d1, found, err := unstructured.NestedStringMap(s1.Object, "data")
	if err != nil || !found {
		t.Fatalf("failed to get data from secret 1: err=%v, found=%v", err, found)
	}
	d2, found, err := unstructured.NestedStringMap(s2.Object, "data")
	if err != nil || !found {
		t.Fatalf("failed to get data from secret 2: err=%v, found=%v", err, found)
	}

	if d1["secret1"] == d2["secret1"] {
		t.Fatalf("different salts must produce different hashes, both got %q", d1["secret1"])
	}
}

func TestRandomSalt(t *testing.T) {
	values := map[Salt]struct{}{}
	for range 1000 {
		values[RandomSalt()] = struct{}{}
	}
	if len(values) != 1000 {
		t.Fatalf("duplicate random salt: got %d unique out of 1000", len(values))
	}
}

func BenchmarkHashValue(b *testing.B) {
	data := []byte("random secure key, 32 bytes long")
	for b.Loop() {
		HashValue(data, testSalt)
	}
}

// Test helpers

func newTestGatherer(salt Salt) *Gatherer {
	return &Gatherer{
		opts: &Options{
			Salt: salt,
			Log:  zap.NewNop().Sugar(),
		},
	}
}

func loadTestdata(t *testing.T, name string) *unstructured.Unstructured {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "sanitize", name))
	if err != nil {
		t.Fatal(err)
	}
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, &obj.Object); err != nil {
		t.Fatal(err)
	}
	return obj
}

func marshalYAML(t *testing.T, obj any) string {
	t.Helper()
	data, err := yaml.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func unifiedDiff(t *testing.T, expected, actual any) string {
	t.Helper()
	expectedString := marshal(t, expected)
	actualString := marshal(t, actual)
	return udiff.Unified("expected", "actual", expectedString, actualString)
}

func marshal(t *testing.T, obj any) string {
	t.Helper()
	if s, ok := obj.(string); ok {
		return s
	}
	return marshalYAML(t, obj)
}
