package addon

import (
	"testing"

	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_AgentHealthProber_PPA(t *testing.T) {
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	aodc := newAddonDeploymentConfig()
	addPlatformMetricsCustomizedVariables(aodc)
	addAODCConfigReference(managedClusterAddOn, aodc)
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	for _, tc := range []struct {
		name        string
		status      string
		expectedErr error
	}{
		{
			name:   "healthy",
			status: "True",
		},
		{
			name:        "unhealthy",
			status:      "False",
			expectedErr: errProbeConditionNotSatisfied,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := HealthProber(fake.NewClientBuilder().WithScheme(scheme).WithObjects(aodc).Build(), logr.Discard())
			err := healthProber.WorkProber.HealthChecker(
				[]agent.FieldResult{
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Group:     cooprometheusv1alpha1.SchemeGroupVersion.Group,
							Resource:  cooprometheusv1alpha1.PrometheusAgentName,
							Name:      mconfig.PlatformMetricsCollectorApp,
							Namespace: addonfactory.AddonDefaultInstallNamespace,
						},
						FeedbackResult: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name: addoncfg.PaProbeKey,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: &tc.status,
									},
								},
							},
						},
					},
				}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_PPA_UserWorkload(t *testing.T) {
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedCluster.Labels = map[string]string{"vendor": "OpenShift"}
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	aodc := newAddonDeploymentConfig()
	addUserWorkloadMetricsCustomizedVariables(aodc)
	addAODCConfigReference(managedClusterAddOn, aodc)
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	for _, tc := range []struct {
		name        string
		status      string
		isOCP       bool
		expectedErr error
	}{
		{
			name:   "healthy OCP",
			status: "True",
			isOCP:  true,
		},
		{
			name:        "unhealthy OCP",
			status:      "False",
			isOCP:       true,
			expectedErr: errProbeConditionNotSatisfied,
		},
		{
			name:   "unhealthy non-OCP",
			status: "False",
			isOCP:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.isOCP {
				managedCluster.Labels["vendor"] = "OpenShift"
			} else {
				managedCluster.Labels["vendor"] = "Other"
			}
			healthProber := HealthProber(fake.NewClientBuilder().WithScheme(scheme).WithObjects(aodc).Build(), logr.Discard())
			err := healthProber.WorkProber.HealthChecker(
				[]agent.FieldResult{
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Group:     cooprometheusv1alpha1.SchemeGroupVersion.Group,
							Resource:  cooprometheusv1alpha1.PrometheusAgentName,
							Name:      mconfig.UserWorkloadMetricsCollectorApp,
							Namespace: addonfactory.AddonDefaultInstallNamespace,
						},
						FeedbackResult: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name: addoncfg.PaProbeKey,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: &tc.status,
									},
								},
							},
						},
					},
				}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_CLF(t *testing.T) {
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	aodc := newAddonDeploymentConfig()
	addLoggingCustomizedVariables(aodc)
	addAODCConfigReference(managedClusterAddOn, aodc)
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	for _, tc := range []struct {
		name        string
		status      string
		expectedErr error
	}{
		{
			name:   "healthy",
			status: "True",
		},
		{
			name:        "unhealthy",
			status:      "False",
			expectedErr: errProbeConditionNotSatisfied,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := HealthProber(fake.NewClientBuilder().WithScheme(scheme).WithObjects(aodc).Build(), logr.Discard())
			err := healthProber.WorkProber.HealthChecker(
				[]agent.FieldResult{
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Group:     loggingv1.GroupVersion.Group,
							Resource:  addoncfg.ClusterLogForwardersResource,
							Name:      addoncfg.SpokeCLFName,
							Namespace: addoncfg.SpokeCLFNamespace,
						},
						FeedbackResult: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name: addoncfg.ClfProbeKey,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: &tc.status,
									},
								},
							},
						},
					},
				}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_OTELCol(t *testing.T) {
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	aodc := newAddonDeploymentConfig()
	addTracingCustomizedVariables(aodc)
	addAODCConfigReference(managedClusterAddOn, aodc)
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	for _, tc := range []struct {
		name        string
		replicas    int64
		expectedErr error
	}{
		{
			name:     "healthy",
			replicas: 1,
		},
		{
			name:        "unhealthy",
			replicas:    0,
			expectedErr: errProbeConditionNotSatisfied,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := HealthProber(fake.NewClientBuilder().WithScheme(scheme).WithObjects(aodc).Build(), logr.Discard())
			err := healthProber.WorkProber.HealthChecker([]agent.FieldResult{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     otelv1alpha1.GroupVersion.Group,
						Resource:  addoncfg.OpenTelemetryCollectorsResource,
						Name:      addoncfg.SpokeOTELColName,
						Namespace: addoncfg.SpokeOTELColNamespace,
					},
					FeedbackResult: workv1.StatusFeedbackResult{
						Values: []workv1.FeedbackValue{
							{
								Name: addoncfg.OtelColProbeKey,
								Value: workv1.FieldValue{
									Type:    workv1.Integer,
									Integer: &tc.replicas,
								},
							},
						},
					},
				},
			}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_UIPlugin(t *testing.T) {
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	aodc := newAddonDeploymentConfig()
	addUIPluginCustomizedVariables(aodc)
	addPlatformMetricsCustomizedVariables(aodc)
	addAODCConfigReference(managedClusterAddOn, aodc)
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	for _, tc := range []struct {
		name        string
		status      string
		expectedErr error
	}{
		{
			name:   "healthy",
			status: "True",
		},
		{
			name:        "unhealthy",
			status:      "False",
			expectedErr: errProbeConditionNotSatisfied,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := HealthProber(fake.NewClientBuilder().WithScheme(scheme).WithObjects(aodc).Build(), logr.Discard())
			err := healthProber.WorkProber.HealthChecker([]agent.FieldResult{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:    uiplugin.GroupVersion.Group,
						Resource: addoncfg.UiPluginsResource,
						Name:     "monitoring",
					},
					FeedbackResult: workv1.StatusFeedbackResult{
						Values: []workv1.FeedbackValue{
							{
								Name: addoncfg.UipProbeKey,
								Value: workv1.FieldValue{
									Type:   workv1.String,
									String: &tc.status,
								},
							},
						},
					},
				},
			}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestIsVersionOlder(t *testing.T) {
	testCases := []struct {
		name        string
		v1          string
		v2          string
		expected    bool
		expectErr   bool
		expectedErr string
	}{
		{
			name:     "v1 is older",
			v1:       "0.78.0",
			v2:       "0.79.0",
			expected: true,
		},
		{
			name:     "v1 is newer",
			v1:       "0.80.0",
			v2:       "0.79.0",
			expected: false,
		},
		{
			name:     "versions are equal",
			v1:       "0.79.0",
			v2:       "0.79.0",
			expected: false,
		},
		{
			name:     "v1 has fewer parts and is older",
			v1:       "0.78",
			v2:       "0.79.0",
			expected: true,
		},
		{
			name:     "v2 has fewer parts and v1 is older",
			v1:       "0.78.1",
			v2:       "0.79",
			expected: true,
		},
		{
			name:     "versions are equal with different parts",
			v1:       "0.79",
			v2:       "0.79.0",
			expected: false,
		},
		{
			name:     "v1 is older with suffix",
			v1:       "0.78.0-rhobs1",
			v2:       "0.79.0",
			expected: true,
		},
		{
			name:     "v1 is newer with suffix",
			v1:       "0.80.0-rhobs1",
			v2:       "0.79.0",
			expected: false,
		},
		{
			name:     "versions are equal, v1 with suffix",
			v1:       "0.79.0-rhobs1",
			v2:       "0.79.0",
			expected: false,
		},
		{
			name:     "versions are equal, v2 with suffix",
			v1:       "0.79.0",
			v2:       "0.79.0-rhobs1",
			expected: false,
		},
		{
			name:     "versions are equal, both with suffix",
			v1:       "0.79.0-rhobs1",
			v2:       "0.79.0-rhobs2",
			expected: false,
		},
		{
			name:        "invalid v1",
			v1:          "a.b.c",
			v2:          "0.79.0",
			expectErr:   true,
			expectedErr: `invalid version string: a.b.c`,
		},
		{
			name:        "invalid v2",
			v1:          "0.79.0",
			v2:          "a.b.c",
			expectErr:   true,
			expectedErr: `invalid version string: a.b.c`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isOlder, err := isVersionOlder(tc.v1, tc.v2)
			if tc.expectErr {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, isOlder)
			}
		})
	}
}

func newAddonDeploymentConfig() *addonapiv1alpha1.AddOnDeploymentConfig {
	return &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
	}
}

func addPlatformMetricsCustomizedVariables(aodc *addonapiv1alpha1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1alpha1.CustomizedVariable{
		{
			Name:  KeyPlatformMetricsCollection,
			Value: string(PrometheusAgentV1alpha1),
		},
		{
			Name:  KeyMetricsHubHostname,
			Value: "https://the-hub.com",
		},
	}...)
}

func addLoggingCustomizedVariables(aodc *addonapiv1alpha1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1alpha1.CustomizedVariable{
		{
			Name:  KeyPlatformLogsCollection,
			Value: string(ClusterLogForwarderV1),
		},
	}...)
}

func addTracingCustomizedVariables(aodc *addonapiv1alpha1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1alpha1.CustomizedVariable{
		{
			Name:  KeyUserWorkloadTracesCollection,
			Value: string(OpenTelemetryCollectorV1beta1),
		},
	}...)
}

func addUIPluginCustomizedVariables(aodc *addonapiv1alpha1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1alpha1.CustomizedVariable{
		{
			Name:  KeyPlatformMetricsUI,
			Value: string(UIPluginV1alpha1),
		},
	}...)
}

func addAODCConfigReference(managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn, aodc *addonapiv1alpha1.AddOnDeploymentConfig) {
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    addonutils.AddOnDeploymentConfigGVR.Group,
				Resource: addoncfg.AddonDeploymentConfigResource,
			},
			DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
				ConfigReferent: addonapiv1alpha1.ConfigReferent{
					Namespace: aodc.Namespace,
					Name:      aodc.Name,
				},
			},
		},
	}
}

func addUserWorkloadMetricsCustomizedVariables(aodc *addonapiv1alpha1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1alpha1.CustomizedVariable{
		{
			Name:  KeyUserWorkloadMetricsCollection,
			Value: string(PrometheusAgentV1alpha1),
		},
		{
			Name:  KeyMetricsHubHostname,
			Value: "https://the-hub.com",
		},
	}...)
}
