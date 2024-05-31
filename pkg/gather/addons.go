// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"net/http"
	"slices"

	"k8s.io/client-go/rest"
)

func createAddons(config *rest.Config, client *http.Client, out *OutputDirectory, opts *Options, q Queuer) (map[string]Addon, error) {
	registry := map[string]Addon{}

	if addonEnabled("logs", opts) {
		addon, err := NewLogsAddon(config, client, out, opts, q)
		if err != nil {
			return nil, err
		}
		registry["pods"] = addon
	}

	if addonEnabled("rook", opts) {
		addon, err := NewRookCephAddon(config, client, out, opts, q)
		if err != nil {
			return nil, err
		}
		registry["ceph.rook.io/cephclusters"] = addon
	}

	return registry, nil
}

func addonEnabled(name string, opts *Options) bool {
	return opts.Addons == nil || slices.Contains(opts.Addons, name)
}
