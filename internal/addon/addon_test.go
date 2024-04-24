package addon

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "open-cluster-management.io/api/work/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_AgentHealthProber_Healthy(t *testing.T) {
	fakeKubeClient := fake.NewClientBuilder().Build()
	colDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spoke-otelcol-collector",
			Namespace: "spoke-otelcol",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}

	err := fakeKubeClient.Create(context.TODO(), colDeployment, &client.CreateOptions{})
	require.NoError(t, err)

	readyReplicas := int64(colDeployment.Status.ReadyReplicas)

	healthProber := AgentHealthProber()

	err = healthProber.WorkProber.HealthCheck(v1.ResourceIdentifier{
		Group:     colDeployment.APIVersion,
		Resource:  colDeployment.Kind,
		Name:      colDeployment.Name,
		Namespace: colDeployment.Namespace,
	}, v1.StatusFeedbackResult{
		Values: []v1.FeedbackValue{
			{
				Name: "readyReplicas",
				Value: v1.FieldValue{
					Type:    v1.Integer,
					Integer: &readyReplicas,
				},
			},
		},
	})

	require.NoError(t, err)

}

func Test_AgentHealthProber(t *testing.T) {

	fakeKubeClient := fake.NewClientBuilder().Build()

	cloDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-logging-operator",
			Namespace: "openshift-logging",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}

	err := fakeKubeClient.Create(context.TODO(), cloDeployment, &client.CreateOptions{})
	require.NoError(t, err)

	readyReplicas := int64(cloDeployment.Status.ReadyReplicas)

	healthProber := AgentHealthProber()

	err = healthProber.WorkProber.HealthCheck(v1.ResourceIdentifier{
		Group:     cloDeployment.APIVersion,
		Resource:  cloDeployment.Kind,
		Name:      cloDeployment.Name,
		Namespace: cloDeployment.Namespace,
	}, v1.StatusFeedbackResult{
		Values: []v1.FeedbackValue{
			{
				Name: "readyReplicas",
				Value: v1.FieldValue{
					Type:    v1.Integer,
					Integer: &readyReplicas,
				},
			},
		},
	})

	expectedErr := fmt.Errorf("readyReplica is %d for deployement %s/%s", readyReplicas, cloDeployment.Namespace, cloDeployment.Name)
	require.EqualError(t, err, expectedErr.Error())

}
