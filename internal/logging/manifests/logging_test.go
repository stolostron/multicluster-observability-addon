package manifests

import (
	"encoding/json"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
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
			name:       "not set",
			subChannel: "stable-5.9",
		},
		{
			name:       "user set",
			value:      "stable-5.7",
			subChannel: "stable-5.7",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resources := Options{
				SubscriptionChannel: tc.value,
			}
			subChannel := buildSubscriptionChannel(resources)
			require.Equal(t, tc.subChannel, subChannel)
		})
	}
}

func Test_BuildSecrets(t *testing.T) {
	resources := Options{
		Secrets: map[addon.Endpoint]corev1.Secret{
			"foo": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "cluster-1",
				},
				Data: map[string][]byte{
					"foo-1": []byte("foo-user"),
					"foo-2": []byte("foo-pass"),
				},
			},
			"bar": {
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
	require.Equal(t, "bar", secretsValue[0].Name)
	require.Equal(t, "foo", secretsValue[1].Name)

	gotData := &map[string][]byte{}
	err = json.Unmarshal([]byte(secretsValue[0].Data), gotData)
	require.NoError(t, err)
	require.Equal(t, resources.Secrets["bar"].Data, *gotData)

	gotData = &map[string][]byte{}
	err = json.Unmarshal([]byte(secretsValue[1].Data), gotData)
	require.NoError(t, err)
	require.Equal(t, resources.Secrets["foo"].Data, *gotData)
}

func Test_BuildCLFSpec(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		clf *loggingv1.ClusterLogForwarder
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
				Name:      "mcoa-instance",
			},
		},
	}

	// Setup configuration resources: ClusterLogForwarder, AddOnDeploymentConfig, Secrets, ConfigMaps
	clf = &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-instance",
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
					Secret: &loggingv1.OutputSecretSpec{
						Name: "app-logs-secret",
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
					Secret: &loggingv1.OutputSecretSpec{
						Name: "cluster-logs-secret",
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

	// Setup the fake k8s client
	resources := Options{
		ClusterLogForwarder: clf,
	}
	clfSpec, err := buildClusterLogForwarderSpec(resources)
	require.NoError(t, err)
	require.NotNil(t, clfSpec.Outputs[0].Secret)
	require.NotNil(t, clfSpec.Outputs[1].Secret)
	require.Equal(t, "app-logs-secret", clfSpec.Outputs[0].Secret.Name)
	require.Equal(t, "cluster-logs-secret", clfSpec.Outputs[1].Secret.Name)
}
