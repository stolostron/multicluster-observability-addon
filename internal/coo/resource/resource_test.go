package resource

import (
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/mcoagateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = certmanagerv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = persesv1.AddToScheme(scheme)
	return scheme
}

func TestLokiDatasourceClientCertificateLifecycle(t *testing.T) {
	k8s := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	r := &HubResourceReconciler{Client: k8s, Logger: logr.Discard()}
	ctx := t.Context()

	require.NoError(t, r.reconcileDatasources(ctx, false, false, true))

	cert := &certmanagerv1.Certificate{}
	certKey := client.ObjectKey{Name: mcoagateway.DefaultPersesLokiClientMTLSSecretName, Namespace: addoncfg.InstallNamespace}
	require.NoError(t, k8s.Get(ctx, certKey, cert))
	assert.Equal(t, ManagedByLabelValue, cert.Labels[addoncfg.ManagedByK8sLabelKey])
	require.NotNil(t, cert.Spec.SecretTemplate)
	assert.Equal(t, ManagedByLabelValue, cert.Spec.SecretTemplate.Labels[addoncfg.ManagedByK8sLabelKey])
	assert.Contains(t, cert.Spec.Usages, certmanagerv1.UsageClientAuth)

	ds := &persesv1.PersesDatasource{}
	dsKey := client.ObjectKey{Name: dsLoki, Namespace: addoncfg.InstallNamespace}
	require.NoError(t, k8s.Get(ctx, dsKey, ds))
	require.NotNil(t, ds.Spec.Client)
	require.NotNil(t, ds.Spec.Client.TLS)
	require.NotNil(t, ds.Spec.Client.TLS.CaCert)
	require.NotNil(t, ds.Spec.Client.TLS.UserCert)
	assert.Equal(t, mcoagateway.DefaultStorageMTLSSecretName, ds.Spec.Client.TLS.CaCert.Name)
	assert.Equal(t, "ca.crt", ds.Spec.Client.TLS.CaCert.CertPath)
	assert.Equal(t, cert.Spec.SecretName, ds.Spec.Client.TLS.UserCert.Name)
	assert.Equal(t, "tls.crt", ds.Spec.Client.TLS.UserCert.CertPath)
	assert.Equal(t, "tls.key", ds.Spec.Client.TLS.UserCert.PrivateKeyPath)

	issuedSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: cert.Spec.SecretName, Namespace: addoncfg.InstallNamespace,
		Labels: cert.Spec.SecretTemplate.Labels,
	}}
	require.NoError(t, k8s.Create(ctx, issuedSecret))

	require.NoError(t, r.reconcileDatasources(ctx, false, false, false))
	assert.True(t, apierrors.IsNotFound(k8s.Get(ctx, dsKey, ds)))
	assert.True(t, apierrors.IsNotFound(k8s.Get(ctx, certKey, cert)))
	assert.True(t, apierrors.IsNotFound(k8s.Get(ctx, certKey, issuedSecret)))
}

func newDashboard(name, namespace string) *persesv1.PersesDashboard {
	return &persesv1.PersesDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestCleanupOrphanDashboards(t *testing.T) {
	knownNames := allPossibleDashboardNames()
	require.NotEmpty(t, knownNames, "allPossibleDashboardNames should return at least one name")

	var knownName string
	for name := range knownNames {
		knownName = name
		break
	}

	t.Run("deletes orphan dashboard with known name", func(t *testing.T) {
		orphan := newDashboard(knownName, addoncfg.InstallNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(orphan).Build()

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), map[string]struct{}{})
		require.NoError(t, err)

		list := &persesv1.PersesDashboardList{}
		require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(addoncfg.InstallNamespace)))
		assert.Empty(t, list.Items)
	})

	t.Run("preserves user-created dashboard with unknown name", func(t *testing.T) {
		userDashboard := newDashboard("my-custom-dashboard", addoncfg.InstallNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(userDashboard).Build()

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), map[string]struct{}{})
		require.NoError(t, err)

		list := &persesv1.PersesDashboardList{}
		require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(addoncfg.InstallNamespace)))
		assert.Len(t, list.Items, 1)
		assert.Equal(t, "my-custom-dashboard", list.Items[0].Name)
	})

	t.Run("preserves currently desired dashboard", func(t *testing.T) {
		desired := newDashboard(knownName, addoncfg.InstallNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(desired).Build()

		desiredNames := map[string]struct{}{
			addoncfg.InstallNamespace + "/" + knownName: {},
		}

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), desiredNames)
		require.NoError(t, err)

		list := &persesv1.PersesDashboardList{}
		require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(addoncfg.InstallNamespace)))
		assert.Len(t, list.Items, 1)
	})

	t.Run("cleans up across both namespaces", func(t *testing.T) {
		orphan1 := newDashboard(knownName, addoncfg.InstallNamespace)
		orphan2 := newDashboard(knownName, addoncfg.AnalyticsNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(orphan1, orphan2).Build()

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), map[string]struct{}{})
		require.NoError(t, err)

		for _, ns := range []string{addoncfg.InstallNamespace, addoncfg.AnalyticsNamespace} {
			list := &persesv1.PersesDashboardList{}
			require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(ns)))
			assert.Empty(t, list.Items, "namespace %s should be empty", ns)
		}
	})
}

func TestAllPossibleDashboardNames(t *testing.T) {
	names := allPossibleDashboardNames()
	assert.NotEmpty(t, names)

	second := allPossibleDashboardNames()
	assert.Equal(t, names, second, "should return the same set on subsequent calls")

	for _, name := range virtualizationDashboardNames {
		assert.Contains(t, names, name)
	}
}

var virtualizationDashboardNames = []string{
	"acm-virtual-machines-inventory",
	"acm-virtual-machines-utilization",
	"acm-virtual-machines-service-level",
	"acm-virtual-machines-by-time-in-status",
	"acm-node-memory-overview",
	"acm-openshift-virtualization-overview",
	"acm-openshift-virtualization-single-cluster-view",
	"acm-openshift-virtualization-single-vm-view",
	"acm-virtual-machines-top-consumers",
}

func TestBuildVirtualizationPersesDashboards(t *testing.T) {
	dashboards, err := buildVirtualizationPersesDashboards()
	require.NoError(t, err)
	require.Len(t, dashboards, len(virtualizationDashboardNames))

	byName := map[string]persesv1.PersesDashboard{}
	for _, db := range dashboards {
		byName[db.Name] = db
	}
	for _, name := range virtualizationDashboardNames {
		require.Contains(t, byName, name)
		db := byName[name]
		assert.Equal(t, addoncfg.InstallNamespace, db.Namespace)
		assert.Equal(t, ManagedByLabelValue, db.Labels[addoncfg.ManagedByK8sLabelKey])
	}
}

func TestBuildDesiredDashboardsIncludesVirtualizationWhenMetricsUIEnabled(t *testing.T) {
	reconciler := &HubResourceReconciler{
		Opts: addon.Options{
			Platform: addon.PlatformOptions{
				Metrics: addon.MetricsOptions{
					CollectionEnabled: true,
					UI:                addon.MetricsUIOptions{Enabled: true},
				},
			},
		},
	}

	dashboards := reconciler.buildDesiredDashboards(false, false)
	namespaces := map[string]string{}
	for _, db := range dashboards {
		namespaces[db.Name] = db.Namespace
	}
	for _, name := range virtualizationDashboardNames {
		require.Contains(t, namespaces, name)
		assert.Equal(t, addoncfg.InstallNamespace, namespaces[name])
	}
}

func TestBuildDesiredDashboardsOmitsVirtualizationWhenMetricsUIDisabled(t *testing.T) {
	reconciler := &HubResourceReconciler{}
	dashboards := reconciler.buildDesiredDashboards(false, false)
	names := map[string]struct{}{}
	for _, db := range dashboards {
		names[db.Name] = struct{}{}
	}
	for _, name := range virtualizationDashboardNames {
		assert.NotContains(t, names, name)
	}
}
