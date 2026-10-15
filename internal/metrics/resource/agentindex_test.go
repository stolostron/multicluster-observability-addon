package resource

import (
	"testing"

	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

func TestAgentIndex(t *testing.T) {
	cmaoUID := types.UID("cmao-uid-1")
	newCMAOForIndex := func(placements ...addonv1beta1.PlacementStrategy) *addonv1beta1.ClusterManagementAddOn {
		return &addonv1beta1.ClusterManagementAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name: addoncfg.Name,
				UID:  cmaoUID,
			},
			Spec: addonv1beta1.ClusterManagementAddOnSpec{
				InstallStrategy: addonv1beta1.InstallStrategy{
					Placements: placements,
				},
			},
		}
	}

	cmaoOwnerRef := metav1.OwnerReference{
		APIVersion:         addonv1beta1.GroupVersion.String(),
		Kind:               "ClusterManagementAddOn",
		Name:               addoncfg.Name,
		UID:                cmaoUID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}

	cmaoManagedFields := []metav1.ManagedFieldsEntry{
		{
			Manager:    addoncfg.Name,
			Operation:  metav1.ManagedFieldsOperationApply,
			APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.String(),
		},
	}
	userModifiedManagedFields := []metav1.ManagedFieldsEntry{
		{
			Manager:    addoncfg.Name,
			Operation:  metav1.ManagedFieldsOperationApply,
			APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.String(),
		},
		{
			Manager:    "kubectl-patch",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.String(),
		},
	}

	globalRef := addonv1beta1.PlacementRef{Namespace: config.HubInstallNamespace, Name: "global"}
	customRef := addonv1beta1.PlacementRef{Namespace: config.HubInstallNamespace, Name: "custom"}

	promAgentConfig := func(name string) addonv1beta1.AddOnConfig {
		return addonv1beta1.AddOnConfig{
			ConfigGroupResource: addonv1beta1.ConfigGroupResource{
				Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
				Resource: cooprometheusv1alpha1.PrometheusAgentName,
			},
			ConfigReferent: addonv1beta1.ConfigReferent{
				Namespace: config.HubInstallNamespace,
				Name:      name,
			},
		}
	}

	t.Run("classifies CMAO-owned, user-owned, unrecognized, and other-component agents", func(t *testing.T) {
		cmaoDefaultGlobal := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "mcoa-default-platform-metrics-collector-global",
				Namespace:       config.HubInstallNamespace,
				Labels:          map[string]string{addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp},
				Annotations:     map[string]string{addoncfg.PlacementAnnotationKey: globalRef.Namespace + "/" + globalRef.Name},
				OwnerReferences: []metav1.OwnerReference{cmaoOwnerRef},
				ManagedFields:   cmaoManagedFields,
			},
		}
		userModifiedDefault := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "mcoa-default-platform-metrics-collector-custom",
				Namespace:       config.HubInstallNamespace,
				Labels:          map[string]string{addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp},
				Annotations:     map[string]string{addoncfg.PlacementAnnotationKey: customRef.Namespace + "/" + customRef.Name},
				OwnerReferences: []metav1.OwnerReference{cmaoOwnerRef},
				ManagedFields:   userModifiedManagedFields,
			},
		}
		userUnrecognized := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "user-agent-without-part-of",
				Namespace:   config.HubInstallNamespace,
				Labels:      map[string]string{addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp},
				Annotations: map[string]string{addoncfg.PlacementAnnotationKey: globalRef.Namespace + "/" + globalRef.Name},
			},
		}
		uwlAgent := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uwl-custom-agent",
				Namespace: config.HubInstallNamespace,
				Labels: map[string]string{
					addoncfg.ComponentK8sLabelKey: config.UserWorkloadMetricsCollectorApp,
					addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
				},
			},
		}
		unrelatedAgent := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "third-party-agent",
				Namespace: config.HubInstallNamespace,
				Labels:    map[string]string{addoncfg.ComponentK8sLabelKey: "something-else"},
			},
		}

		idx, err := newAgentIndex(newCMAOForIndex(), []cooprometheusv1alpha1.PrometheusAgent{
			cmaoDefaultGlobal,
			userModifiedDefault,
			userUnrecognized,
			uwlAgent,
			unrelatedAgent,
		}, false)
		require.NoError(t, err)

		assert.Equal(t, config.PlatformMetricsCollectorApp, idx.appName)
		assert.Contains(t, idx.cmaoOwnedAgentsByName, cmaoDefaultGlobal.Name)
		assert.Len(t, idx.cmaoOwnedAgentsForPlacement(globalRef), 1)
		assert.False(t, idx.isPlacementCoveredByUser(globalRef))
		assert.True(t, idx.isPlacementCoveredByUser(customRef))
		assert.NotContains(t, idx.cmaoOwnedAgentsByName, unrelatedAgent.Name)
	})

	t.Run("cmaoOwnedAgentsForPlacement indexes CMAO-owned agents only via placement annotation", func(t *testing.T) {
		annotatedAgent := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cmao-annotated",
				Namespace:       config.HubInstallNamespace,
				Labels:          map[string]string{addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp},
				Annotations:     map[string]string{addoncfg.PlacementAnnotationKey: globalRef.Namespace + "/" + globalRef.Name},
				OwnerReferences: []metav1.OwnerReference{cmaoOwnerRef},
				ManagedFields:   cmaoManagedFields,
			},
		}
		configOnlyAgent := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cmao-in-configs-without-annotation",
				Namespace:       config.HubInstallNamespace,
				Labels:          map[string]string{addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp},
				OwnerReferences: []metav1.OwnerReference{cmaoOwnerRef},
				ManagedFields:   cmaoManagedFields,
			},
		}

		placement := addonv1beta1.PlacementStrategy{
			PlacementRef: globalRef,
			Configs: []addonv1beta1.AddOnConfig{
				promAgentConfig(annotatedAgent.Name),
				promAgentConfig(configOnlyAgent.Name),
			},
		}

		idx, err := newAgentIndex(newCMAOForIndex(placement), []cooprometheusv1alpha1.PrometheusAgent{
			annotatedAgent,
			configOnlyAgent,
		}, false)
		require.NoError(t, err)

		got := idx.cmaoOwnedAgentsForPlacement(globalRef)
		require.Len(t, got, 1)
		assert.Equal(t, annotatedAgent.Name, got[0].Name)
	})

	t.Run("isPlacementCoveredByUser handles annotations, placement.Configs, other-component, and stale entries", func(t *testing.T) {
		cmaoAgent := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "mcoa-default-platform-metrics-collector-global",
				Namespace:       config.HubInstallNamespace,
				Labels:          map[string]string{addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp},
				OwnerReferences: []metav1.OwnerReference{cmaoOwnerRef},
				ManagedFields:   cmaoManagedFields,
			},
		}
		userAgentInCluster := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-platform-agent",
				Namespace: config.HubInstallNamespace,
				Labels: map[string]string{
					addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp,
					addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
				},
			},
		}
		unlabeledUserAgentInConfigs := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unlabeled-user-platform-agent",
				Namespace: config.HubInstallNamespace,
				Labels: map[string]string{
					addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp,
				},
			},
		}
		unlabeledUwlAgentInConfigs := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unlabeled-user-uwl-agent",
				Namespace: config.HubInstallNamespace,
				Labels: map[string]string{
					addoncfg.ComponentK8sLabelKey: config.UserWorkloadMetricsCollectorApp,
				},
			},
		}

		ref1 := addonv1beta1.PlacementRef{Namespace: config.HubInstallNamespace, Name: "p1"}
		ref2 := addonv1beta1.PlacementRef{Namespace: config.HubInstallNamespace, Name: "p2"}
		ref3 := addonv1beta1.PlacementRef{Namespace: config.HubInstallNamespace, Name: "p3"}
		ref4 := addonv1beta1.PlacementRef{Namespace: config.HubInstallNamespace, Name: "p4"}
		ref5 := addonv1beta1.PlacementRef{Namespace: config.HubInstallNamespace, Name: "p5"}

		cmao := newCMAOForIndex(
			// 1. Existing labeled user agent referenced in placement.Configs -> covered
			addonv1beta1.PlacementStrategy{
				PlacementRef: ref1,
				Configs:      []addonv1beta1.AddOnConfig{promAgentConfig(userAgentInCluster.Name)},
			},
			// 2. Existing user agent without part-of label referenced in placement.Configs -> covered
			addonv1beta1.PlacementStrategy{
				PlacementRef: ref2,
				Configs:      []addonv1beta1.AddOnConfig{promAgentConfig(unlabeledUserAgentInConfigs.Name)},
			},
			// 3. Existing CMAO agent referenced in placement.Configs -> not covered by user
			addonv1beta1.PlacementStrategy{
				PlacementRef: ref3,
				Configs:      []addonv1beta1.AddOnConfig{promAgentConfig(cmaoAgent.Name)},
			},
			// 4. Other component (UWL) agent without part-of label referenced in placement.Configs -> not covered for platform
			addonv1beta1.PlacementStrategy{
				PlacementRef: ref4,
				Configs:      []addonv1beta1.AddOnConfig{promAgentConfig(unlabeledUwlAgentInConfigs.Name)},
			},
			// 5. Non-existent/stale agent entry in placement.Configs -> ignored (not covered by user)
			addonv1beta1.PlacementStrategy{
				PlacementRef: ref5,
				Configs:      []addonv1beta1.AddOnConfig{promAgentConfig("missing-or-stale-agent")},
			},
		)

		idx, err := newAgentIndex(cmao, []cooprometheusv1alpha1.PrometheusAgent{
			cmaoAgent,
			userAgentInCluster,
			unlabeledUserAgentInConfigs,
			unlabeledUwlAgentInConfigs,
		}, false)
		require.NoError(t, err)

		assert.True(t, idx.isPlacementCoveredByUser(ref1))
		assert.True(t, idx.isPlacementCoveredByUser(ref2))
		assert.False(t, idx.isPlacementCoveredByUser(ref3))
		assert.False(t, idx.isPlacementCoveredByUser(ref4))
		assert.False(t, idx.isPlacementCoveredByUser(ref5))
	})

	t.Run("returns error when recognized agent has malformed placement annotation", func(t *testing.T) {
		malformedAgent := cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-agent-bad-annotation",
				Namespace: config.HubInstallNamespace,
				Labels: map[string]string{
					addoncfg.ComponentK8sLabelKey: config.PlatformMetricsCollectorApp,
					addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
				},
				Annotations: map[string]string{
					addoncfg.PlacementAnnotationKey: "open-cluster-management-observability/global, malformed-entry",
				},
			},
		}

		_, err := newAgentIndex(newCMAOForIndex(), []cooprometheusv1alpha1.PrometheusAgent{malformedAgent}, false)
		require.ErrorIs(t, err, errInvalidPlacementReference)
	})
}
