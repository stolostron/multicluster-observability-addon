package handlers

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
		name           string
		isHub          bool
		subscription   *operatorv1alpha1.Subscription
		expectedResult bool
		expectedErrMsg string
	}{
		{
			name:           "Non-hub cluster",
			isHub:          false,
			expectedResult: true,
		},
		{
			name:           "Hub cluster",
			isHub:          true,
			expectedResult: true,
		},
		{
			name:  "Hub cluster with COO installed",
			isHub: true,
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cooSubscriptionName,
					Namespace: cooSubscriptionNamespace,
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: cooSubscriptionChannel,
				},
			},
			expectedResult: false,
		},
		{
			name:  "Hub cluster with COO installed with multicluster-observability-addon release label",
			isHub: true,
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cooSubscriptionName,
					Namespace: cooSubscriptionNamespace,
					Labels: map[string]string{
						"release": "multicluster-observability-addon",
					},
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: cooSubscriptionChannel,
				},
			},
			expectedResult: true,
		},
		{
			name:  "Hub cluster with wrong version of COO installed and incident detection enabled",
			isHub: true,
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cooSubscriptionName,
					Namespace: cooSubscriptionNamespace,
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: "wrong-channel",
				},
			},
			expectedResult: false,
			expectedErrMsg: errInvalidSubscriptionChannel.Error(),
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

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}
