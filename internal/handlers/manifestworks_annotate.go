package handlers

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/go-logr/logr"
	"github.com/mitchellh/hashstructure"
	lhandlers "github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const manifestworkConfigHash = "mcoa.openshift.io/config-hash"

func UpdateAnnotationOnManifestWorks(ctx context.Context, log logr.Logger, req ctrl.Request, k client.Client) error {
	ll := log.WithValues("managedclusteraddon", req.NamespacedName, "event", "updateAnnotation")

	var addon addonapiv1alpha1.ManagedClusterAddOn
	if err := k.Get(ctx, req.NamespacedName, &addon); err != nil {
		if apierrors.IsNotFound(err) {
			// maybe the user deleted it before we could react? Either way this isn't an issue
			ll.Error(err, "could not find the requested managedclusteraddon", "name", req.NamespacedName)
			return nil
		}
		return kverrors.Wrap(err, "failed to lookup managedclusteraddon", "name", req.NamespacedName)
	}

	var manifestworkList workv1.ManifestWorkList
	if err := k.List(ctx, &manifestworkList, &client.ListOptions{
		Namespace: req.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"open-cluster-management.io/addon-name": "multicluster-observability-addon",
		}),
	}); err != nil {
		return kverrors.Wrap(err, "failed to list manifestwork", "name", req.NamespacedName)
	}

	if len(manifestworkList.Items) == 0 {
		ll.Info("could not find the matching manifestwork", "name", req.NamespacedName)
		return nil
	}

	opts, err := lhandlers.BuildOptions(k, &addon, nil)
	if err != nil {
		ll.Error(err, "failed to buildOptions managedclusteraddon")
		return kverrors.Wrap(err, "failed to buildOptions managedclusteraddon", "name", req.NamespacedName)
	}

	hash, err := hashstructure.Hash(opts, &hashstructure.HashOptions{})
	if err != nil {
		return kverrors.Wrap(err, "failed to compute options hash", "name", req.NamespacedName)
	}

	var objects []client.Object
	for _, manifestwork := range manifestworkList.Items {
		manifestwork.Annotations[manifestworkConfigHash] = fmt.Sprint(hash)
		objects = append(objects, &manifestwork)
	}

	for _, obj := range objects {
		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := manifests.MutateFuncFor(obj, desired, nil)

		op, err := ctrl.CreateOrUpdate(ctx, k, obj, mutateFn)
		if err != nil {
			klog.Error(err, "failed to configure resource")
			continue
		}

		msg := fmt.Sprintf("Resource has been %s", op)
		switch op {
		case ctrlutil.OperationResultNone:
			klog.Info(msg)
		default:
			klog.Info(msg)
		}
	}

	return nil
}
