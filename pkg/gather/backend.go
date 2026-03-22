// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"context"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

type gatherBackend struct {
	g *Gatherer
}

func (b *gatherBackend) Context() context.Context {
	return b.g.ctx
}

func (b *gatherBackend) Config() *rest.Config {
	return b.g.config
}

func (b *gatherBackend) HTTPClient() *http.Client {
	return b.g.httpClient
}

func (b *gatherBackend) Options() *Options {
	return b.g.opts
}

func (b *gatherBackend) Output() *OutputDirectory {
	return &b.g.output
}

func (b *gatherBackend) Queue(work WorkFunc) {
	b.g.inspectQueue.Queue(work)
}

func (b *gatherBackend) GatherResource(gvr schema.GroupVersionResource, name types.NamespacedName) {
	b.g.gatherResource(gvr, name)
}
