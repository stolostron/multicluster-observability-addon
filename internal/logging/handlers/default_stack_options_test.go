package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	routev1 "github.com/openshift/api/route/v1"
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
	require.NoError(t, routev1.Install(scheme))

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
	const routeHost = "obs-api.apps.example.com"
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: obsAPIRouteName, Namespace: addoncfg.InstallNamespace},
		Spec:       routev1.RouteSpec{Host: routeHost},
	}
	serverCert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifests.ObsAPIServerMTLSSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string][]byte{corev1.TLSCertKey: testServerCertPEM(t, routeHost)},
	}

	t.Run("loads collection without touching storage", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clf, collectionSecret, route, serverCert).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"},
			Status:     addonapiv1beta1.ManagedClusterAddOnStatus{ConfigReferences: []addonapiv1beta1.ConfigReference{clfRef}},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		require.NoError(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
		assert.NotNil(t, opts.DefaultStack.Collection.ClusterLogForwarder)
		assert.Nil(t, opts.DefaultStack.Storage.LokiStack)
		assert.Equal(t, "https://obs-api.apps.example.com/api/logs/v1/otlp/v1/logs", opts.DefaultStack.LokiURL)
	})

	t.Run("no CLF ref skips collection until the hub publishes it", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		require.NoError(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
		assert.Nil(t, opts.DefaultStack.Collection.ClusterLogForwarder)
	})

	t.Run("stale missing CLF ref does not hide the owned template", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clf, collectionSecret, route, serverCert).Build()
		missing := configRef(loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource, "default-stack-instance-global", addoncfg.InstallNamespace)
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"},
			Status:     addonapiv1beta1.ManagedClusterAddOnStatus{ConfigReferences: []addonapiv1beta1.ConfigReference{missing, clfRef}},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		require.NoError(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
		assert.Equal(t, clf.Name, opts.DefaultStack.Collection.ClusterLogForwarder.Name)
	})

	t.Run("errors until the obs-api server certificate includes the route host", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clf, collectionSecret, route).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"},
			Status:     addonapiv1beta1.ManagedClusterAddOnStatus{ConfigReferences: []addonapiv1beta1.ConfigReference{clfRef}},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		require.Error(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
	})

	t.Run("errors until the obs-api route has a host", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clf, collectionSecret).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"},
			Status:     addonapiv1beta1.ManagedClusterAddOnStatus{ConfigReferences: []addonapiv1beta1.ConfigReference{clfRef}},
		}
		opts := &manifests.Options{Platform: addon.LogsOptions{DefaultStack: true}}
		require.Error(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
	})

	t.Run("disabled default stack is a no-op", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).Build()
		mcAddon := &addonapiv1beta1.ManagedClusterAddOn{ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "spoke-1"}}
		opts := &manifests.Options{}
		require.NoError(t, buildDefaultStackCollectionOptions(t.Context(), k8s, mcAddon, opts))
		assert.Nil(t, opts.DefaultStack.Collection.ClusterLogForwarder)
	})
}

func testServerCertPEM(t *testing.T, dnsNames ...string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: manifests.ObsAPIServerCertCommonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
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
