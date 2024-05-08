package handlers

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/go-logr/logr"
	"github.com/mitchellh/hashstructure"
	addonhelm "github.com/rhobs/multicluster-observability-addon/internal/addon/helm"
	lhandlers "github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	loggingmanifests "github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	thandlers "github.com/rhobs/multicluster-observability-addon/internal/tracing/handlers"
	tracingmanifests "github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
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

	hash, err := getHash(ctx, ll, req, k)
	if err != nil {
		return err
	}

	var objects []client.Object
	for _, manifestwork := range manifestworkList.Items {
		manifestwork.Annotations[manifestworkConfigHash] = hash
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

// getHash computes the hash based on the enabled signals and the objects to reconcile
func getHash(ctx context.Context, log logr.Logger, req ctrl.Request, k client.Client) (string, error) {
	var addon addonapiv1alpha1.ManagedClusterAddOn
	if err := k.Get(ctx, req.NamespacedName, &addon); err != nil {
		if apierrors.IsNotFound(err) {
			// maybe the user deleted it before we could react? Either way this isn't an issue
			log.Error(err, "could not find the requested managedclusteraddon", "name", req.NamespacedName)
			return "", nil
		}
		return "", kverrors.Wrap(err, "failed to lookup managedclusteraddon", "name", req.NamespacedName)
	}

	aodc, err := addonhelm.GetAddOnDeploymentConfig(k, &addon)
	if err != nil {
		return "", kverrors.Wrap(err, " error getting the addon addondeploymentconfig")
	}
	opts, err := addonhelm.BuildOptions(aodc)
	if err != nil {
		return "", kverrors.Wrap(err, " error getting the building options")
	}

	lOpts := loggingmanifests.Options{}
	if !opts.LoggingDisabled {
		lOpts, err = lhandlers.BuildOptions(k, &addon, nil)
		if err != nil {
			return "", kverrors.Wrap(err, "failed to buildOptions managedclusteraddon", "name", req.NamespacedName, "signal", "logging")
		}
	}

	tOpts := tracingmanifests.Options{}
	if !opts.TracingDisabled {
		tOpts, err = thandlers.BuildOptions(k, &addon, nil)
		if err != nil {
			return "", kverrors.Wrap(err, "failed to buildOptions managedclusteraddon", "name", req.NamespacedName, "signal", "tracing")
		}
	}

	addonParameters := struct {
		loggingOps  loggingmanifests.Options
		tracingOpts tracingmanifests.Options
	}{
		loggingOps:  lOpts,
		tracingOpts: tOpts,
	}

	hash, err := hashstructure.Hash(addonParameters, &hashstructure.HashOptions{})
	if err != nil {
		return "", kverrors.Wrap(err, "failed to compute options hash", "name", req.NamespacedName)
	}

	return fmt.Sprint(hash), nil
}
