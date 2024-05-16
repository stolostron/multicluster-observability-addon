package manifests

import (
	"encoding/json"
	"fmt"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

func Test_BuildSubscriptionChannel(t *testing.T) {
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
			subChannel: "stable-5.9",
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
			resources := Options{
				AddOnDeploymentConfig: adoc,
			}
			subChannel := buildSubscriptionChannel(resources)
			require.Equal(t, tc.subChannel, subChannel)
		})
	}
}

func Test_BuildSecrets(t *testing.T) {
	resources := Options{
		Secrets: []corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "cluster-1",
				},
				Data: map[string][]byte{
					"foo-1": []byte("foo-user"),
					"foo-2": []byte("foo-pass"),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: "cluster-1",
				},
				Data: map[string][]byte{
					"bar-1": []byte("bar-user"),
					"bar-2": []byte("bar-pass"),
				},
			},
		},
	}
	secretsValue, err := buildSecrets(resources)
	require.NoError(t, err)
	require.Equal(t, "foo", secretsValue[0].Name)
	require.Equal(t, "bar", secretsValue[1].Name)

	gotData := &map[string][]byte{}
	err = json.Unmarshal([]byte(secretsValue[0].Data), gotData)
	require.NoError(t, err)
	require.Equal(t, resources.Secrets[0].Data, *gotData)

	gotData = &map[string][]byte{}
	err = json.Unmarshal([]byte(secretsValue[1].Data), gotData)
	require.NoError(t, err)
	require.Equal(t, resources.Secrets[1].Data, *gotData)
}

func Test_BuildCLFSpec(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		clf                              *loggingv1.ClusterLogForwarder
		appLogsSecret, clusterLogsSecret *corev1.Secret

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
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "logging",
			},
			Annotations: map[string]string{
				AnnotationTargetOutputName: "app-logs",
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
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "logging",
			},
			Annotations: map[string]string{
				AnnotationTargetOutputName: "cluster-logs",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert"),
			"tls.key": []byte("key"),
		},
	}

	// Setup the fake k8s client
	resources := Options{
		ClusterLogForwarder: clf,
		Secrets: []corev1.Secret{
			*appLogsSecret,
			*clusterLogsSecret,
		},
	}
	clfSpec, err := buildClusterLogForwarderSpec(resources)
	require.NoError(t, err)
	require.NotNil(t, clfSpec.Outputs[0].Secret)
	require.NotNil(t, clfSpec.Outputs[1].Secret)
	require.Equal(t, appLogsSecret.Name, clfSpec.Outputs[0].Secret.Name)
	require.Equal(t, clusterLogsSecret.Name, clfSpec.Outputs[1].Secret.Name)
}

func Test_TemplateWithSecret(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		wrongTargetAnnotationValue bool
		secretName                 string
		expectedSecretName         string
	}{
		{
			name:                       "SignalAnnotationSet",
			wrongTargetAnnotationValue: false,
			secretName:                 "my-secret",
			expectedSecretName:         "my-secret",
		},
		{
			name:                       "SignalAnnotationNotSet",
			wrongTargetAnnotationValue: true,
			secretName:                 "my-secret",
			expectedSecretName:         "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.secretName,
					Namespace: "cluster-1",
					Annotations: map[string]string{
						"logging.mcoa.openshift.io/target-output-name": "foo",
					},
				},
			}

			if tc.wrongTargetAnnotationValue {
				secret.Annotations = map[string]string{
					"logging.mcoa.openshift.io/target-output-name": "bar",
				}
			}

			spec := &loggingv1.ClusterLogForwarderSpec{
				Outputs: []loggingv1.OutputSpec{
					{
						Name: "foo",
					},
				},
			}

			err := templateWithSecret(spec, *secret)
			assert.NoError(t, err)
			if tc.wrongTargetAnnotationValue {
				assert.Nil(t, spec.Outputs[0].Secret)
			} else {
				assert.NotNil(t, spec.Outputs[0].Secret)
				assert.Equal(t, tc.expectedSecretName, spec.Outputs[0].Secret.Name)
			}
		})
	}
}