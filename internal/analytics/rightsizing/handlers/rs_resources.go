package handlers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// RSConfigMapPredicate returns a predicate that filters ConfigMap watch events
// to only RS ConfigMaps. Delete and generic events are ignored to prevent
// MCOA from reconciling when MCO deletes ConfigMaps during its finalizer cleanup.
func RSConfigMapPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isRSConfigMap(e.Object.GetNamespace(), e.Object.GetName())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isRSConfigMap(e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func isRSConfigMap(namespace, name string) bool {
	if namespace != addoncfg.InstallNamespace {
		return false
	}
	switch name {
	case rightsizing.NamespaceConfigMapName, rightsizing.VirtualizationConfigMapName,
		rightsizing.NamespacePlacementCMName, rightsizing.VirtualizationPlacementCMName:
		return true
	}
	return false
}

// ReconcileRSResources ensures right-sizing ConfigMap resources are cleaned up
// for disabled features.
// Called from ResourceCreator (hub-wide, not per-cluster) to avoid race conditions.
func (o *OptionsBuilder) ReconcileRSResources(ctx context.Context, opts addon.Options) error {
	if !opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled {
		if err := o.deleteRSConfigMap(ctx, rightsizing.NamespaceConfigMapName); err != nil {
			return fmt.Errorf("failed to cleanup namespace configmap: %w", err)
		}
	}

	if !opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled {
		if err := o.deleteRSConfigMap(ctx, rightsizing.VirtualizationConfigMapName); err != nil {
			return fmt.Errorf("failed to cleanup virtualization configmap: %w", err)
		}
	}

	return nil
}

const rsPlacementHashVar = "rsPlacementHash"

// SyncPlacementHash computes a hash of the RS placement ConfigMaps and
// stores it as a CustomizedVariable on the AddOnDeploymentConfig spec.
// Updating the spec (not an annotation) increments the ADC's generation,
// which the addon-config-controller detects and propagates as an MCA
// status update, ultimately triggering the addon-deploy-controller to
// regenerate ManifestWorks.
//
// Cluster label changes are handled separately by the
// AgentDeployTriggerClusterFilter registered on the addon factory.
func (o *OptionsBuilder) SyncPlacementHash(ctx context.Context, aodc *addonv1alpha1.AddOnDeploymentConfig) error {
	hash := o.computePlacementHash(ctx)

	for _, v := range aodc.Spec.CustomizedVariables {
		if v.Name == rsPlacementHashVar && v.Value == hash {
			return nil
		}
	}

	found := false
	for i, v := range aodc.Spec.CustomizedVariables {
		if v.Name == rsPlacementHashVar {
			aodc.Spec.CustomizedVariables[i].Value = hash
			found = true
			break
		}
	}
	if !found {
		aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables,
			addonv1alpha1.CustomizedVariable{Name: rsPlacementHashVar, Value: hash})
	}

	if err := o.Client.Update(ctx, aodc); err != nil {
		return fmt.Errorf("failed to update ADC with placement hash: %w", err)
	}
	o.Logger.Info("Updated RS placement hash on ADC spec, addon-deploy-controller will re-sync",
		"hash", hash)
	return nil
}

// computePlacementHash computes a hash of the RS placement ConfigMap contents.
// Only ConfigMap data is hashed; cluster label changes are handled by the
// AgentDeployTriggerClusterFilter registered on the addon factory.
func (o *OptionsBuilder) computePlacementHash(ctx context.Context) string {
	h := sha256.New()

	for _, cmName := range []string{
		rightsizing.NamespacePlacementCMName,
		rightsizing.VirtualizationPlacementCMName,
	} {
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{Name: cmName, Namespace: addoncfg.InstallNamespace}
		if err := o.Client.Get(ctx, key, cm); err != nil {
			_, _ = fmt.Fprintf(h, "%s=absent;", cmName)
			continue
		}
		keys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			_, _ = fmt.Fprintf(h, "%s/%s=%s;", cmName, k, cm.Data[k])
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// deleteRSConfigMap deletes a right-sizing ConfigMap resource if it exists.
func (o *OptionsBuilder) deleteRSConfigMap(ctx context.Context, configMapName string) error {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{Name: configMapName, Namespace: addoncfg.InstallNamespace}
	if err := o.Client.Get(ctx, key, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get configmap %s: %w", configMapName, err)
	}
	if err := o.Client.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete configmap %s: %w", configMapName, err)
	}
	o.Logger.V(1).Info("Deleted right-sizing ConfigMap", "name", configMapName, "namespace", addoncfg.InstallNamespace)
	return nil
}
