package handlers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testClusterName = "spoke-1"

var (
	_ = operatorv1alpha1.AddToScheme(scheme.Scheme)
	_ = workv1.Install(scheme.Scheme)
)

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

			var result bool
			var err error
			if tc.isHub {
				result, err = InstallOfCOOOnTheHubIsNeeded(context.Background(), k8sClientBuilder.Build(), logr.Discard())
			}
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

// manifestWorkWithCOOStatusFeedback builds a ManifestWork carrying status feedback for the
// coo-status ConfigMap, mimicking what the work agent reports back once the endpoint-monitoring-operator
// has written the COO detection result.
func manifestWorkWithCOOStatusFeedback(name string, installed string) *workv1.ManifestWork {
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
							Group:    "",
							Resource: "configmaps",
							Name:     addoncfg.CooStatusConfigMapName,
						},
						Conditions: []metav1.Condition{
							{
								Type:               workv1.WorkAvailable,
								Status:             metav1.ConditionTrue,
								Reason:             "ResourceAvailable",
								LastTransitionTime: metav1.Now(),
							},
						},
						StatusFeedbacks: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name:  addoncfg.CooStatusInstalledFeedbackName,
									Value: workv1.FieldValue{Type: workv1.String, String: &installed},
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

func TestInstallOfCOOOnSpokeIsNeeded(t *testing.T) {
	tests := []struct {
		name            string
		objects         []client.Object
		expectedInstall bool
	}{
		{
			name:            "no manifestwork yet: defer until endpoint operator reports",
			objects:         nil,
			expectedInstall: false,
		},
		{
			name:            "manifestwork exists but no status feedback yet: defer until endpoint operator reports",
			objects:         []client.Object{manifestWorkWithNoFeedback("addon-deploy-0")},
			expectedInstall: false,
		},
		{
			name:            "COO status reports installed: don't install our own",
			objects:         []client.Object{manifestWorkWithCOOStatusFeedback("addon-deploy-0", "true")},
			expectedInstall: false,
		},
		{
			name:            "COO status reports not installed: safe to install",
			objects:         []client.Object{manifestWorkWithCOOStatusFeedback("addon-deploy-0", "false")},
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
