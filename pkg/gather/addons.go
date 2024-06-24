// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"net/http"
	"slices"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

type AddonBackend interface {
	// Config returns the rest config for this cluster that can be used to
	// create a new client.
	Config() *rest.Config

	// HTTPClient returns the http client connected to the cluster. It can be
	// used to create a new client sharing the same http client.
	HTTPClient() *http.Client

	// Output returns the output for this gathering.
	Output() *OutputDirectory

	// Options returns gathering options for this cluster.
	Options() *Options

	// Queue function on the work queue.
	Queue(WorkFunc)

	// GatherResource gathers the specified resource asynchronically.
	GatherResource(schema.GroupVersionResource, types.NamespacedName)
}

type addonFunc func(AddonBackend) (Addon, error)

type addonInfo struct {
	Resource  string
	AddonFunc addonFunc
}

var addonRegistry = map[string]addonInfo{}

func registerAddon(name string, ai addonInfo) {
	addonRegistry[name] = ai
}

func createAddons(backend AddonBackend) (map[string]Addon, error) {
	registry := map[string]Addon{}

	for name, addonInfo := range addonRegistry {
		if addonEnabled(name, backend.Options()) {
			addon, err := addonInfo.AddonFunc(backend)
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
