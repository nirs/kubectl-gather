// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

const (
	pvcsName = "pvcs"
)

type pvcsAddon struct {
	AddonBackend
	log *zap.SugaredLogger
}

func init() {
	registerAddon(pvcsName, addonInfo{
		Resource:  "persistentvolumeclaims",
		AddonFunc: NewPVCAddon,
	})
}

func NewPVCAddon(backend AddonBackend) (Addon, error) {
	return &pvcsAddon{
		AddonBackend: backend,
		log:          backend.Options().Log.Named(pvcsName),
	}, nil
}

func (a *pvcsAddon) Inspect(pvc *unstructured.Unstructured) error {
	// Needed only when gathering specific namespaces.
	if len(a.Options().Namespaces) == 0 {
		return nil
	}

	a.log.Debugf("Inspecting pvc \"%s/%s\"", pvc.GetNamespace(), pvc.GetName())

	a.gatherPersistentVolume(pvc)
	a.gatherStorageClass(pvc)

	return nil
}

func (a *pvcsAddon) gatherPersistentVolume(pvc *unstructured.Unstructured) {
	name, found, err := unstructured.NestedString(pvc.Object, "spec", "volumeName")
	if err != nil {
		a.log.Warnf("Cannot get pvc \"%s/%s\" volumeName: %s",
			pvc.GetNamespace(), pvc.GetName(), err)
		return
	}

	if name == "" || !found {
		return
	}

	gvr := corev1.SchemeGroupVersion.WithResource("persistentvolumes")
	a.GatherResource(gvr, types.NamespacedName{Name: name})
}

func (a *pvcsAddon) gatherStorageClass(pvc *unstructured.Unstructured) {
	name, found, err := unstructured.NestedString(pvc.Object, "spec", "storageClassName")
	if err != nil {
		a.log.Warnf("Cannot get pvc \"%s/%s\" storageClassName: %s",
			pvc.GetNamespace(), pvc.GetName(), err)
		return
	}
	if name == "" || !found {
		// TODO: Get the default storage class?
		return
	}

	gvr := storagev1.SchemeGroupVersion.WithResource("storageclasses")
	a.GatherResource(gvr, types.NamespacedName{Name: name})
}
