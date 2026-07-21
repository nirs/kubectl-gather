// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	secretsName = "secrets"
)

type secretsAddon struct {
	addonMeta
	AddonBackend
	log *zap.SugaredLogger
}

func init() {
	registerAddon(secretsName, addonInfo{
		Resources: []string{"secrets"},
		AddonFunc: NewSecretsAddon,
	})
}

func NewSecretsAddon(backend AddonBackend) (Addon, error) {
	return &secretsAddon{
		addonMeta:    addonMeta{name: secretsName},
		AddonBackend: backend,
		log:          backend.Options().Log.Named(secretsName),
	}, nil
}

func (a *secretsAddon) Inspect(item *unstructured.Unstructured, _ *time.Time) error {
	a.log.Debugf("Inspecting secret \"%s/%s\"", item.GetNamespace(), item.GetName())
	sanitizeSecret(item, a.Options().Salt, a.log)
	return nil
}
