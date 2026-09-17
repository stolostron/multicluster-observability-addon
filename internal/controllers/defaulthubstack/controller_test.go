package defaulthubstack

import (
	"testing"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	cooresource "github.com/stolostron/multicluster-observability-addon/internal/coo/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = addonv1beta1.Install(scheme)
	_ = operatorsv1.AddToScheme(scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme)
	_ = persesv1.AddToScheme(scheme)
	_ = uiplugin.AddToScheme(scheme)
	return scheme
}

func TestManagedByPredicate(t *testing.T) {
	t.Run("matches managed-by label", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					addoncfg.ManagedByK8sLabelKey: cooresource.ManagedByLabelValue,
				},
			},
		}
		assert.True(t, managedByPredicate.Create(event.CreateEvent{Object: obj}))
	})

	t.Run("rejects wrong label value", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					addoncfg.ManagedByK8sLabelKey: "someone-else",
				},
			},
		}
		assert.False(t, managedByPredicate.Create(event.CreateEvent{Object: obj}))
	})

	t.Run("rejects no label", func(t *testing.T) {
		obj := &corev1.ConfigMap{}
		assert.False(t, managedByPredicate.Create(event.CreateEvent{Object: obj}))
	})
}

func TestMCOAAODCPredicate(t *testing.T) {
	t.Run("matches MCOA AODC", func(t *testing.T) {
		obj := &addonv1beta1.AddOnDeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      addoncfg.Name,
				Namespace: addoncfg.InstallNamespace,
			},
		}
		assert.True(t, mcoaAODCPredicate.Create(event.CreateEvent{Object: obj}))
	})

	t.Run("rejects wrong name", func(t *testing.T) {
		obj := &addonv1beta1.AddOnDeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wrong-name",
				Namespace: addoncfg.InstallNamespace,
			},
		}
		assert.False(t, mcoaAODCPredicate.Create(event.CreateEvent{Object: obj}))
	})

	t.Run("rejects wrong namespace", func(t *testing.T) {
		obj := &addonv1beta1.AddOnDeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      addoncfg.Name,
				Namespace: "wrong-ns",
			},
		}
		assert.False(t, mcoaAODCPredicate.Create(event.CreateEvent{Object: obj}))
	})
}

func TestReconcile_AODCNotFound(t *testing.T) {
	scheme := newTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &DefaultHubStackReconciler{
		Client:            fakeClient,
		Log:               logr.Discard(),
		Scheme:            scheme,
		watchesRegistered: true,
	}

	result, err := r.Reconcile(t.Context(), reconcile.Request{})
	require.NoError(t, err)
	assert.NotZero(t, result.RequeueAfter)
}

func TestReconcile_EmptyOptions(t *testing.T) {
	scheme := newTestScheme()
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(aodc).Build()

	r := &DefaultHubStackReconciler{
		Client:            fakeClient,
		Log:               logr.Discard(),
		Scheme:            scheme,
		watchesRegistered: true,
	}

	result, err := r.Reconcile(t.Context(), reconcile.Request{})
	require.NoError(t, err)
	assert.NotZero(t, result.RequeueAfter)

	uip := &uiplugin.UIPlugin{}
	err = fakeClient.Get(t.Context(), client.ObjectKey{Name: "monitoring"}, uip)
	assert.True(t, err != nil, "UIPlugin should not exist with empty options")
}

func TestReconcile_COOSubscriptionWrongChannel(t *testing.T) {
	scheme := newTestScheme()
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{Name: "platformMetricsCollection", Value: "prometheusagent/v1alpha1"},
				{Name: "metricsHubHostname", Value: "https://the-hub.com"},
			},
		},
	}
	sub := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.CooSubscriptionName,
			Namespace: addoncfg.CooSubscriptionNamespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			Channel: "wrong-channel",
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(aodc, sub).Build()

	r := &DefaultHubStackReconciler{
		Client:            fakeClient,
		Log:               logr.Discard(),
		Scheme:            scheme,
		watchesRegistered: true,
	}

	_, err := r.Reconcile(t.Context(), reconcile.Request{})
	require.ErrorIs(t, err, addoncfg.ErrInvalidSubscriptionChannel)
}
