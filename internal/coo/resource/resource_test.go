package resource

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = persesv1.AddToScheme(scheme)
	return scheme
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

func TestReconcileCOOOperatorCatalogSource(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = persesv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = operatorsv1.AddToScheme(scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name         string
		opts         addon.Options
		wantSource   string
		wantSourceNS string
	}{
		{
			name:         "defaults to the redhat-operators catalog",
			opts:         addon.Options{},
			wantSource:   addoncfg.DefaultCooCatalogSource,
			wantSourceNS: addoncfg.DefaultCooCatalogSourceNamespace,
		},
		{
			name: "uses the custom catalog from the addon options",
			opts: addon.Options{
				COO: addon.COOOptions{
					CatalogSource:          "cs-redhat-operator-index-v4-20",
					CatalogSourceNamespace: "mirrored-catalogs",
				},
			},
			wantSource:   "cs-redhat-operator-index-v4-20",
			wantSourceNS: "mirrored-catalogs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			r := &HubResourceReconciler{
				Client: fakeClient,
				Logger: logr.Discard(),
				Opts:   tt.opts,
			}

			require.NoError(t, r.reconcileCOOOperator(t.Context(), true))

			sub := &operatorsv1alpha1.Subscription{}
			require.NoError(t, fakeClient.Get(t.Context(),
				client.ObjectKey{Namespace: addoncfg.CooSubscriptionNamespace, Name: addoncfg.CooSubscriptionName}, sub))
			require.NotNil(t, sub.Spec)
			assert.Equal(t, tt.wantSource, sub.Spec.CatalogSource)
			assert.Equal(t, tt.wantSourceNS, sub.Spec.CatalogSourceNamespace)
		})
	}
}

func TestCheckDashboardWriteReadiness(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))

	dashboardCRD := func(conversion *apiextensionsv1.CustomResourceConversion, versions ...apiextensionsv1.CustomResourceDefinitionVersion) *apiextensionsv1.CustomResourceDefinition {
		return &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "persesdashboards.perses.dev"},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Versions:   versions,
				Conversion: conversion,
			},
		}
	}
	v1alpha1 := apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1alpha1", Served: true}
	v1alpha1Storage := apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1alpha1", Served: true, Storage: true}
	v1alpha1Unserved := apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1alpha1"}
	v1alpha2Storage := apiextensionsv1.CustomResourceDefinitionVersion{Name: "v1alpha2", Served: true, Storage: true}
	webhook := &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.WebhookConverter}
	none := &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}

	deleting := dashboardCRD(webhook, v1alpha1, v1alpha2Storage)
	deleting.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	deleting.Finalizers = []string{"test.perses.dev/keep"}

	tests := []struct {
		name    string
		crd     *apiextensionsv1.CustomResourceDefinition
		readErr error // the API reader returns this error instead of the CRD
		ready   bool  // dashboard writes may start
		wait    bool  // the caller retries after a delay
	}{
		{name: "CRD missing", wait: true},
		{name: "CRD read forbidden", readErr: errors.NewForbidden(apiextensionsv1.Resource("customresourcedefinitions"), "persesdashboards.perses.dev", assert.AnError)},
		{name: "CRD being deleted", crd: deleting, wait: true},
		{name: "v1alpha1 not served", crd: dashboardCRD(webhook, v1alpha1Unserved, v1alpha2Storage)},
		{name: "v1alpha1 removed", crd: dashboardCRD(webhook, v1alpha2Storage)},
		{name: "v1alpha1 is the storage version", crd: dashboardCRD(nil, v1alpha1Storage), ready: true},
		{name: "v1alpha2 storage without conversion", crd: dashboardCRD(nil, v1alpha1, v1alpha2Storage), wait: true},
		{name: "v1alpha2 storage with None conversion", crd: dashboardCRD(none, v1alpha1, v1alpha2Storage), wait: true},
		{name: "v1alpha2 storage with webhook conversion", crd: dashboardCRD(webhook, v1alpha1, v1alpha2Storage), ready: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiReader := fake.NewClientBuilder().WithScheme(scheme)
			if tt.crd != nil {
				apiReader = apiReader.WithObjects(tt.crd)
			}
			if tt.readErr != nil {
				apiReader = apiReader.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
						return tt.readErr
					},
				})
			}
			r := &HubResourceReconciler{
				// The CRD is only in the API reader: the check must not read it from the cache.
				Client:    fake.NewClientBuilder().WithScheme(scheme).Build(),
				APIReader: apiReader.Build(),
				Logger:    logr.Discard(),
			}

			err := r.checkDashboardWriteReadiness(t.Context())
			switch {
			case tt.ready:
				require.NoError(t, err)
			case tt.wait:
				require.ErrorIs(t, err, ErrPersesAPINotReady)
			case tt.readErr != nil:
				// A read error other than NotFound is a reconcile error, not a wait.
				require.ErrorIs(t, err, tt.readErr)
				require.NotErrorIs(t, err, ErrPersesAPINotReady)
			default:
				require.ErrorIs(t, err, errPersesWriteVersionNotServed)
				require.NotErrorIs(t, err, ErrPersesAPINotReady)
			}
		})
	}
}
