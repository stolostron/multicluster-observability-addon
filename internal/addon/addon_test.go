package addon

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/require"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	workv1 "open-cluster-management.io/api/work/v1"
)

func Test_AgentHealthProber_PPA(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: %s status condition type is %s for %s/%s", errProbeConditionNotSatisfied, cooprometheusv1alpha1.PrometheusAgentName, "False", addonfactory.AddonDefaultInstallNamespace, mconfig.PlatformMetricsCollectorApp)
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
			healthProber := AgentHealthProber(logr.Discard())
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
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_CLF(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: %s status condition type is %s for %s/%s", errProbeConditionNotSatisfied, addoncfg.ClusterLogForwardersResource, "False", addoncfg.SpokeCLFNamespace, addoncfg.SpokeCLFName)
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
			healthProber := AgentHealthProber(logr.Discard())
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
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_OTELCol(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: %s replicas is %d for %s/%s", errProbeConditionNotSatisfied, addoncfg.OpenTelemetryCollectorsResource, 0, addoncfg.SpokeOTELColNamespace, addoncfg.SpokeOTELColName)
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
			healthProber := AgentHealthProber(logr.Discard())
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
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_UIPlugin(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: %s status condition type is %s for %s", errProbeConditionNotSatisfied, addoncfg.UiPluginsResource, "False", "monitoring")
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
			healthProber := AgentHealthProber(logr.Discard())
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
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
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
