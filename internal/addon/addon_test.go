package addon

import (
	"testing"

	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	clusterlifecycleconstants "github.com/stolostron/cluster-lifecycle-api/constants"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	workv1 "open-cluster-management.io/api/work/v1"
)

func newTestGetter(aodc *addonapiv1beta1.AddOnDeploymentConfig) addonutils.AddOnDeploymentConfigGetter {
	if aodc == nil {
		//nolint:staticcheck // client.Apply is deprecated, but alternative requires ApplyConfigurations which we don't have
		return addonutils.NewAddOnDeploymentConfigGetter(fakeaddon.NewSimpleClientset())
	}
	//nolint:staticcheck // client.Apply is deprecated, but alternative requires ApplyConfigurations which we don't have
	return addonutils.NewAddOnDeploymentConfigGetter(fakeaddon.NewSimpleClientset(aodc))
}

func Test_AgentHealthProber_PPA(t *testing.T) {
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	aodc := newAddonDeploymentConfig()
	addPlatformMetricsCustomizedVariables(aodc)
	addAODCConfigReference(managedClusterAddOn, aodc)
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))

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
			healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
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
					scrapeConfigFieldResult(),
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
	require.NoError(t, addonapiv1beta1.Install(scheme))

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
			healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
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
					scrapeConfigFieldResult(),
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
	require.NoError(t, addonapiv1beta1.Install(scheme))

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
			healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
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
	require.NoError(t, addonapiv1beta1.Install(scheme))

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
			healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
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
	require.NoError(t, addonapiv1beta1.Install(scheme))

	for _, tc := range []struct {
		name        string
		status      string
		isHub       bool
		expectedErr error
	}{
		{
			name:   "healthy on hub",
			status: "True",
			isHub:  true,
		},
		{
			name:        "unhealthy on hub",
			status:      "False",
			isHub:       true,
			expectedErr: errProbeConditionNotSatisfied,
		},
		{
			name:   "ignored on spoke",
			status: "False",
			isHub:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.isHub {
				managedCluster.Labels = map[string]string{clusterlifecycleconstants.SelfManagedClusterLabelKey: "true"}
			} else {
				managedCluster.Labels = map[string]string{}
			}

			healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
			metricsStatus := "True"
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
									String: &metricsStatus,
								},
							},
						},
					},
				},
				scrapeConfigFieldResult(),
			}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_MissingResources(t *testing.T) {
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))

	t.Run("metrics enabled but missing prometheus agent", func(t *testing.T) {
		aodc := newAddonDeploymentConfig()
		addPlatformMetricsCustomizedVariables(aodc)
		addAODCConfigReference(managedClusterAddOn, aodc)

		healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
		err := healthProber.WorkProber.HealthChecker(
			[]agent.FieldResult{scrapeConfigFieldResult()}, // Missing PPA
			managedCluster, managedClusterAddOn)
		require.ErrorIs(t, err, errMissingFields)
	})

	t.Run("logging enabled but missing clf", func(t *testing.T) {
		aodc := newAddonDeploymentConfig()
		addLoggingCustomizedVariables(aodc)
		addAODCConfigReference(managedClusterAddOn, aodc)

		healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
		err := healthProber.WorkProber.HealthChecker(
			// We pass an unrelated field to bypass the initial check for empty fields
			// in the healthChecker function.
			[]agent.FieldResult{scrapeConfigFieldResult()}, // unrelated field to bypass empty check
			managedCluster, managedClusterAddOn)
		require.ErrorIs(t, err, errMissingFields)
	})

	t.Run("tracing enabled but missing otel collector", func(t *testing.T) {
		aodc := newAddonDeploymentConfig()
		addTracingCustomizedVariables(aodc)
		addAODCConfigReference(managedClusterAddOn, aodc)

		healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
		err := healthProber.WorkProber.HealthChecker(
			// We pass an unrelated field to bypass the initial check for empty fields
			// in the healthChecker function.
			[]agent.FieldResult{scrapeConfigFieldResult()}, // unrelated field
			managedCluster, managedClusterAddOn)
		require.ErrorIs(t, err, errMissingFields)
	})

	t.Run("ui plugin enabled on hub but missing resource", func(t *testing.T) {
		managedCluster.Labels = map[string]string{clusterlifecycleconstants.SelfManagedClusterLabelKey: "true"}
		defer func() { managedCluster.Labels = nil }()

		aodc := newAddonDeploymentConfig()
		addPlatformMetricsCustomizedVariables(aodc)
		addUIPluginCustomizedVariables(aodc)
		addAODCConfigReference(managedClusterAddOn, aodc)

		healthProber := HealthProber(newTestGetter(aodc), logr.Discard())
		metricsStatus := "True"
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
									String: &metricsStatus,
								},
							},
						},
					},
				},
				scrapeConfigFieldResult(),
			}, managedCluster, managedClusterAddOn)
		require.ErrorIs(t, err, errMissingFields)
		require.Contains(t, err.Error(), addoncfg.UiPluginsResource)
	})
}

func scrapeConfigFieldResult() agent.FieldResult {
	version := "0.79.0"
	isEstablished := "True"
	return agent.FieldResult{
		ResourceIdentifier: workv1.ResourceIdentifier{
			Group:    "apiextensions.k8s.io",
			Resource: "customresourcedefinitions",
			Name:     "scrapeconfigs.monitoring.rhobs",
		},
		FeedbackResult: workv1.StatusFeedbackResult{
			Values: []workv1.FeedbackValue{
				{
					Name: addoncfg.PrometheusOperatorVersionFeedbackName,
					Value: workv1.FieldValue{
						Type:   workv1.String,
						String: &version,
					},
				},
				{
					Name: addoncfg.IsEstablishedFeedbackName,
					Value: workv1.FieldValue{
						Type:   workv1.String,
						String: &isEstablished,
					},
				},
			},
		},
	}
}

func newAddonDeploymentConfig() *addonapiv1beta1.AddOnDeploymentConfig {
	return &addonapiv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
	}
}

func addPlatformMetricsCustomizedVariables(aodc *addonapiv1beta1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1beta1.CustomizedVariable{
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

func addLoggingCustomizedVariables(aodc *addonapiv1beta1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1beta1.CustomizedVariable{
		{
			Name:  KeyPlatformLogsCollection,
			Value: string(ClusterLogForwarderV1),
		},
	}...)
}

func addTracingCustomizedVariables(aodc *addonapiv1beta1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1beta1.CustomizedVariable{
		{
			Name:  KeyUserWorkloadTracesCollection,
			Value: string(OpenTelemetryCollectorV1beta1),
		},
	}...)
}

func addUIPluginCustomizedVariables(aodc *addonapiv1beta1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1beta1.CustomizedVariable{
		{
			Name:  KeyPlatformMetricsUI,
			Value: string(UIPluginV1alpha1),
		},
	}...)
}

func addAODCConfigReference(managedClusterAddOn *addonapiv1beta1.ManagedClusterAddOn, aodc *addonapiv1beta1.AddOnDeploymentConfig) {
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1beta1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
				Group:    addonutils.AddOnDeploymentConfigGVR.Group,
				Resource: addoncfg.AddonDeploymentConfigResource,
			},
			DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
				ConfigReferent: addonapiv1beta1.ConfigReferent{
					Namespace: aodc.Namespace,
					Name:      aodc.Name,
				},
			},
		},
	}
}

func addUserWorkloadMetricsCustomizedVariables(aodc *addonapiv1beta1.AddOnDeploymentConfig) {
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, []addonapiv1beta1.CustomizedVariable{
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
