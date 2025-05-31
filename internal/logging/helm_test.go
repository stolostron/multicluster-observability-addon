package logging

import (
	"context"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/handlers"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	_ = loggingv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = addonapiv1alpha1.AddToScheme(scheme.Scheme)
)

func fakeGetValues(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
		keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addoncfg.AddonDeploymentConfigResource)
		if err := k8s.Get(context.TODO(), keys[0], aodc, &client.GetOptions{}); err != nil {
			return nil, err
		}
		addonOpts, err := addon.BuildOptions(aodc)
		if err != nil {
			return nil, err
		}

		// Check if this is a hub cluster by looking for the local-cluster label
		isHub := false
		if cluster != nil {
			if val, ok := cluster.Labels["local-cluster"]; ok {
				isHub = val == "true"
			}
		}

		opts, err := handlers.BuildOptions(context.TODO(), k8s, mcAddon, addonOpts.Platform.Logs, addonOpts.UserWorkloads.Logs, isHub)
		if err != nil {
			return nil, err
		}

		logging, err := manifests.BuildValues(opts)
		if err != nil {
			return nil, err
		}

		return addonfactory.JsonStructToValues(logging)
	}
}

func newLoggingAgentAddon(initObjects []client.Object, addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig) agent.AgentAddon {
	initObjects = append(initObjects, addOnDeploymentConfig)
	// Setup the fake k8s client with all objects
	fakeKubeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(initObjects...).
		Build()

	// Setup the fake addon client
	fakeAddonClient := fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.LoggingChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}
	return loggingAgentAddon
}

func newMCAOUnmanagedScenario() *addonapiv1alpha1.ManagedClusterAddOn {
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      "multicluster-observability-addon",
			},
			DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
				ConfigReferent: addonapiv1alpha1.ConfigReferent{
					Namespace: "open-cluster-management-observability",
					Name:      "multicluster-observability-addon",
				},
				SpecHash: "fake-spec-hash",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "observability.openshift.io",
				Resource: "clusterlogforwarders",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      "mcoa-instance",
			},
		},
	}

	return managedClusterAddOn
}

func newAODCUnmanagedScenario() *addonapiv1alpha1.AddOnDeploymentConfig {
	addOnDeploymentConfig := &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "openshiftLoggingChannel",
					Value: "latest-version",
				},
				{
					Name:  "platformLogsCollection",
					Value: "clusterlogforwarders.v1.observability.openshift.io",
				},
			},
		},
	}
	return addOnDeploymentConfig
}

func newCLFUnmanagedScenario() *loggingv1.ClusterLogForwarder {
	return &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-instance",
			Namespace: "open-cluster-management-observability",
		},
		Spec: loggingv1.ClusterLogForwarderSpec{
			Inputs: []loggingv1.InputSpec{
				{
					Name:           "infra-logs",
					Infrastructure: &loggingv1.Infrastructure{},
				},
			},
			Outputs: []loggingv1.OutputSpec{
				{
					Name: "cloudwatch-output",
					Type: loggingv1.OutputTypeCloudwatch,
					Cloudwatch: &loggingv1.Cloudwatch{
						Authentication: &loggingv1.CloudwatchAuthentication{
							Type: loggingv1.CloudwatchAuthTypeAccessKey,
							AWSAccessKey: &loggingv1.CloudwatchAWSAccessKey{
								KeyId: loggingv1.SecretReference{
									SecretName: "static-authentication",
									Key:        "key",
								},
								KeySecret: loggingv1.SecretReference{
									SecretName: "static-authentication",
									Key:        "pass",
								},
							},
						},
					},
				},
			},
			Pipelines: []loggingv1.PipelineSpec{
				{
					Name:       "infra-to-cloudwatch",
					InputRefs:  []string{"infra-logs"},
					OutputRefs: []string{"cloudwatch-output"},
				},
			},
		},
	}
}

// Test_Logging_Unmanaged_CLF mainly tests the creation of the ClusterLogForwarder
// and the static authentication secrets.
func Test_Logging_Unmanaged_CLF(t *testing.T) {
	// Setup a managed cluster
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	// Register the addon for the managed cluster
	managedClusterAddOn := newMCAOUnmanagedScenario()
	addOnDeploymentConfig := newAODCUnmanagedScenario()

	// Setup configuration resources: ClusterLogForwarder, AddOnDeploymentConfig, Secrets, ConfigMaps
	clf := &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-instance",
			Namespace: "open-cluster-management-observability",
			Annotations: map[string]string{
				"observability.openshift.io/tech-preview-otlp-output": "true",
			},
		},
		Spec: loggingv1.ClusterLogForwarderSpec{
			Inputs: []loggingv1.InputSpec{
				{
					Name: "app-logs",
					Application: &loggingv1.Application{
						Includes: []loggingv1.NamespaceContainerSpec{
							{
								Namespace: "ns-1",
							},
							{
								Namespace: "ns-2",
							},
						},
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
					Loki: &loggingv1.Loki{
						Authentication: &loggingv1.HTTPAuthentication{
							Token: &loggingv1.BearerToken{
								From: loggingv1.BearerTokenFromSecret,
								Secret: &loggingv1.BearerTokenSecretKey{
									Name: "static-authentication",
									Key:  "pass",
								},
							},
						},
						LabelKeys: []string{"key-1", "key-2"},
						TenantKey: "tenant-x",
					},
					// Simply here to test the ConfigMap reference
					TLS: &loggingv1.OutputTLSSpec{
						TLSSpec: loggingv1.TLSSpec{
							CA: &loggingv1.ValueReference{
								ConfigMapName: "foo",
							},
						},
					},
				},
				{
					Name: "cluster-logs",
					Type: loggingv1.OutputTypeCloudwatch,
					Cloudwatch: &loggingv1.Cloudwatch{
						Authentication: &loggingv1.CloudwatchAuthentication{
							Type: loggingv1.CloudwatchAuthTypeAccessKey,
							AWSAccessKey: &loggingv1.CloudwatchAWSAccessKey{
								KeyId: loggingv1.SecretReference{
									SecretName: "static-authentication",
									Key:        "key",
								},
								KeySecret: loggingv1.SecretReference{
									SecretName: "static-authentication",
									Key:        "pass",
								},
							},
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
					InputRefs:  []string{"infra-logs", string(loggingv1.InputTypeAudit)},
					OutputRefs: []string{"cluster-logs"},
				},
			},
		},
	}

	staticCred := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "static-authentication",
			Namespace: "open-cluster-management-observability",
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}

	caConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "open-cluster-management-observability",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}

	// Create the LoggingAgentAddon
	loggingAgentAddon := newLoggingAgentAddon([]client.Object{managedClusterAddOn, clf, staticCred, caConfigMap}, addOnDeploymentConfig)

	// Render manifests and return them as k8s runtime objects
	objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Equal(t, 11, len(objects))

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *operatorsv1alpha1.Subscription:
			require.Equal(t, "latest-version", obj.Spec.Channel)
		case *loggingv1.ClusterLogForwarder:
			require.Equal(t, "true", obj.GetAnnotations()["observability.openshift.io/tech-preview-otlp-output"])
			require.NotNil(t, obj.Spec.Outputs[0].Loki.Authentication.Token.Secret)
			require.NotNil(t, obj.Spec.Outputs[1].Cloudwatch.Authentication.AWSAccessKey)
			require.Equal(t, "static-authentication", obj.Spec.Outputs[0].Loki.Authentication.Token.Secret.Name)
			require.Equal(t, "static-authentication", obj.Spec.Outputs[1].Cloudwatch.Authentication.AWSAccessKey.KeySecret.SecretName)
			// Check name and namespace to make sure that if we change the helm
			// manifests that we don't break the addon probes
			require.Equal(t, addoncfg.SpokeCLFName, obj.Name)
			require.Equal(t, addoncfg.SpokeCLFNamespace, obj.Namespace)
		case *corev1.Secret:
			if obj.Name == "static-authentication" {
				require.Equal(t, staticCred.Data, obj.Data)
			}
		case *corev1.ConfigMap:
			if obj.Name == "foo" {
				require.Equal(t, caConfigMap.Data, obj.Data)
			}
		}
	}
}

// Test_Logging_Unmanaged tests the scenarios fo the Unamanged scenario
func Test_Logging_Unmanaged(t *testing.T) {
	testCases := []struct {
		name                 string
		isHubCluster         bool
		existingSubscription bool
		nbExptedObjects      int
	}{
		{
			name:            "Spoke",
			nbExptedObjects: 9,
		},
		{
			name:                 "Hub without existing subscription",
			isHubCluster:         true,
			existingSubscription: false,
			nbExptedObjects:      9,
		},
		{
			name:                 "Hub with existing subscription",
			isHubCluster:         true,
			existingSubscription: true,
			nbExptedObjects:      6,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup a managed cluster with local-cluster label to simulate a hub
			managedCluster := addontesting.NewManagedCluster("cluster-1")
			if tc.isHubCluster {
				// This identifies it as a hub cluster
				managedCluster.Labels = map[string]string{
					"local-cluster": "true",
				}
			}

			// Register the addon for the managed cluster
			managedClusterAddOn := newMCAOUnmanagedScenario()
			addOnDeploymentConfig := newAODCUnmanagedScenario()

			// Setup configuration resources
			clf := newCLFUnmanagedScenario()

			staticCred := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "static-authentication",
					Namespace: "open-cluster-management-observability",
				},
				Data: map[string][]byte{
					"key":  []byte("data"),
					"pass": []byte("data"),
				},
			}

			// Setup test objects
			initObjects := []client.Object{clf, staticCred}

			// Conditionally add existing subscription based on test case
			if tc.existingSubscription {
				existingSubscription := &operatorsv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-logging",
						Namespace: "openshift-logging",
					},
					Spec: &operatorsv1alpha1.SubscriptionSpec{
						Channel:                "latest-version",
						InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
						CatalogSource:          "redhat-operators",
						CatalogSourceNamespace: "openshift-marketplace",
					},
				}
				initObjects = append(initObjects, existingSubscription)
			}

			logginAgentAddon := newLoggingAgentAddon(initObjects, addOnDeploymentConfig)

			// Render manifests and return them as k8s runtime objects
			objects, err := logginAgentAddon.Manifests(managedCluster, managedClusterAddOn)
			require.NoError(t, err)
			require.Equal(t, tc.nbExptedObjects, len(objects))
		})
	}
}
