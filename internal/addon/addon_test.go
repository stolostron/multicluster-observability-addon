package addon

import (
	"fmt"
	"testing"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/require"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	workv1 "open-cluster-management.io/api/work/v1"
)

func Test_AgentHealthProber_PPA(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: prometheusagents status condition type is %s for %s/%s", errProbeConditionNotSatisfied, "False", addoncfg.InstallNamespace, mconfig.PlatformMetricsCollectorApp)
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	for _, tc := range []struct {
		name        string
		status      string
		expectedErr string
	}{
		{
			name:   "healthy",
			status: "True",
		},
		{
			name:        "unhealthy",
			status:      "False",
			expectedErr: unhealthyError.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := AgentHealthProber()
			err := healthProber.WorkProber.HealthChecker(
				[]agent.FieldResult{
					{
						ResourceIdentifier: workv1.ResourceIdentifier{
							Group:     loggingv1.GroupVersion.Group,
							Resource:  prometheusalpha1.PrometheusAgentName,
							Name:      mconfig.PlatformMetricsCollectorApp,
							Namespace: addoncfg.InstallNamespace,
						},
						FeedbackResult: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name: "isAvailable",
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: &tc.status,
									},
								},
							},
						},
					},
				}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_CLF(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: clusterlogforwarders status condition type is %s for %s/%s", errProbeConditionNotSatisfied, "False", addoncfg.SpokeCLFNamespace, addoncfg.SpokeCLFName)
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	for _, tc := range []struct {
		name        string
		status      string
		expectedErr string
	}{
		{
			name:   "healthy",
			status: "True",
		},
		{
			name:        "unhealthy",
			status:      "False",
			expectedErr: unhealthyError.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := AgentHealthProber()
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
									Name: "isReady",
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: &tc.status,
									},
								},
							},
						},
					},
				}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_OTELCol(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: opentelemetrycollectors replicas is %d for %s/%s", errProbeConditionNotSatisfied, 0, addoncfg.SpokeOTELColNamespace, addoncfg.SpokeOTELColName)
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	for _, tc := range []struct {
		name        string
		replicas    int64
		expectedErr string
	}{
		{
			name:     "healthy",
			replicas: 1,
		},
		{
			name:        "unhealthy",
			replicas:    0,
			expectedErr: unhealthyError.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := AgentHealthProber()
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
								Name: "replicas",
								Value: workv1.FieldValue{
									Type:    workv1.Integer,
									Integer: &tc.replicas,
								},
							},
						},
					},
				},
			}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_UIPlugin(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: uiplugins status condition type is %s for %s", errProbeConditionNotSatisfied, "False", addoncfg.IDetectionUIPluginName)
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	for _, tc := range []struct {
		name        string
		status      string
		expectedErr string
	}{
		{
			name:   "healthy",
			status: "True",
		},
		{
			name:        "unhealthy",
			status:      "False",
			expectedErr: unhealthyError.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			healthProber := AgentHealthProber()
			err := healthProber.WorkProber.HealthChecker([]agent.FieldResult{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:    uiplugin.GroupVersion.Group,
						Resource: addoncfg.UiPluginsResource,
						Name:     addoncfg.IDetectionUIPluginName,
					},
					FeedbackResult: workv1.StatusFeedbackResult{
						Values: []workv1.FeedbackValue{
							{
								Name: "isAvailable",
								Value: workv1.FieldValue{
									Type:   workv1.String,
									String: &tc.status,
								},
							},
						},
					},
				},
			}, managedCluster, managedClusterAddOn)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
