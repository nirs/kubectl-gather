// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ramenName             = "ramen"
	clusterTimeAnnotation = "kubectl-gather.nirs.github.com/cluster-time"
)

type ramenAddon struct {
	addonMeta
	AddonBackend
	log *zap.SugaredLogger
}

func init() {
	registerAddon(ramenName, addonInfo{
		Resources: []string{
			"ramendr.openshift.io/drplacementcontrols",
			"ramendr.openshift.io/volumereplicationgroups",
		},
		AddonFunc: NewRamenAddon,
	})
}

func NewRamenAddon(backend AddonBackend) (Addon, error) {
	return &ramenAddon{
		addonMeta:    addonMeta{name: ramenName},
		AddonBackend: backend,
		log:          backend.Options().Log.Named(ramenName),
	}, nil
}

func (a *ramenAddon) Inspect(item *unstructured.Unstructured, clusterTime *time.Time) error {
	kind := item.GetKind()
	a.log.Debugf("Inspecting %s \"%s/%s\"", kind, item.GetNamespace(), item.GetName())

	if clusterTime != nil {
		a.annotateClusterTime(item, clusterTime)
	}

	if kind == "DRPlacementControl" {
		a.gatherDRPolicy(item)
	}

	return nil
}

func (a *ramenAddon) annotateClusterTime(item *unstructured.Unstructured, clusterTime *time.Time) {
	annotations := item.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[clusterTimeAnnotation] = clusterTime.UTC().Format(time.RFC3339)
	item.SetAnnotations(annotations)
}

func (a *ramenAddon) gatherDRPolicy(item *unstructured.Unstructured) {
	ref, found, err := unstructured.NestedMap(item.Object, "spec", "drPolicyRef")
	if err != nil {
		a.log.Warnf("Cannot get drpc \"%s/%s\" drPolicyRef: %s",
			item.GetNamespace(), item.GetName(), err)
		return
	}
	if !found {
		a.log.Warnf("Missing drPolicyRef in drpc \"%s/%s\"",
			item.GetNamespace(), item.GetName())
		return
	}

	apiVersion, _ := ref["apiVersion"].(string)
	name, _ := ref["name"].(string)
	if apiVersion == "" || name == "" {
		a.log.Warnf("Invalid drPolicyRef in drpc \"%s/%s\"",
			item.GetNamespace(), item.GetName())
		return
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		a.log.Warnf("Cannot parse drPolicyRef apiVersion %q: %s", apiVersion, err)
		return
	}

	gvr := gv.WithResource("drpolicies")
	a.GatherResource(gvr, types.NamespacedName{Name: name})
}
