package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

func TestNewTestCacheOptions_FailsOnMissingInformer(t *testing.T) {
	opts := NewTestCacheOptions()
	assert.True(t, opts.ReaderFailOnMissingInformer, "Test cache options must enable ReaderFailOnMissingInformer")
	require.NotNil(t, opts.DefaultTransform, "DefaultTransform must be configured")

	testEnv := SetupTestEnv(t)

	mgr, err := ctrl.NewManager(testEnv.Cfg, ctrl.Options{
		Scheme:  testEnv.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache:   opts,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		_ = mgr.Start(ctx)
	}()
	require.True(t, mgr.GetCache().WaitForCacheSync(ctx), "manager cache failed to sync")

	// Reading an unwatched resource through the manager's cached client must fail immediately.
	unwatched := &corev1.ConfigMap{}
	err = mgr.GetClient().Get(ctx, client.ObjectKey{Namespace: "default", Name: "non-existent"}, unwatched)
	require.Error(t, err)

	var errNotCached *cache.ErrResourceNotCached
	assert.ErrorAs(t, err, &errNotCached, "Expected ErrResourceNotCached when reading unwatched resource from cached client")
}
