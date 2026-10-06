package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
		expectedHubCOOInstall   *bool
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
			name:  "Non-hub cluster with incident detection enabled installs COO",
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
			name:  "Non-hub cluster with right-sizing does not install COO",
			isHub: false,
			options: addon.Options{
				Platform: addon.PlatformOptions{
					Enabled: true,
					AnalyticsOptions: addon.AnalyticsOptions{
						RightSizing: addon.RightSizingOptions{
							Delegated:        true,
							NamespaceEnabled: true,
						},
					},
				},
			},
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
			expectedCOOInstall:      false, // hub COO installation handled by HubResourceReconciler
			expectedHubCOOInstall:   ptr.To(true),
		},
		{
			name:                    "Hub cluster with no features enabled",
			isHub:                   true,
			options:                 addon.Options{},
			expectedUIPluginInstall: false,
			expectedCOOInstall:      false,
			expectedHubCOOInstall:   ptr.To(true),
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
			expectedHubCOOInstall:   ptr.To(false),
		},
		{
			name:  "Hub cluster with COO installed externally in openshift-operators",
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
					Namespace: "openshift-operators",
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: addoncfg.CooSubscriptionChannel,
				},
			},
			expectedUIPluginInstall: true,
			expectedCOOInstall:      false,
			expectedHubCOOInstall:   ptr.To(false),
		},
		{
			name:    "Hub cluster with external COO subscription carrying release label in openshift-operators is not mistaken for MCOA managed",
			isHub:   true,
			options: addon.Options{},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      addoncfg.CooSubscriptionName,
					Namespace: "openshift-operators",
					Labels: map[string]string{
						addoncfg.ReleaseLabelKey: addoncfg.Name,
					},
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: addoncfg.CooSubscriptionChannel,
				},
			},
			expectedHubCOOInstall: ptr.To(false),
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
						addoncfg.ReleaseLabelKey: addoncfg.Name,
					},
				},
				Spec: &operatorv1alpha1.SubscriptionSpec{
					Channel: addoncfg.CooSubscriptionChannel,
				},
			},
			expectedUIPluginInstall: true,
			expectedCOOInstall:      false, // hub COO installation handled by HubResourceReconciler
			expectedHubCOOInstall:   ptr.To(true),
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
			name:    "Hub cluster with nil Subscription spec returns ErrInvalidSubscriptionChannel",
			isHub:   true,
			options: addon.Options{},
			subscription: &operatorv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      addoncfg.CooSubscriptionName,
					Namespace: addoncfg.CooSubscriptionNamespace,
				},
				Spec: nil,
			},
			expectedErrMsg: addoncfg.ErrInvalidSubscriptionChannel.Error(),
		},
		{
			name:  "Hub cluster with right-sizing enabled",
			isHub: true,
			options: addon.Options{
				Platform: addon.PlatformOptions{
					Enabled: true,
					AnalyticsOptions: addon.AnalyticsOptions{
						RightSizing: addon.RightSizingOptions{
							Delegated:             true,
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
			},
			expectedUIPluginInstall: true,
			expectedCOOInstall:      false,
			expectedHubCOOInstall:   ptr.To(true),
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
			expectedHubCOOInstall:   ptr.To(true),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k8sClientBuilder := fake.NewClientBuilder().
				WithScheme(scheme.Scheme)

			if tc.subscription != nil {
				k8sClientBuilder = k8sClientBuilder.WithObjects(tc.subscription)
			}

			var canManage bool
			var err error
			if tc.isHub {
				canManage, err = CanManageCOOOnTheHub(context.Background(), k8sClientBuilder.Build(), logr.Discard())
			}
			cooValues := manifests.BuildValues(tc.options, tc.isHub)

			if tc.expectedErrMsg != "" {
				assert.False(t, canManage)
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			require.NoError(t, err)
			if tc.isHub && tc.expectedHubCOOInstall != nil {
				assert.Equal(t, *tc.expectedHubCOOInstall, canManage, "CanManageCOOOnTheHub return mismatch")
			}
			assert.Equal(t, tc.expectedUIPluginInstall, cooValues.Enabled)
			assert.Equal(t, tc.expectedCOOInstall, cooValues.InstallCOO)
		})
	}
}

type errorReader struct {
	client.Reader
	err error
}

func (e *errorReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return e.err
}

var errSimulatedAPI = errors.New("simulated API error")

func TestCanManageCOOOnTheHub_Error(t *testing.T) {
	reader := &errorReader{err: errSimulatedAPI}
	canManage, err := CanManageCOOOnTheHub(context.Background(), reader, logr.Discard())
	assert.False(t, canManage)
	assert.ErrorContains(t, err, "failed to get cluster observability operator subscription: simulated API error")
}

func TestCardinalityRulesConfigMapPredicate(t *testing.T) {
	pred := CardinalityRulesConfigMapPredicate()

	validCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      thanosRulerCustomRulesName,
			Namespace: addoncfg.InstallNamespace,
		},
	}
	wrongNameCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-rules",
			Namespace: addoncfg.InstallNamespace,
		},
	}
	wrongNamespaceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      thanosRulerCustomRulesName,
			Namespace: "other-namespace",
		},
	}

	assert.True(t, pred.Create(event.CreateEvent{Object: validCM}))
	assert.False(t, pred.Create(event.CreateEvent{Object: wrongNameCM}))
	assert.False(t, pred.Create(event.CreateEvent{Object: wrongNamespaceCM}))

	assert.True(t, pred.Update(event.UpdateEvent{ObjectNew: validCM}))
	assert.False(t, pred.Update(event.UpdateEvent{ObjectNew: wrongNameCM}))
	assert.False(t, pred.Update(event.UpdateEvent{ObjectNew: wrongNamespaceCM}))

	assert.True(t, pred.Delete(event.DeleteEvent{Object: validCM}))
	assert.False(t, pred.Delete(event.DeleteEvent{Object: wrongNameCM}))
	assert.False(t, pred.Delete(event.DeleteEvent{Object: wrongNamespaceCM}))

	assert.False(t, pred.Generic(event.GenericEvent{Object: validCM}))
}

func TestCooSubscriptionPredicate(t *testing.T) {
	pred := CooSubscriptionPredicate()

	validSub := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:       addoncfg.CooSubscriptionName,
			Namespace:  addoncfg.CooSubscriptionNamespace,
			Generation: 1,
			Labels:     map[string]string{addoncfg.ReleaseLabelKey: addoncfg.Name},
		},
	}
	wrongNameSub := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "other-operator",
			Namespace:  addoncfg.CooSubscriptionNamespace,
			Generation: 1,
		},
	}
	otherNamespaceSub := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:       addoncfg.CooSubscriptionName,
			Namespace:  "openshift-operators",
			Generation: 1,
		},
	}
	packageMatchSub := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "custom-coo",
			Namespace:  "openshift-operators",
			Generation: 1,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			Package: addoncfg.CooSubscriptionName,
		},
	}

	assert.True(t, pred.Create(event.CreateEvent{Object: validSub}))
	assert.False(t, pred.Create(event.CreateEvent{Object: wrongNameSub}))
	assert.True(t, pred.Create(event.CreateEvent{Object: otherNamespaceSub}))
	assert.True(t, pred.Create(event.CreateEvent{Object: packageMatchSub}))

	assert.True(t, pred.Delete(event.DeleteEvent{Object: validSub}))
	assert.False(t, pred.Delete(event.DeleteEvent{Object: wrongNameSub}))
	assert.True(t, pred.Delete(event.DeleteEvent{Object: otherNamespaceSub}))
	assert.True(t, pred.Delete(event.DeleteEvent{Object: packageMatchSub}))

	// Status-only update (generation and labels unchanged) should be ignored
	statusUpdateSub := validSub.DeepCopy()
	assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: validSub, ObjectNew: statusUpdateSub}))

	// Spec update (generation bumped) should trigger reconciliation
	specUpdateSub := validSub.DeepCopy()
	specUpdateSub.SetGeneration(2)
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: validSub, ObjectNew: specUpdateSub}))

	// Label update should trigger reconciliation
	labelUpdateSub := validSub.DeepCopy()
	labelUpdateSub.SetLabels(map[string]string{addoncfg.ReleaseLabelKey: "changed"})
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: validSub, ObjectNew: labelUpdateSub}))

	// Updates to wrong name should be ignored even if generation changed
	wrongNameUpdate := wrongNameSub.DeepCopy()
	wrongNameUpdate.SetGeneration(2)
	assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: wrongNameSub, ObjectNew: wrongNameUpdate}))
}
