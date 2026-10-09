package common

import (
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func controllerRefTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))
	return scheme
}

// clfRef mirrors how the addon-framework populates Status.ConfigReferences from
// the ClusterManagementAddOn's placement configs. The reference is recorded
// whether or not the object behind it exists: the framework's addonconfig
// controller skips the spec hash on NotFound but keeps the reference.
func clfRef(namespace, name string) addonapiv1beta1.ConfigReference {
	return addonapiv1beta1.ConfigReference{
		ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
			Group:    loggingv1.GroupVersion.Group,
			Resource: addoncfg.ClusterLogForwardersResource,
		},
		DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
			ConfigReferent: addonapiv1beta1.ConfigReferent{Namespace: namespace, Name: name},
		},
	}
}

func TestGetResourceWithOwnerRef(t *testing.T) {
	ctx := t.Context()

	ownedCLF := func(t *testing.T, scheme *runtime.Scheme, name string) *loggingv1.ClusterLogForwarder {
		t.Helper()
		clf := &loggingv1.ClusterLogForwarder{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: addoncfg.InstallNamespace},
		}
		require.NoError(t, controllerutil.SetControllerReference(cmaoOwnerStub(), clf, scheme))
		return clf
	}

	get := func(fakeClient client.Client, refs ...addonapiv1beta1.ConfigReference) (*loggingv1.ClusterLogForwarder, error) {
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "cluster-a"},
			Status:     addonapiv1beta1.ManagedClusterAddOnStatus{ConfigReferences: refs},
		}
		return GetResourceWithOwnerRef(ctx, fakeClient, mcAddon,
			loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource, &loggingv1.ClusterLogForwarder{})
	}

	// The ClusterManagementAddOn ships with clusterlogforwarders/instance
	// hardcoded on the global placement as the slot an admin fills in for
	// unmanaged collection. Most hubs never create it, and it is listed before
	// the default-stack CLF, so aborting on the first NotFound would make
	// enabling platformLogsDefault fail to render any logging values.
	t.Run("skips an empty slot listed before the owned resource", func(t *testing.T) {
		scheme := controllerRefTestScheme(t)
		managed := ownedCLF(t, scheme, "default-stack-instance-global")
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(managed).Build()

		got, err := get(fakeClient,
			clfRef(addoncfg.InstallNamespace, "instance"), // never created
			clfRef(addoncfg.InstallNamespace, "default-stack-instance-global"),
		)
		require.NoError(t, err)
		assert.Equal(t, "default-stack-instance-global", got.Name)
	})

	t.Run("skips a reference that outlived its object", func(t *testing.T) {
		scheme := controllerRefTestScheme(t)
		managed := ownedCLF(t, scheme, "default-stack-instance-global")
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(managed).Build()

		got, err := get(fakeClient,
			clfRef(addoncfg.InstallNamespace, "default-stack-instance-global"),
			clfRef(addoncfg.InstallNamespace, "deleted-clf"),
		)
		require.NoError(t, err)
		assert.Equal(t, "default-stack-instance-global", got.Name)
	})

	t.Run("errors when every reference is dangling", func(t *testing.T) {
		scheme := controllerRefTestScheme(t)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		_, err := get(fakeClient, clfRef(addoncfg.InstallNamespace, "instance"))
		require.ErrorIs(t, err, errMissingOwnerRef)
	})

	t.Run("errors when the referenced resource is not owned by MCOA", func(t *testing.T) {
		scheme := controllerRefTestScheme(t)
		userCLF := &loggingv1.ClusterLogForwarder{
			ObjectMeta: metav1.ObjectMeta{Name: "user-clf", Namespace: addoncfg.InstallNamespace},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(userCLF).Build()

		_, err := get(fakeClient, clfRef(addoncfg.InstallNamespace, "user-clf"))
		require.ErrorIs(t, err, errMissingOwnerRef)
	})

	t.Run("errors when there are no references at all", func(t *testing.T) {
		scheme := controllerRefTestScheme(t)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		_, err := get(fakeClient)
		require.ErrorIs(t, err, errMissingResourceRefs)
	})
}
