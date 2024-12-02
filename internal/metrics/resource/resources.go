package resource

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"github.com/go-logr/logr"
	prometheus "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	mu                 sync.Mutex
	initialized        bool
	ErrUnsupportedType = fmt.Errorf("unsupported type")
)

func DeployDefaultResourcesOnce(ctx context.Context, c client.Client, logger logr.Logger, ns string) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return nil
	}

	logger.Info("deploying default monitoring resources")

	// Get clusterManagementAddon resource to use as owner
	owner := &addonapiv1alpha1.ClusterManagementAddOn{}
	if err := c.Get(ctx, types.NamespacedName{Name: addon.Name, Namespace: ns}, owner); err != nil {
		return err
	}

	// Deploy default resources
	resources := DefaultPlaftformAgentResources(ns)
	resources = append(resources, DefaultUserWorkloadAgentResources(ns)...)
	for _, resource := range resources {
		if err := controllerutil.SetControllerReference(owner, resource, c.Scheme()); err != nil {
			return err
		}

		res, err := ctrl.CreateOrUpdate(ctx, c, resource, mutateFn(resource.DeepCopyObject().(client.Object), resource))
		if err != nil {
			return fmt.Errorf("failed to create or update resource %s: %w", resource.GetName(), err)
		}
		if res != controllerutil.OperationResultNone {
			logger.Info("resource created or updated", "kind", resource.DeepCopyObject().GetObjectKind().GroupVersionKind().Kind, "name", resource.GetName(), "action", res)
		}
	}

	initialized = true

	return nil
}

func mutateFn(want, existing client.Object) controllerutil.MutateFn {
	return func() error {
		existingLabels := existing.GetLabels()
		maps.Copy(existingLabels, want.GetLabels())
		existing.SetLabels(existingLabels)

		existingAnnotations := existing.GetAnnotations()
		maps.Copy(existingAnnotations, want.GetAnnotations())
		existing.SetAnnotations(existingAnnotations)

		existing.SetOwnerReferences(want.GetOwnerReferences())

		switch existing.(type) {
		case *prometheusalpha1.PrometheusAgent:
			existing.(*prometheusalpha1.PrometheusAgent).Spec = want.(*prometheusalpha1.PrometheusAgent).Spec
		case *prometheus.PrometheusRule:
			existing.(*prometheus.PrometheusRule).Spec = want.(*prometheus.PrometheusRule).Spec
		case *prometheusalpha1.ScrapeConfig:
			existing.(*prometheusalpha1.ScrapeConfig).Spec = want.(*prometheusalpha1.ScrapeConfig).Spec
		case *corev1.ConfigMap:
			existing.(*corev1.ConfigMap).Data = want.(*corev1.ConfigMap).Data
		default:
			return fmt.Errorf("%w: %T", ErrUnsupportedType, existing)
		}

		return nil
	}
}
