// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"net/http"

	"k8s.io/client-go/rest"
)

func createAddons(config *rest.Config, client *http.Client, out *OutputDirectory, opts *Options, q Queuer) (map[string]Addon, error) {
	logsAddon, err := NewLogsAddon(config, client, out, opts, q)
	if err != nil {
		return nil, err
	}

	rookAddon, err := NewRookCephAddon(config, client, out, opts, q)
	if err != nil {
		return nil, err
	}

	registry := map[string]Addon{
		"pods":                      logsAddon,
		"cephclusters.ceph.rook.io": rookAddon,
	}

	return registry, nil
}
