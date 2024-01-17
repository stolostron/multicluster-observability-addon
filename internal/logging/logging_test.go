package logging

import (
	"fmt"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_Logging_BuildSubscriptionChannel(t *testing.T) {
	for _, tc := range []struct {
		name       string
		key        string
		value      string
		subChannel string
	}{
		{
			name:       "unknown key",
			key:        "test",
			value:      "stable-1.0",
			subChannel: "stable-5.8",
		},
		{
			name:       "known key",
			key:        "loggingSubscriptionChannel",
			value:      "stable-5.7",
			subChannel: "stable-5.7",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adoc := &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{
							Name:  tc.key,
							Value: tc.value,
						},
					},
				},
			}
			subChannel := buildSubscriptionChannel(adoc)
			require.Equal(t, tc.subChannel, subChannel)
		})
	}
}

func Test_BuildCLFSpec(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		clf                              *loggingv1.ClusterLogForwarder
		appLogsSecret, clusterLogsSecret *corev1.Secret
		appLogsCm                        *corev1.ConfigMap

		// Test clients
		fakeKubeClient client.Client

		clusterName = "cluster-1"
	)

	// Register the addon for the managed cluster
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "logging.openshift.io",
				Resource: "clusterlogforwarders",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management",
				Name:      "instance",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "secrets",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: clusterName,
				Name:      fmt.Sprintf("%s-app-logs", clusterName),
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "secrets",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: clusterName,
				Name:      fmt.Sprintf("%s-cluster-logs", clusterName),
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "configmaps",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: clusterName,
				Name:      fmt.Sprintf("%s-app-logs", clusterName),
			},
		},
	}

	// Setup configuration resources: ClusterLogForwarder, AddOnDeploymentConfig, Secrets, ConfigMaps
	clf = &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance",
			Namespace: "open-cluster-management",
		},
		Spec: loggingv1.ClusterLogForwarderSpec{
			Inputs: []loggingv1.InputSpec{
				{
					Name: "app-logs",
					Application: &loggingv1.Application{
						Namespaces: []string{"ns-1", "ns-2"},
					},
				},
				{
					Name:           "infra-logs",
					Infrastructure: &loggingv1.Infrastructure{},
				},
			},
			Outputs: []loggingv1.OutputSpec{
				{
					Name: "app-logs",
					Type: loggingv1.OutputTypeLoki,
					OutputTypeSpec: loggingv1.OutputTypeSpec{
						Loki: &loggingv1.Loki{
							LabelKeys: []string{"key-1", "key-2"},
							TenantKey: "tenant-x",
						},
					},
				},
				{
					Name: "cluster-logs",
					Type: loggingv1.OutputTypeCloudwatch,
					OutputTypeSpec: loggingv1.OutputTypeSpec{
						Cloudwatch: &loggingv1.Cloudwatch{
							GroupBy:     loggingv1.LogGroupByLogType,
							GroupPrefix: pointer.String("test-prefix"),
						},
					},
				},
			},
			Pipelines: []loggingv1.PipelineSpec{
				{
					Name:       "app-logs",
					InputRefs:  []string{"app-logs"},
					OutputRefs: []string{"app-logs"},
				},
				{
					Name:       "cluster-logs",
					InputRefs:  []string{"infra-logs", loggingv1.InputNameAudit},
					OutputRefs: []string{"cluster-logs"},
				},
			},
		},
	}

	appLogsSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-app-logs", clusterName),
			Namespace: clusterName,
			Annotations: map[string]string{
				annotationTargetOutputName: "app-logs",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert"),
			"tls.key": []byte("key"),
		},
	}

	clusterLogsSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-cluster-logs", clusterName),
			Namespace: clusterName,
			Annotations: map[string]string{
				annotationTargetOutputName: "cluster-logs",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert"),
			"tls.key": []byte("key"),
		},
	}

	appLogsCm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-app-logs", clusterName),
			Namespace: clusterName,
			Annotations: map[string]string{
				annotationTargetOutputName: "app-logs",
			},
		},
		Data: map[string]string{
			"url": "https://example.com",
		},
	}

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(clf, appLogsCm, appLogsSecret, clusterLogsSecret).
		Build()

	clfSpec, err := buildClusterLogForwarderSpec(fakeKubeClient, managedClusterAddOn)
	require.NoError(t, err)
	require.NotNil(t, clfSpec.Outputs[0].Secret)
	require.NotNil(t, clfSpec.Outputs[1].Secret)
	require.Equal(t, appLogsSecret.Name, clfSpec.Outputs[0].Secret.Name)
	require.Equal(t, clusterLogsSecret.Name, clfSpec.Outputs[1].Secret.Name)
	require.Equal(t, appLogsCm.Data["url"], clfSpec.Outputs[0].URL)

}

func Test_TemplateWithConfigMap(t *testing.T) {
	configReference := addonapiv1alpha1.ConfigReference{
		ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
			Group:    "",
			Resource: "configmap",
		},
		ConfigReferent: addonapiv1alpha1.ConfigReferent{
			Namespace: "cluster-1",
			Name:      "cluster-1",
		},
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-1",
			Namespace: "cluster-1",
			Annotations: map[string]string{
				"logging.mcoa.openshift.io/target-output-name": "foo",
			},
		},
		Data: map[string]string{
			"url": "http://foo.bar",
		},
	}
	client := fake.NewClientBuilder().
		WithObjects(cm).
		Build()

	spec := &loggingv1.ClusterLogForwarderSpec{
		Outputs: []loggingv1.OutputSpec{
			{
				Name: "foo",
			},
		},
	}

	err := templateWithConfigMap(client, spec, configReference)
	assert.NoError(t, err, "Expected no error")
	assert.Equal(t, "http://foo.bar", spec.Outputs[0].URL)
}

func Test_TemplateWithSecret(t *testing.T) {
	configReference := addonapiv1alpha1.ConfigReference{
		ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
			Group:    "",
			Resource: "secret",
		},
		ConfigReferent: addonapiv1alpha1.ConfigReferent{
			Namespace: "cluster-1",
			Name:      "cluster-1",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-1",
			Namespace: "cluster-1",
			Annotations: map[string]string{
				"logging.mcoa.openshift.io/target-output-name": "foo",
			},
		},
	}
	client := fake.NewClientBuilder().
		WithObjects(secret).
		Build()

	spec := &loggingv1.ClusterLogForwarderSpec{
		Outputs: []loggingv1.OutputSpec{
			{
				Name: "foo",
			},
		},
	}

	err := templateWithSecret(client, spec, configReference)
	assert.NoError(t, err)
	assert.NotNil(t, spec.Outputs[0].Secret)
	assert.Equal(t, "cluster-1", spec.Outputs[0].Secret.Name)
}
