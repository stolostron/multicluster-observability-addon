package addon

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = operatorv1alpha1.AddToScheme(scheme.Scheme)

func TestSkipInstallCOO(t *testing.T) {
	tests := []struct {
		name                string
		isHub               bool
		options             Options
		subscription        *operatorv1alpha1.Subscription
		expectedSkipInstall bool
		expectedErrMsg      string
	}{
		{
			name:                "Non-hub cluster with no features enabled",
			isHub:               false,
			options:             Options{},
			expectedSkipInstall: false,
		},
		{
			name:  "Non-hub cluster with incident detection enabled",
			isHub: false,
			options: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						IncidentDetection: IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			expectedSkipInstall: true,
		},
		{
			name:  "Hub cluster with incident detection enabled but no COO installed",
			isHub: true,
			options: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						IncidentDetection: IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			expectedSkipInstall: true,
		},
		{
			name:                "Hub cluster with no features enabled",
			isHub:               true,
			options:             Options{},
			expectedSkipInstall: false,
		},
		{
			name:  "Hub cluster with COO installed and incident detection enabled",
			isHub: true,
			options: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						IncidentDetection: IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      COOSubscriptionName,
					Namespace: COOSubscriptionNamespace,
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: cooSubscriptionChannel,
				},
			},
			expectedSkipInstall: false,
		},
		{
			name:  "Hub cluster with COO installed with multicluster-observability-addon release label",
			isHub: true,
			options: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						IncidentDetection: IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      COOSubscriptionName,
					Namespace: COOSubscriptionNamespace,
					Labels: map[string]string{
						"release": "multicluster-observability-addon",
					},
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: cooSubscriptionChannel,
				},
			},
			expectedSkipInstall: true,
		},
		{
			name:  "Hub cluster with wrong version of COO installed and incident detection enabled",
			isHub: true,
			options: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						IncidentDetection: IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      COOSubscriptionName,
					Namespace: COOSubscriptionNamespace,
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: "wrong-channel",
				},
			},
			expectedSkipInstall: false,
			expectedErrMsg:      errInvalidSubscriptionChannel.Error(),
		},
		{
			name:  "Hub cluster with metrics enabled but not incident detection",
			isHub: true,
			options: Options{
				Platform: PlatformOptions{
					Enabled: true,
					Metrics: MetricsOptions{
						CollectionEnabled: true,
					},
				},
			},
			expectedSkipInstall: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k8sClientBuilder := fake.NewClientBuilder().
				WithScheme(scheme.Scheme)

			if tc.subscription != nil {
				k8sClientBuilder = k8sClientBuilder.WithObjects(tc.subscription)
			}

			result, err := InstallCOO(context.Background(), k8sClientBuilder.Build(), logr.Discard(), tc.isHub, tc.options)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedSkipInstall, result)
		})
	}
}
