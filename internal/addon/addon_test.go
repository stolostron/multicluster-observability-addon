package addon

import (
	"fmt"
	"testing"

	//"github.com/openshift/cluster-logging-operator/internal/status"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	v1 "open-cluster-management.io/api/work/v1"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingapis "github.com/openshift/cluster-logging-operator/apis"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
)

var (
	_ = loggingapis.AddToScheme(scheme.Scheme)
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
)

func Test_AgentHealthProber_Healthy(t *testing.T) {
	otelcol := &otelv1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OtelcolName,
			Namespace: OtelcolNS,
		},
		Spec: otelv1alpha1.OpenTelemetryCollectorSpec{
			Replicas: ptr.To[int32](1),
		},
	}

	healthProber := AgentHealthProber()

	err := healthProber.WorkProber.HealthCheck(v1.ResourceIdentifier{
		Group:     otelcol.APIVersion,
		Resource:  OtelcolResource,
		Name:      otelcol.Name,
		Namespace: otelcol.Namespace,
	}, v1.StatusFeedbackResult{
		Values: []v1.FeedbackValue{
			{
				Name: "replicas",
				Value: v1.FieldValue{
					Type:    v1.Integer,
					Integer: ptr.To[int64](1),
				},
			},
		},
	})

	require.NoError(t, err)
}

func Test_AgentHealthProber_Unhealthy(t *testing.T) {
	otelcol := &otelv1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OtelcolName,
			Namespace: OtelcolNS,
		},
		Spec: otelv1alpha1.OpenTelemetryCollectorSpec{
			Replicas: ptr.To[int32](0),
		},
	}
	healthProber := AgentHealthProber()

	err := healthProber.WorkProber.HealthCheck(v1.ResourceIdentifier{
		Group:     otelcol.APIVersion,
		Resource:  OtelcolResource,
		Name:      otelcol.Name,
		Namespace: otelcol.Namespace,
	}, v1.StatusFeedbackResult{
		Values: []v1.FeedbackValue{
			{
				Name: "replicas",
				Value: v1.FieldValue{
					Type:    v1.Integer,
					Integer: ptr.To[int64](0),
				},
			},
		},
	})

	expectedErr := fmt.Errorf("%w: replicas is %d for %s/%s", errUnavailable, 0, otelcol.Namespace, otelcol.Name)
	require.EqualError(t, err, expectedErr.Error())
}
