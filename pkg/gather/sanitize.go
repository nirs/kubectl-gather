// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/pbkdf2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Salt is a 16-byte (128-bit) salt for PBKDF2 hashing.
type Salt [16]byte

const (
	// OWASP recommended iterations for PBKDF2-HMAC-SHA256, needed to protect
	// weak passwords in user applications. Could use fewer iterations if we
	// know secrets are random keys (e.g. ceph keys).
	// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#pbkdf2
	pbkdf2Iterations = 600_000

	// sanitizedAnnotation is added to sanitized secrets with the base64-encoded
	// salt. Prevents double-hashing on repeated runs and makes the salt visible
	// in the gathered output for verification.
	sanitizedAnnotation = "kubectl-gather.nirs.github.com/sanitized"

	lastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
)

// RandomSalt generates a cryptographically secure random salt.
// It may panic if the system cannot provide random data.
func RandomSalt() Salt {
	var salt Salt
	// rand.Read panics if the underlying OS returns an error.
	_, _ = rand.Read(salt[:])
	return salt
}

// HashValue returns the PBKDF2-HMAC-SHA256 hash of data with the given salt.
func HashValue(data []byte, salt Salt) []byte {
	return pbkdf2.Key(data, salt[:], pbkdf2Iterations, sha256.Size, sha256.New)
}

// sanitizeResource modifies the resource in place, replacing sensitive data
// with deterministic hashes. Currently handles Secret resources, other resource
// types pass through unchanged.
func (g *Gatherer) sanitizeResource(item *unstructured.Unstructured) {
	switch item.GetKind() {
	case "Secret":
		g.sanitizeSecret(item)
	}
}

// sanitizeSecret hashes secret data values and strips the
// last-applied-configuration annotation.
func (g *Gatherer) sanitizeSecret(item *unstructured.Unstructured) {
	log := g.opts.Log
	obj := item.Object
	name := item.GetName()
	salt := g.opts.Salt
	saltB64 := base64.StdEncoding.EncodeToString(salt[:])

	// If already sanitized, skip to avoid double hashing.
	existing, found, _ := unstructured.NestedString(obj, "metadata", "annotations", sanitizedAnnotation)
	if found {
		if existing != saltB64 {
			log.Warnf("Secret %q: already sanitized with different salt %q", name, existing)
		}
		return
	}

	// Replace each secret value with a deterministic PBKDF2 hash.
	data, found, err := unstructured.NestedStringMap(obj, "data")
	if err != nil {
		log.Warnf("Secret %q: failed to read data, removing data: %s", name, err)
		unstructured.RemoveNestedField(obj, "data")
	}
	if found {
		hashed := make(map[string]string, len(data))
		for key, value := range data {
			raw, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				log.Warnf("Secret %q: dropping key %q: invalid base64: %s", name, key, err)
				continue
			}
			h := HashValue(raw, salt)
			hashed[key] = base64.StdEncoding.EncodeToString(h)
		}
		if err := unstructured.SetNestedStringMap(obj, hashed, "data"); err != nil {
			log.Warnf("Secret %q: failed to set sanitized data: %s", name, err)
		}
	}

	// Remove last-applied-configuration which may contain plaintext secret data.
	unstructured.RemoveNestedField(obj, "metadata", "annotations", lastAppliedConfigAnnotation)

	// Mark the secret as sanitized with the salt used.
	if err := unstructured.SetNestedField(obj, saltB64, "metadata", "annotations", sanitizedAnnotation); err != nil {
		log.Warnf("Secret %q: failed to set sanitized annotation: %s", name, err)
	}
}
