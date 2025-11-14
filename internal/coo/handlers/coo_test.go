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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
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
			name:  "Non-hub cluster with incident detection enabled",
			isHub: false,
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

			result, err := InstallCOO(context.Background(), k8sClientBuilder.Build(), logr.Discard(), tc.isHub)
			cooValues := manifests.BuildValues(tc.options, result, tc.isHub)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedUIPluginInstall, cooValues.Enabled)
			assert.Equal(t, tc.expectedCOOInstall, cooValues.InstallCOO)
		})
	}
}
