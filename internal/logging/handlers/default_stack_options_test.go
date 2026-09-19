package handlers

import (
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func cmaoOwnerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: addonapiv1beta1.GroupVersion.String(),
		Kind:       "ClusterManagementAddOn",
		Name:       addoncfg.Name,
		Controller: ptr.To(true),
	}
}

func configRef(group, resource, name, namespace string) addonapiv1beta1.ConfigReference {
	return addonapiv1beta1.ConfigReference{
		ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{Group: group, Resource: resource},
		DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
			ConfigReferent: addonapiv1beta1.ConfigReferent{Name: name, Namespace: namespace},
		},
	}
}

func TestBuildDefaultStackCollectionOptions(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, loggingv1.AddToScheme(scheme))
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	clf := &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "mcoa-default-global",
			Namespace:       addoncfg.InstallNamespace,
			OwnerReferences: []metav1.OwnerReference{cmaoOwnerRef()},
		},
	}
	collectionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifests.DefaultCollectionMTLSSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
	}
	clfRef := configRef(loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource, clf.Name, clf.Namespace)

	t.Run("loads collection without touching storage", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clf, collectionSecret).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"},
			Status:     addonapiv1beta1.ManagedClusterAddOnStatus{ConfigReferences: []addonapiv1beta1.ConfigReference{clfRef}},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		require.NoError(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
		assert.NotNil(t, opts.DefaultStack.Collection.ClusterLogForwarder)
		assert.Nil(t, opts.DefaultStack.Storage.LokiStack)
	})

	t.Run("disabled default stack is a no-op", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"}}
		opts := &manifests.Options{}
		require.NoError(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
		assert.Nil(t, opts.DefaultStack.Collection.ClusterLogForwarder)
	})
}

func TestBuildDefaultStackStorageOptions(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lokiv1.AddToScheme(scheme))
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	ls := &lokiv1.LokiStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "mcoa-default-global",
			Namespace:       addoncfg.InstallNamespace,
			OwnerReferences: []metav1.OwnerReference{cmaoOwnerRef()},
		},
		Spec: lokiv1.LokiStackSpec{
			Storage: lokiv1.ObjectStorageSpec{
				Secret: lokiv1.ObjectStorageSecretSpec{Name: manifests.DefaultStorageObjStorageSecretName},
			},
		},
	}
	objStorageSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifests.DefaultStorageObjStorageSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
	}
	storageMTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifests.DefaultStorageMTLSSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
	}
	lsRef := configRef(lokiv1.GroupVersion.Group, addoncfg.LokiStacksResource, ls.Name, ls.Namespace)

	t.Run("no LokiStack ref skips storage", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ls).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		require.NoError(t, buildDefaultStackStorageOptions(t.Context(), k8s, mcAddon, opts))
		assert.Nil(t, opts.DefaultStack.Storage.LokiStack)
	})

	t.Run("MCAO LokiStack ref loads storage regardless of IsHub", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ls, objStorageSecret, storageMTLSSecret).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace},
			Status:     addonapiv1beta1.ManagedClusterAddOnStatus{ConfigReferences: []addonapiv1beta1.ConfigReference{lsRef}},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		opts.IsHub = false
		require.NoError(t, buildDefaultStackStorageOptions(t.Context(), k8s, mcAddon, opts))
		require.NotNil(t, opts.DefaultStack.Storage.LokiStack)
		assert.Equal(t, ls.Name, opts.DefaultStack.Storage.LokiStack.Name)
		assert.Equal(t, manifests.DefaultStorageObjStorageSecretName, opts.DefaultStack.Storage.ObjStorageSecret.Name)
		assert.Equal(t, manifests.DefaultStorageMTLSSecretName, opts.DefaultStack.Storage.MTLSSecret.Name)
	})

	t.Run("IsHub without LokiStack ref still skips storage", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ls).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}, IsHub: true}
		require.NoError(t, buildDefaultStackStorageOptions(t.Context(), k8s, mcAddon, opts))
		assert.Nil(t, opts.DefaultStack.Storage.LokiStack)
	})
}
