package common

import (
	"context"
	"fmt"
	"maps"

	"github.com/go-logr/logr"
	prometheus "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	errUnsupportedType           = fmt.Errorf("unsupported type")
	errFailedToSetOwnerReference = fmt.Errorf("failed to set owner reference")
)

func CreateOrUpdateWithAddOnOwner(ctx context.Context, logger logr.Logger, k8s client.Client, objs []client.Object) error {
	// ClusterManagementAddOn as owner
	owner := &addonapiv1alpha1.ClusterManagementAddOn{}
	if err := k8s.Get(ctx, types.NamespacedName{Name: addon.Name, Namespace: addon.InstallNamespace}, owner); err != nil {
		return err
	}

	for _, obj := range objs {
		// Set owner reference
		if err := controllerutil.SetControllerReference(owner, obj, k8s.Scheme()); err != nil {
			return fmt.Errorf("%w: %s", errFailedToSetOwnerReference, err.Error())
		}

		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := mutateFuncFor(obj, desired)
		res, err := ctrl.CreateOrUpdate(ctx, k8s, obj, mutateFn)
		if err != nil {
			return fmt.Errorf("failed to create or update resource %s: %w", obj.GetName(), err)
		}
		if res != controllerutil.OperationResultNone {
			logger.Info("resource created or updated", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName(), "action", res)
		}
	}

	return nil
}

func mutateFuncFor(want, existing client.Object) controllerutil.MutateFn {
	return func() error {
		maps.Copy(existing.GetLabels(), want.GetLabels())
		maps.Copy(existing.GetAnnotations(), want.GetAnnotations())

		switch existingTyped := existing.(type) {
		case *prometheusalpha1.PrometheusAgent:
			existingTyped.Spec = want.(*prometheusalpha1.PrometheusAgent).Spec
		case *prometheus.PrometheusRule:
			existingTyped.Spec = want.(*prometheus.PrometheusRule).Spec
		case *prometheusalpha1.ScrapeConfig:
			existingTyped.Spec = want.(*prometheusalpha1.ScrapeConfig).Spec
		case *corev1.ConfigMap:
			existingTyped.Data = want.(*corev1.ConfigMap).Data
		default:
			return fmt.Errorf("%w: %T", errUnsupportedType, existing)
		}

		return nil
	}
}
