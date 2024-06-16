// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"net/http"
	"slices"

	"k8s.io/client-go/rest"
)

type addonFunc func(*rest.Config, *http.Client, *OutputDirectory, *Options, Queuer) (Addon, error)

type addonInfo struct {
	Resource  string
	AddonFunc addonFunc
}

var addonRegistry map[string]addonInfo

func init() {
	addonRegistry = map[string]addonInfo{}

	addonRegistry["logs"] = addonInfo{
		Resource:  "pods",
		AddonFunc: NewLogsAddon,
	}

	addonRegistry["rook"] = addonInfo{
		Resource:  "ceph.rook.io/cephclusters",
		AddonFunc: NewRookCephAddon,
	}
}

func createAddons(config *rest.Config, client *http.Client, out *OutputDirectory, opts *Options, q Queuer) (map[string]Addon, error) {
	registry := map[string]Addon{}

	for name, addonInfo := range addonRegistry {
		if addonEnabled(name, opts) {
			addon, err := addonInfo.AddonFunc(config, client, out, opts, q)
			if err != nil {
				return nil, err
			}
			registry[addonInfo.Resource] = addon
		}
	}

	return registry, nil
}

func addonEnabled(name string, opts *Options) bool {
	return opts.Addons == nil || slices.Contains(opts.Addons, name)
}

func AvailableAddons() []string {
	addonNames := make([]string, 0, len(addonRegistry))
	for name := range addonRegistry {
		addonNames = append(addonNames, name)
	}
	return addonNames
}
