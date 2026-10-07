package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEnvtestBootstrap(t *testing.T) {
	testEnv := SetupTestEnv(t)
	require.NotNil(t, testEnv)
	require.NotNil(t, testEnv.K8sClient)

	ctx := t.Context()

	// Verify core namespace creation
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-bootstrap"},
	}
	err := testEnv.K8sClient.Create(ctx, ns)
	require.NoError(t, err)

	// Verify custom resource (CRD) creation
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-aodc",
			Namespace: "test-bootstrap",
		},
	}
	err = testEnv.K8sClient.Create(ctx, aodc)
	require.NoError(t, err)

	fetched := &addonv1beta1.AddOnDeploymentConfig{}
	err = testEnv.K8sClient.Get(ctx, client.ObjectKeyFromObject(aodc), fetched)
	require.NoError(t, err)
	assert.Equal(t, "test-aodc", fetched.Name)
}
