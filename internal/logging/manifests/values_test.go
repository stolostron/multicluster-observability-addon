package manifests

import (
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldInstallCLO(t *testing.T) {
	const testChannel = "stable"

	tests := []struct {
		name           string
		opts           Options
		expectedResult bool
		expectedError  error
	}{
		{
			name: "No subscription provided",
			opts: Options{
				ClusterLoggingSubscription: nil,
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "Empty subscription name",
			opts: Options{
				ClusterLoggingSubscription: &operatorv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name: "",
					},
				},
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "Subscription with mismatched channel",
			opts: Options{
				ClusterLoggingSubscription: &operatorv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-logging",
					},
					Spec: &operatorv1alpha1.SubscriptionSpec{
						Channel: "wrong-channel",
					},
				},
			},
			expectedResult: false,
			expectedError:  errInvalidSubscriptionChannel,
		},
		{
			name: "Subscription with matching channel",
			opts: Options{
				ClusterLoggingSubscription: &operatorv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-logging",
					},
					Spec: &operatorv1alpha1.SubscriptionSpec{
						Channel: testChannel,
					},
				},
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Subscription with our release label",
			opts: Options{
				ClusterLoggingSubscription: &operatorv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-logging",
						Labels: map[string]string{
							"release": "multicluster-observability-addon",
						},
					},
					Spec: &operatorv1alpha1.SubscriptionSpec{
						Channel: testChannel,
					},
				},
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "Subscription with different release label value",
			opts: Options{
				ClusterLoggingSubscription: &operatorv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-logging",
						Labels: map[string]string{
							"release": "some-other-value",
						},
					},
					Spec: &operatorv1alpha1.SubscriptionSpec{
						Channel: testChannel,
					},
				},
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Subscription with different label key",
			opts: Options{
				ClusterLoggingSubscription: &operatorv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-logging",
						Labels: map[string]string{
							"app": "multicluster-observability-addon",
						},
					},
					Spec: &operatorv1alpha1.SubscriptionSpec{
						Channel: testChannel,
					},
				},
			},
			expectedResult: false,
			expectedError:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := shouldInstallCLO(tc.opts, testChannel)

			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestBuildValues_DefaultStackStorage(t *testing.T) {
	clf := &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{Name: "clf", Namespace: "ns"},
	}
	baseOpts := Options{
		Platform: addon.LogsOptions{DefaultStack: true},
		DefaultStack: DefaultStack{
			Collection: Collection{
				ClusterLogForwarder: clf,
			},
		},
	}

	t.Run("storage enabled when LokiStack is referenced", func(t *testing.T) {
		opts := baseOpts
		opts.DefaultStack.Storage.LokiStack = &lokiv1.LokiStack{
			ObjectMeta: metav1.ObjectMeta{Name: "ls"},
			Spec:       lokiv1.LokiStackSpec{Size: lokiv1.SizeOneXPico},
		}
		opts.DefaultStack.Storage.ObjStorageSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "obj"}}
		opts.DefaultStack.Storage.MTLSSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls"}}

		values, err := BuildValues(opts)
		require.NoError(t, err)
		assert.True(t, values.Managed.Collection.Enabled)
		assert.True(t, values.Managed.Storage.Enabled)
		assert.NotEmpty(t, values.Managed.Storage.LSSpec)
	})

	t.Run("storage disabled without LokiStack even if IsHub", func(t *testing.T) {
		opts := baseOpts
		opts.IsHub = true

		values, err := BuildValues(opts)
		require.NoError(t, err)
		assert.True(t, values.Managed.Collection.Enabled)
		assert.False(t, values.Managed.Storage.Enabled)
		assert.Empty(t, values.Managed.Storage.LSSpec)
	})
}
