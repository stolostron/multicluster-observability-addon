package manifests

import (
	"testing"

	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
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
				assert.Error(t, err)
				assert.Equal(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedResult, result)
		})
	}
}
