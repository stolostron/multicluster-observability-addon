package handlers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = operatorv1alpha1.AddToScheme(scheme.Scheme)

func TestInstallCOO(t *testing.T) {
	tests := []struct {
		name                    string
		isHub                   bool
		options                 addon.Options
		subscription            *operatorv1alpha1.Subscription
		expectedUIPluginInstall bool
		expectedCOOInstall      bool
		expectedErrMsg          string
	}{
		{
			name:                    "Non-hub cluster with no features enabled",
			isHub:                   false,
			options:                 addon.Options{},
			expectedUIPluginInstall: false,
			expectedCOOInstall:      false,
		},
		{
			name:  "Hub cluster with incident detection enabled but no COO installed",
			isHub: true,
			options: addon.Options{
				Platform: addon.PlatformOptions{
					Enabled: true,
					AnalyticsOptions: addon.AnalyticsOptions{
						IncidentDetection: addon.IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			expectedUIPluginInstall: true,
			expectedCOOInstall:      true,
		},
		{
			name:                    "Hub cluster with no features enabled",
			isHub:                   true,
			options:                 addon.Options{},
			expectedUIPluginInstall: false,
			expectedCOOInstall:      false,
		},
		{
			name:  "Hub cluster with COO installed and incident detection enabled",
			isHub: true,
			options: addon.Options{
				Platform: addon.PlatformOptions{
					Enabled: true,
					AnalyticsOptions: addon.AnalyticsOptions{
						IncidentDetection: addon.IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      addoncfg.CooSubscriptionName,
					Namespace: addoncfg.CooSubscriptionNamespace,
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: addoncfg.CooSubscriptionChannel,
				},
			},
			expectedUIPluginInstall: true,
			expectedCOOInstall:      false,
		},
		{
			name:  "Hub cluster with COO installed with multicluster-observability-addon release label",
			isHub: true,
			options: addon.Options{
				Platform: addon.PlatformOptions{
					Enabled: true,
					AnalyticsOptions: addon.AnalyticsOptions{
						IncidentDetection: addon.IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      addoncfg.CooSubscriptionName,
					Namespace: addoncfg.CooSubscriptionNamespace,
					Labels: map[string]string{
						"release": "multicluster-observability-addon",
					},
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: addoncfg.CooSubscriptionChannel,
				},
			},
			expectedUIPluginInstall: true,
			expectedCOOInstall:      true,
		},
		{
			name:  "Hub cluster with wrong version of COO installed and incident detection enabled",
			isHub: true,
			options: addon.Options{
				Platform: addon.PlatformOptions{
					Enabled: true,
					AnalyticsOptions: addon.AnalyticsOptions{
						IncidentDetection: addon.IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      addoncfg.CooSubscriptionName,
					Namespace: addoncfg.CooSubscriptionNamespace,
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: "wrong-channel",
				},
			},
			expectedUIPluginInstall: false,
			expectedCOOInstall:      false,
			expectedErrMsg:          addoncfg.ErrInvalidSubscriptionChannel.Error(),
		},
		{
			name:  "Hub cluster with metrics enabled but not incident detection",
			isHub: true,
			options: addon.Options{
				Platform: addon.PlatformOptions{
					Enabled: true,
					Metrics: addon.MetricsOptions{
						CollectionEnabled: true,
					},
				},
			},
			expectedUIPluginInstall: false,
			expectedCOOInstall:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k8sClientBuilder := fake.NewClientBuilder().
				WithScheme(scheme.Scheme)

			if tc.subscription != nil {
				k8sClientBuilder = k8sClientBuilder.WithObjects(tc.subscription)
			}

			var result bool
			var err error
			if tc.isHub {
				result, err = InstallOfCOOOnTheHubIsNeeded(context.Background(), k8sClientBuilder.Build(), logr.Discard())
			}
			cooValues := manifests.BuildValues(tc.options, result, tc.isHub, false)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedUIPluginInstall, cooValues.Enabled)
			assert.Equal(t, tc.expectedCOOInstall, cooValues.InstallCOO)
		})
	}
}

func TestInstallOfCOOOnSpokeIsNeeded(t *testing.T) {
	tests := []struct {
		name            string
		claims          []clusterv1.ManagedClusterClaim
		expectedInstall bool
	}{
		{
			name:            "no ClusterClaims yet: defer",
			claims:          nil,
			expectedInstall: false,
		},
		{
			name: "COO not installed: safe to install",
			claims: []clusterv1.ManagedClusterClaim{
				{Name: addoncfg.CooInstalledClaimName, Value: "false"},
				{Name: addoncfg.CooManagedByClaimName, Value: ""},
			},
			expectedInstall: true,
		},
		{
			name: "COO installed by MCOA: keep managing",
			claims: []clusterv1.ManagedClusterClaim{
				{Name: addoncfg.CooInstalledClaimName, Value: "true"},
				{Name: addoncfg.CooManagedByClaimName, Value: "mcoa"},
			},
			expectedInstall: true,
		},
		{
			name: "COO installed by external party: don't install",
			claims: []clusterv1.ManagedClusterClaim{
				{Name: addoncfg.CooInstalledClaimName, Value: "true"},
				{Name: addoncfg.CooManagedByClaimName, Value: "external"},
			},
			expectedInstall: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "spoke-1"},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: tc.claims,
				},
			}
			result := InstallOfCOOOnSpokeIsNeeded(cluster, logr.Discard())
			assert.Equal(t, tc.expectedInstall, result)
		})
	}
}
