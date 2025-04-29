package logging

import (
	"context"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
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
		keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addon.AddonDeploymentConfigResource)
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
			if val, ok := cluster.Labels["local-cluster"]; ok && val == "true" {
				isHub = true
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

func Test_Logging_Spoke(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedCluster      *clusterv1.ManagedCluster
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
		clf                   *loggingv1.ClusterLogForwarder
		staticCred            *corev1.Secret
		caConfigMap           *corev1.ConfigMap

		// Test clients
		fakeKubeClient  client.Client
		fakeAddonClient *fakeaddon.Clientset
	)

	// Setup a managed cluster
	managedCluster = addontesting.NewManagedCluster("cluster-1")

	// Register the addon for the managed cluster
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")
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

	// Setup configuration resources: ClusterLogForwarder, AddOnDeploymentConfig, Secrets, ConfigMaps
	clf = &loggingv1.ClusterLogForwarder{
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

	staticCred = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "static-authentication",
			Namespace: "open-cluster-management-observability",
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}

	caConfigMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "open-cluster-management-observability",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}

	addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
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

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(addOnDeploymentConfig, clf, staticCred, caConfigMap).
		Build()

	// Setup the fake addon client
	fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.LoggingChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

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
			require.Equal(t, addon.SpokeCLFName, obj.Name)
			require.Equal(t, addon.SpokeCLFNamespace, obj.Namespace)
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

func Test_Logging_HubCluster(t *testing.T) {
	testCases := []struct {
		name                 string
		existingSubscription bool
		nbExptedObjects      int
	}{
		{
			name:                 "Hub with existing subscription",
			existingSubscription: true,
			nbExptedObjects:      6,
		},
		{
			name:                 "Hub without existing subscription",
			existingSubscription: false,
			nbExptedObjects:      9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				// Addon environment and registration
				managedCluster      *clusterv1.ManagedCluster
				managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

				// Addon configuration
				addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
				clf                   *loggingv1.ClusterLogForwarder
				staticCred            *corev1.Secret

				// Test clients
				fakeKubeClient  client.Client
				fakeAddonClient *fakeaddon.Clientset

				// Existing subscription (optional)
				existingSubscription *operatorsv1alpha1.Subscription
			)

			// Setup a managed cluster with local-cluster label to simulate a hub
			managedCluster = addontesting.NewManagedCluster("cluster-1")
			managedCluster.Labels = map[string]string{
				"local-cluster": "true", // This identifies it as a hub cluster
			}

			// Register the addon for the managed cluster
			managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")
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

			// Setup configuration resources
			clf = &loggingv1.ClusterLogForwarder{
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

			staticCred = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "static-authentication",
					Namespace: "open-cluster-management-observability",
				},
				Data: map[string][]byte{
					"key":  []byte("data"),
					"pass": []byte("data"),
				},
			}

			addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicluster-observability-addon",
					Namespace: "open-cluster-management-observability",
				},
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{
							Name:  "openshiftLoggingChannel",
							Value: "stable-5.8",
						},
						{
							Name:  "platformLogsCollection",
							Value: "clusterlogforwarders.v1.observability.openshift.io",
						},
					},
				},
			}

			// Setup test objects
			testObjects := []client.Object{
				addOnDeploymentConfig,
				clf,
				staticCred,
			}

			// Conditionally add existing subscription based on test case
			if tc.existingSubscription {
				existingSubscription = &operatorsv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-logging",
						Namespace: "openshift-logging",
					},
					Spec: &operatorsv1alpha1.SubscriptionSpec{
						Channel:                "stable-5.8",
						InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
						CatalogSource:          "redhat-operators",
						CatalogSourceNamespace: "openshift-marketplace",
					},
				}
				testObjects = append(testObjects, existingSubscription)
			}

			// Setup the fake k8s client with all objects
			fakeKubeClient = fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(testObjects...).
				Build()

			// Setup the fake addon client
			fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
			addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
				addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
				addonfactory.ToAddOnCustomizedVariableValues,
			)

			// Wire everything together to a fake addon instance
			loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.LoggingChartDir).
				WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient)).
				WithAgentRegistrationOption(&agent.RegistrationOption{}).
				WithScheme(scheme.Scheme).
				BuildHelmAgentAddon()
			if err != nil {
				klog.Fatalf("failed to build agent %v", err)
			}

			// Render manifests and return them as k8s runtime objects
			objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
			require.NoError(t, err)
			require.Equal(t, tc.nbExptedObjects, len(objects))
		})
	}
}
