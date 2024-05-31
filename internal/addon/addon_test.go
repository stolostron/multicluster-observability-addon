package addon

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "open-cluster-management.io/api/work/v1"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
)

func Test_AgentHealthProber_CLF(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: clusterlogforwarder status condition type is %s for %s/%s", errUnavailable, "False", SpokeCLFNamespace, SpokeCLFName)
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
			err := healthProber.WorkProber.HealthCheck(v1.ResourceIdentifier{
				Group:     loggingv1.GroupVersion.Group,
				Resource:  ClusterLogForwardersResource,
				Name:      SpokeCLFName,
				Namespace: SpokeCLFNamespace,
			}, v1.StatusFeedbackResult{
				Values: []v1.FeedbackValue{
					{
						Name: "isReady",
						Value: v1.FieldValue{
							Type:   v1.String,
							String: &tc.status,
						},
					},
				},
			})
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_AgentHealthProber_OTELCol(t *testing.T) {
	unhealthyError := fmt.Errorf("%w: opentelemetrycollector replicas is %d for %s/%s", errUnavailable, 0, SpokeOTELColNamespace, SpokeOTELColName)
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
			err := healthProber.WorkProber.HealthCheck(v1.ResourceIdentifier{
				Group:     otelv1alpha1.GroupVersion.Group,
				Resource:  OpenTelemetryCollectorsResource,
				Name:      SpokeOTELColName,
				Namespace: SpokeOTELColNamespace,
			}, v1.StatusFeedbackResult{
				Values: []v1.FeedbackValue{
					{
						Name: "replicas",
						Value: v1.FieldValue{
							Type:    v1.Integer,
							Integer: &tc.replicas,
						},
					},
				},
			})
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
