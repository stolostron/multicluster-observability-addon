package handlers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testClusterName = "spoke-1"

var _ = operatorv1alpha1.AddToScheme(scheme.Scheme)
var _ = workv1.Install(scheme.Scheme)

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

			result, err := InstallOfCOOOnTheHubIsNeeded(context.Background(), k8sClientBuilder.Build(), logr.Discard(), tc.isHub)
			cooValues := manifests.BuildValues(tc.options, result, tc.isHub, false)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedUIPluginInstall, cooValues.Enabled)
			assert.Equal(t, tc.expectedCOOInstall, cooValues.InstallCOO)
		})
	}
}

// manifestWorkWithNoFeedback builds a ManifestWork for the addon that hasn't reported any
// status feedback yet, mimicking the very first reconcile(s) for a cluster before the work
// agent has observed anything on the spoke.
func manifestWorkWithNoFeedback(name string) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testClusterName,
			Labels:    map[string]string{addonapiv1beta1.AddonLabelKey: addoncfg.Name},
		},
	}
}

// manifestWorkWithCRDFeedback builds a ManifestWork carrying status feedback for the
// alertmanagers.monitoring.rhobs CRD, mimicking what the work agent reports back once it
// has observed the CRD on the spoke.
func manifestWorkWithCRDFeedback(name string, olmManaged string) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testClusterName,
			Labels:    map[string]string{addonapiv1beta1.AddonLabelKey: addoncfg.Name},
		},
		Status: workv1.ManifestWorkStatus{
			ResourceStatus: workv1.ManifestResourceStatus{
				Manifests: []workv1.ManifestCondition{
					{
						ResourceMeta: workv1.ManifestResourceMeta{
							Group:    apiextensionsv1.GroupName,
							Resource: "customresourcedefinitions",
							Name:     mconfig.AlertmanagerCRDName,
						},
						StatusFeedbacks: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name:  addoncfg.IsOLMManagedFeedbackName,
									Value: workv1.FieldValue{Type: workv1.String, String: &olmManaged},
								},
							},
						},
					},
				},
			},
		},
	}
}

// manifestWorkWithCommittedSubscription builds a ManifestWork whose spec already renders
// the COO Subscription manifest, mimicking a previous reconcile where MCOA committed to
// installing COO on the spoke.
func manifestWorkWithCommittedSubscription(name string) *workv1.ManifestWork {
	sub := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.coreos.com/v1alpha1",
			"kind":       "Subscription",
			"metadata": map[string]interface{}{
				"name":      addoncfg.CooSubscriptionName,
				"namespace": addoncfg.CooSubscriptionNamespace,
			},
		},
	}

	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testClusterName,
			Labels:    map[string]string{addonapiv1beta1.AddonLabelKey: addoncfg.Name},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{RawExtension: runtime.RawExtension{Object: sub}},
				},
			},
		},
	}
}

func TestInstallOfCOOOnSpokeIsNeeded(t *testing.T) {
	tests := []struct {
		name            string
		objects         []client.Object
		expectedInstall bool
	}{
		{
			name:            "no manifestwork yet: bootstrap, defer decision",
			objects:         nil,
			expectedInstall: false,
		},
		{
			name:            "manifestwork exists but no status feedback yet: defer decision",
			objects:         []client.Object{manifestWorkWithNoFeedback("addon-deploy-0")},
			expectedInstall: false,
		},
		{
			name:            "COO already OLM-managed on spoke: don't install our own",
			objects:         []client.Object{manifestWorkWithCRDFeedback("addon-deploy-0", "True")},
			expectedInstall: false,
		},
		{
			name:            "COO CRD reported but not OLM-managed: safe to install",
			objects:         []client.Object{manifestWorkWithCRDFeedback("addon-deploy-0", "False")},
			expectedInstall: true,
		},
		{
			name:            "previously committed to install: sticky, keep installing even though CRD now looks OLM-managed",
			objects:         []client.Object{manifestWorkWithCommittedSubscription("addon-deploy-0")},
			expectedInstall: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.objects...).
				Build()

			result, err := InstallOfCOOOnSpokeIsNeeded(context.Background(), k8sClient, logr.Discard(), testClusterName)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedInstall, result)
		})
	}
}
