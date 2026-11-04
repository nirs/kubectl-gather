// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"testing"
)

func TestResourceForKind(t *testing.T) {
	g := newTestGatherer(testSalt)
	g.resourceForKind = map[kindKey]string{
		{Group: "", Kind: "PersistentVolume"}:             "persistentvolumes",
		{Group: "storage.k8s.io", Kind: "StorageClass"}:   "storageclasses",
		{Group: "ramendr.openshift.io", Kind: "DRPolicy"}: "drpolicies",
	}

	tests := []struct {
		name  string
		group string
		kind  string
		want  string
	}{
		{
			name:  "core group resource",
			group: "",
			kind:  "PersistentVolume",
			want:  "persistentvolumes",
		},
		{
			name:  "named group resource",
			group: "storage.k8s.io",
			kind:  "StorageClass",
			want:  "storageclasses",
		},
		{
			name:  "another named group resource",
			group: "ramendr.openshift.io",
			kind:  "DRPolicy",
			want:  "drpolicies",
		},
		{
			name:  "unknown kind",
			group: "",
			kind:  "NoSuchKind",
			want:  "",
		},
		{
			name:  "known kind in wrong group",
			group: "wrong.group.io",
			kind:  "PersistentVolume",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := g.ResourceForKind(tc.group, tc.kind)
			if got != tc.want {
				t.Errorf("ResourceForKind(%q, %q) = %q, want %q",
					tc.group, tc.kind, got, tc.want)
			}
		})
	}
}

func TestResourceForKindEmpty(t *testing.T) {
	// A Gatherer built via New() always initializes resourceForKind, but
	// guard against a nil map (e.g. a zero-value Gatherer in tests) still
	// behaving as "not found" instead of panicking.
	g := newTestGatherer(testSalt)

	if got := g.ResourceForKind("", "PersistentVolume"); got != "" {
		t.Errorf("ResourceForKind on empty map = %q, want empty string", got)
	}
}
