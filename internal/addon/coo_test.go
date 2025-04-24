// filepath: /home/jmarcal/work/multicluster-observability-addon/internal/addon/coo_test.go
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
		subscription        *operatorv1alpha1.Subscription
		expectedSkipInstall bool
		expectedErrMsg      string
	}{
		{
			name:                "Non-hub cluster",
			isHub:               false,
			expectedSkipInstall: false,
		},
		{
			name:                "Hub cluster with no COO installed",
			isHub:               true,
			expectedSkipInstall: false,
		},
		{
			name:  "Hub cluster with COO installed",
			isHub: true,
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      COOSubscriptionName,
					Namespace: COOSubscriptionNamespace,
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: cooSubscriptionChannel,
				},
			},
			expectedSkipInstall: true,
		},
		{
			name:  "Hub cluster with wrong version of COO installed",
			isHub: true,
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k8sClientBuilder := fake.NewClientBuilder().
				WithScheme(scheme.Scheme)

			if tc.subscription != nil {
				k8sClientBuilder = k8sClientBuilder.WithObjects(tc.subscription)
			}

			result, err := SkipInstallCOO(context.Background(), k8sClientBuilder.Build(), logr.Discard(), tc.isHub)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedSkipInstall, result)
		})
	}
}
