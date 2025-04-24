package addon

import (
	"fmt"
	"testing"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stretchr/testify/require"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	workv1 "open-cluster-management.io/api/work/v1"
)

func Test_AgentHealthProber_PPA(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: prometheusagents status condition type is %s for %s/%s", errProbeConditionNotSatisfied, "False", InstallNamespace, PPAName)
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
							Resource:  PrometheusAgentResource,
							Name:      PPAName,
							Namespace: InstallNamespace,
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
	unhealthyError := fmt.Errorf("%w: clusterlogforwarders status condition type is %s for %s/%s", errProbeConditionNotSatisfied, "False", LoggingNamespace, SpokeUnmanagedCLFName)
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
							Resource:  ClusterLogForwardersResource,
							Name:      SpokeUnmanagedCLFName,
							Namespace: LoggingNamespace,
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
	unhealthyError := fmt.Errorf("%w: opentelemetrycollectors replicas is %d for %s/%s", errProbeConditionNotSatisfied, 0, SpokeOTELColNamespace, SpokeOTELColName)
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
						Resource:  OpenTelemetryCollectorsResource,
						Name:      SpokeOTELColName,
						Namespace: SpokeOTELColNamespace,
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
