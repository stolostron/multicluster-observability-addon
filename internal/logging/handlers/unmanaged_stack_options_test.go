package handlers

import (
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// buildCLFConfigReference returns a ManagedClusterAddOn ConfigReference pointing at
// the given ClusterLogForwarder key, as the addon-framework would populate it in
// Status.ConfigReferences once the CMAO's placement Configs are resolved.
func buildCLFConfigReference(namespace, name string) addonapiv1beta1.ConfigReference {
	return addonapiv1beta1.ConfigReference{
		ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
			Group:    loggingv1.GroupVersion.Group,
			Resource: addoncfg.ClusterLogForwardersResource,
		},
		DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
			ConfigReferent: addonapiv1beta1.ConfigReferent{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func TestGetUnmanagedClusterLogForwarder(t *testing.T) {
	ctx := t.Context()

	t.Run("returns the sole non-owned CLF", func(t *testing.T) {
		scheme := buildTestScheme(t)
		clf := &loggingv1.ClusterLogForwarder{
			ObjectMeta: metav1.ObjectMeta{Name: "user-clf", Namespace: "user-ns"},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clf).Build()

		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			Status: addonapiv1beta1.ManagedClusterAddOnStatus{
				ConfigReferences: []addonapiv1beta1.ConfigReference{
					buildCLFConfigReference("user-ns", "user-clf"),
				},
			},
		}

		got, err := getUnmanagedClusterLogForwarder(ctx, fakeClient, mcAddon)
		require.NoError(t, err)
		assert.Equal(t, "user-clf", got.Name)
	})

	t.Run("skips MCOA's own CMAO-owned CLF and errors as missing", func(t *testing.T) {
		scheme := buildTestScheme(t)
		cmao := buildTestCMAO(globalPlacement())
		managedCLF := &loggingv1.ClusterLogForwarder{
			ObjectMeta: metav1.ObjectMeta{Name: "mcoa-default-global", Namespace: addoncfg.InstallNamespace},
		}
		require.NoError(t, controllerutil.SetControllerReference(cmao, managedCLF, scheme))
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao, managedCLF).Build()

		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			Status: addonapiv1beta1.ManagedClusterAddOnStatus{
				ConfigReferences: []addonapiv1beta1.ConfigReference{
					buildCLFConfigReference(addoncfg.InstallNamespace, "mcoa-default-global"),
				},
			},
		}

		_, err := getUnmanagedClusterLogForwarder(ctx, fakeClient, mcAddon)
		require.Error(t, err)
		require.ErrorIs(t, err, errMissingCLFRef, "the only reference is MCOA's own managed CLF, so it should be treated as if no unmanaged CLF was referenced at all")
	})

	t.Run("picks the real unmanaged CLF even when MCOA's managed CLF is also referenced", func(t *testing.T) {
		scheme := buildTestScheme(t)
		cmao := buildTestCMAO(globalPlacement())
		managedCLF := &loggingv1.ClusterLogForwarder{
			ObjectMeta: metav1.ObjectMeta{Name: "mcoa-default-global", Namespace: addoncfg.InstallNamespace},
		}
		require.NoError(t, controllerutil.SetControllerReference(cmao, managedCLF, scheme))
		userCLF := &loggingv1.ClusterLogForwarder{
			ObjectMeta: metav1.ObjectMeta{Name: "user-clf", Namespace: "user-ns"},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao, managedCLF, userCLF).Build()

		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			Status: addonapiv1beta1.ManagedClusterAddOnStatus{
				ConfigReferences: []addonapiv1beta1.ConfigReference{
					buildCLFConfigReference(addoncfg.InstallNamespace, "mcoa-default-global"),
					buildCLFConfigReference("user-ns", "user-clf"),
				},
			},
		}

		got, err := getUnmanagedClusterLogForwarder(ctx, fakeClient, mcAddon)
		require.NoError(t, err)
		assert.Equal(t, "user-clf", got.Name, "must pick the admin-authored CLF, not MCOA's own managed one")
	})

	t.Run("errors when multiple non-owned CLFs are referenced", func(t *testing.T) {
		scheme := buildTestScheme(t)
		clf1 := &loggingv1.ClusterLogForwarder{ObjectMeta: metav1.ObjectMeta{Name: "clf-1", Namespace: "user-ns"}}
		clf2 := &loggingv1.ClusterLogForwarder{ObjectMeta: metav1.ObjectMeta{Name: "clf-2", Namespace: "user-ns"}}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clf1, clf2).Build()

		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			Status: addonapiv1beta1.ManagedClusterAddOnStatus{
				ConfigReferences: []addonapiv1beta1.ConfigReference{
					buildCLFConfigReference("user-ns", "clf-1"),
					buildCLFConfigReference("user-ns", "clf-2"),
				},
			},
		}

		_, err := getUnmanagedClusterLogForwarder(ctx, fakeClient, mcAddon)
		require.Error(t, err)
		require.ErrorIs(t, err, errMultipleCLFRef)
	})

	t.Run("errors when no CLF is referenced", func(t *testing.T) {
		scheme := buildTestScheme(t)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{}

		_, err := getUnmanagedClusterLogForwarder(ctx, fakeClient, mcAddon)
		require.Error(t, err)
		require.ErrorIs(t, err, errMissingCLFRef)
	})
}
