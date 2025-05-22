package logging

import (
	"context"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
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
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
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
	_ = loggingv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = addonapiv1alpha1.AddToScheme(scheme.Scheme)
	_ = certmanagerv1.AddToScheme(scheme.Scheme)
	_ = lokiv1.AddToScheme(scheme.Scheme)
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
			if val, ok := cluster.Labels["local-cluster"]; ok {
				isHub = val == "true"
			}
		}

		opts, err := handlers.BuildOptions(context.TODO(), k8s, mcAddon, addonOpts.Platform.Logs, addonOpts.UserWorkloads.Logs, isHub, addonOpts.HubHostname)
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
	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.LoggingChartDir).
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

func newCMAODefaultSenario() *addonapiv1alpha1.ClusterManagementAddOn {
	return &addonapiv1alpha1.ClusterManagementAddOn{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterManagementAddOn",
			APIVersion: addonapiv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: addon.Name,
			UID:  "test-uid-12345",
		},
	}
}

func newMCAODefaultScenario() *addonapiv1alpha1.ManagedClusterAddOn {
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
				Namespace: "local-cluster",
				Name:      "default-stack-instance-bar",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "loki.grafana.com",
				Resource: "lokistacks",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "local-cluster",
				Name:      "default-stack-instance-bar",
			},
		},
	}
	return managedClusterAddOn
}

func newAODCDefaultScenario() *addonapiv1alpha1.AddOnDeploymentConfig {
	return &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "openshiftLoggingChannel",
					Value: "stable-latest",
				},
				{
					Name:  "hubHostname",
					Value: "myhub.foo.com",
				},
				{
					Name:  "platformLogsDefault",
					Value: "true",
				},
			},
		},
	}
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

func newCLFDefaultScenario() *loggingv1.ClusterLogForwarder {
	return &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-stack-instance-bar",
			Namespace: "local-cluster",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonapiv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					Name:       addon.Name,
					UID:        "test-uid-12345",
					Controller: ptr.To(true),
				},
			},
		},
		Spec: loggingv1.ClusterLogForwarderSpec{
			ServiceAccount: loggingv1.ServiceAccount{
				Name: "mcoa-logging-managed-collector",
			},
			ManagementState: loggingv1.ManagementStateUnmanaged,
			Filters: []loggingv1.FilterSpec{
				{
					Name: "filter-1",
					Type: loggingv1.FilterTypeDrop,
					KubeAPIAudit: &loggingv1.KubeAPIAudit{
						Rules: []auditv1.PolicyRule{
							{
								Level: auditv1.LevelMetadata,
							},
						},
					},
				},
			},
			Outputs: []loggingv1.OutputSpec{
				{
					Name: "hub-lokistack",
					Type: loggingv1.OutputTypeOTLP,
					OTLP: &loggingv1.OTLP{
						URL: "https://not-the-final-url",
					},
					TLS: &loggingv1.OutputTLSSpec{
						InsecureSkipVerify: true,
						TLSSpec: loggingv1.TLSSpec{
							CA: &loggingv1.ValueReference{
								Key:        "ca.crt",
								SecretName: "mcoa-logging-managed-collection-tls",
							},
							Certificate: &loggingv1.ValueReference{
								Key:        "tls.crt",
								SecretName: "mcoa-logging-managed-collection-tls",
							},
							Key: &loggingv1.SecretReference{
								Key:        "tls.key",
								SecretName: "mcoa-logging-managed-collection-tls",
							},
						},
					},
				},
			},
			Pipelines: []loggingv1.PipelineSpec{
				{
					Name:       "not-the-correct-name",
					InputRefs:  []string{"infrastructure"},
					OutputRefs: []string{"hub-lokistack"},
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
			ServiceAccount: loggingv1.ServiceAccount{
				Name: "mcoa-sa",
			},
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
		case *corev1.ServiceAccount:
			require.Equal(t, "mcoa-sa", obj.Name)
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
			require.Equal(t, addon.UnmanagedCLFName, obj.Name)
			require.Equal(t, manifests.LoggingNamespace, obj.Namespace)
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

func Test_Logging_Managed_Collection_Spoke(t *testing.T) {
	// Setup a managed cluster
	managedCluster := addontesting.NewManagedCluster("cluster-1")

	// Register the addon for the managed cluster
	managedClusterAddOn := newMCAODefaultScenario()
	addOnDeploymentConfig := newAODCDefaultScenario()
	// Needed to validate the controller reference
	cmao := newCMAODefaultSenario()

	// Emulate the secret that would've been created by the cert-manager
	mtls := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-logging-managed-collection-tls",
			Namespace: "cluster-1",
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("data"),
			"tls.crt": []byte("data"),
			"tls.key": []byte("data"),
		},
	}

	existingCLF := newCLFDefaultScenario()

	expectedCLF := &loggingv1.ClusterLogForwarder{
		TypeMeta: metav1.TypeMeta{
			APIVersion: loggingv1.GroupVersion.String(),
			Kind:       "ClusterLogForwarder",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifests.DefaultCollectionCLFName,
			Namespace: "openshift-logging",
			Annotations: map[string]string{
				"observability.openshift.io/tech-preview-otlp-output": "enabled",
			},
			Labels: map[string]string{
				"release": "multicluster-observability-addon",
				"chart":   "collection-1.0.0",
				"app":     "collection",
			},
		},
		Spec: loggingv1.ClusterLogForwarderSpec{
			ManagementState: loggingv1.ManagementStateManaged,
			ServiceAccount: loggingv1.ServiceAccount{
				Name: "mcoa-logging-managed-collector",
			},
			Filters: []loggingv1.FilterSpec{
				{
					Name: "filter-1",
					Type: loggingv1.FilterTypeDrop,
					KubeAPIAudit: &loggingv1.KubeAPIAudit{
						Rules: []auditv1.PolicyRule{
							{
								Level: auditv1.LevelMetadata,
							},
						},
					},
				},
			},
			Outputs: []loggingv1.OutputSpec{
				{
					Name: "hub-lokistack",
					Type: loggingv1.OutputTypeOTLP,
					OTLP: &loggingv1.OTLP{
						URL: "https://mcoa-logging-managed-storage-openshift-logging.apps.myhub.foo.com/api/logs/v1/cluster-1/otlp/v1/logs",
					},
					TLS: &loggingv1.OutputTLSSpec{
						InsecureSkipVerify: true,
						TLSSpec: loggingv1.TLSSpec{
							CA: &loggingv1.ValueReference{
								Key:        "ca.crt",
								SecretName: "mcoa-logging-managed-collection-tls",
							},
							Certificate: &loggingv1.ValueReference{
								Key:        "tls.crt",
								SecretName: "mcoa-logging-managed-collection-tls",
							},
							Key: &loggingv1.SecretReference{
								Key:        "tls.key",
								SecretName: "mcoa-logging-managed-collection-tls",
							},
						},
					},
				},
			},
			Pipelines: []loggingv1.PipelineSpec{
				{
					Name:       "infra-hub-lokistack",
					InputRefs:  []string{"infrastructure"},
					OutputRefs: []string{"hub-lokistack"},
				},
			},
		},
	}

	loggingAgentAddon := newLoggingAgentAddon([]client.Object{cmao, managedClusterAddOn, existingCLF, mtls}, addOnDeploymentConfig)

	// Render manifests and return them as k8s runtime objects
	objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Equal(t, 9, len(objects))

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *corev1.ServiceAccount:
			require.Equal(t, "mcoa-logging-managed-collector", obj.Name)
		case *operatorsv1alpha1.Subscription:
			require.Equal(t, "stable-latest", obj.Spec.Channel)
		case *loggingv1.ClusterLogForwarder:
			require.Equal(t, expectedCLF, obj)
			// Check name and namespace to make sure that if we change the helm
			// manifests that we don't break the addon probes
			require.Equal(t, manifests.DefaultCollectionCLFName, obj.Name)
			require.Equal(t, manifests.LoggingNamespace, obj.Namespace)
		}
	}
}

func Test_Logging_Managed_Storage(t *testing.T) {
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
			nbExptedObjects:      12,
		},
		{
			name:                 "Hub with existing subscription",
			isHubCluster:         true,
			existingSubscription: true,
			nbExptedObjects:      9,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup a managed cluster
			managedCluster := addontesting.NewManagedCluster("local-cluster")
			if tc.isHubCluster {
				managedCluster.Labels = map[string]string{
					"local-cluster": "true",
				}
			}

			// Register the addon for the managed cluster
			managedClusterAddOn := newMCAODefaultScenario()
			addOnDeploymentConfig := newAODCDefaultScenario()
			// Needed to validate the controller reference
			cmao := newCMAODefaultSenario()

			// Emulate the secret that would've been created by the cert-manager
			mtlsCollection := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mcoa-logging-managed-collection-tls",
					Namespace: "local-cluster",
				},
				Data: map[string][]byte{
					"ca.crt":  []byte("data"),
					"tls.crt": []byte("data"),
					"tls.key": []byte("data"),
				},
			}
			existingCLF := newCLFDefaultScenario()
			existingLS := &lokiv1.LokiStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-stack-instance-bar",
					Namespace: "local-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: addonapiv1alpha1.GroupVersion.String(),
							Kind:       "ClusterManagementAddOn",
							Name:       addon.Name,
							UID:        "test-uid-12345",
							Controller: ptr.To(true),
						},
					},
				},
				Spec: lokiv1.LokiStackSpec{
					ManagementState:  lokiv1.ManagementStateUnmanaged,
					Size:             lokiv1.SizeOneXMedium,
					StorageClassName: "foo-blob",
					Storage: lokiv1.ObjectStorageSpec{
						Secret: lokiv1.ObjectStorageSecretSpec{
							Type: "azure",
							Name: "mcoa-bar-azure-secret",
						},
						Schemas: []lokiv1.ObjectStorageSchema{
							{
								Version:       lokiv1.ObjectStorageSchemaV13,
								EffectiveDate: "2025-01-01",
							},
						},
					},
					Tenants: &lokiv1.TenantsSpec{
						Mode: lokiv1.Static,
						Authentication: []lokiv1.AuthenticationSpec{
							{
								TenantName: "tenant-1",
								TenantID:   "tenant-1",
								MTLS: &lokiv1.MTLSSpec{
									CA: &lokiv1.CASpec{
										CAKey: "ca.crt",
										CA:    "mcoa-logging-managed-storage-tls",
									},
								},
							},
							{
								TenantName: "tenant-2",
								TenantID:   "tenant-2",
								MTLS: &lokiv1.MTLSSpec{
									CA: &lokiv1.CASpec{
										CAKey: "ca.crt",
										CA:    "mcoa-logging-managed-storage-tls",
									},
								},
							},
						},
						Authorization: &lokiv1.AuthorizationSpec{
							Roles: []lokiv1.RoleSpec{
								{
									Name:        "tenant-1-logs",
									Resources:   []string{"logs"},
									Permissions: []lokiv1.PermissionType{"read", "write"},
									Tenants:     []string{"tenant-1"},
								},
								{
									Name:        "tenant-2-logs",
									Resources:   []string{"logs"},
									Permissions: []lokiv1.PermissionType{"read", "write"},
									Tenants:     []string{"tenant-2"},
								},
								{
									Name:        "cluster-reader",
									Resources:   []string{"logs"},
									Permissions: []lokiv1.PermissionType{"read"},
									Tenants:     []string{"tenant-1", "tenant-2"},
								},
							},
							RoleBindings: []lokiv1.RoleBindingsSpec{
								{
									Name:  "tenant-1-logs",
									Roles: []string{"tenant-1-logs"},
									Subjects: []lokiv1.Subject{
										{
											Kind: "group",
											Name: "tenant-1",
										},
									},
								},
								{
									Name:  "tenant-2-logs",
									Roles: []string{"tenant-2-logs"},
									Subjects: []lokiv1.Subject{
										{
											Kind: "group",
											Name: "tenant-2",
										},
									},
								},
								{
									Name:  "cluster-reader",
									Roles: []string{"cluster-reader"},
									Subjects: []lokiv1.Subject{
										{
											Kind: "group",
											Name: "mcoa-logs-admin",
										},
									},
								},
							},
						},
					},
					Limits: &lokiv1.LimitsSpec{
						Global: &lokiv1.LimitsTemplateSpec{
							IngestionLimits: &lokiv1.IngestionLimitSpec{
								IngestionRate: 200,
							},
						},
					},
				},
			}

			// Tenants
			tenant1 := addontesting.NewAddon("multicluster-observability-addon", "tenant-1")
			tenant2 := addontesting.NewAddon("multicluster-observability-addon", "tenant-2")

			// Emulate the secret that would've been created by the cert-manager
			mtls := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mcoa-logging-managed-storage-tls",
					Namespace: "local-cluster",
				},
				Data: map[string][]byte{
					"ca.crt":  []byte("data"),
					"tls.crt": []byte("data"),
					"tls.key": []byte("data"),
				},
			}

			objstorage := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mcoa-bar-azure-secret",
					Namespace: "local-cluster",
				},
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			}

			expectedLS := &lokiv1.LokiStack{
				TypeMeta: metav1.TypeMeta{
					APIVersion: lokiv1.GroupVersion.String(),
					Kind:       "LokiStack",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      manifests.DefaultStorageLSName,
					Namespace: "openshift-logging",
					Labels: map[string]string{
						"release": "multicluster-observability-addon",
						"chart":   "storage-1.0.0",
						"app":     "storage",
					},
				},
				Spec: lokiv1.LokiStackSpec{
					ManagementState:  lokiv1.ManagementStateManaged,
					Size:             lokiv1.SizeOneXMedium,
					StorageClassName: "foo-blob",
					Storage: lokiv1.ObjectStorageSpec{
						Secret: lokiv1.ObjectStorageSecretSpec{
							Type: "azure",
							Name: "mcoa-bar-azure-secret",
						},
						Schemas: []lokiv1.ObjectStorageSchema{
							{
								Version:       lokiv1.ObjectStorageSchemaV13,
								EffectiveDate: "2025-01-01",
							},
						},
					},
					Tenants: &lokiv1.TenantsSpec{
						Mode: lokiv1.Static,
						Authentication: []lokiv1.AuthenticationSpec{
							{
								TenantName: "tenant-1",
								TenantID:   "tenant-1",
								MTLS: &lokiv1.MTLSSpec{
									CA: &lokiv1.CASpec{
										CAKey: "ca.crt",
										CA:    "mcoa-logging-managed-storage-tls",
									},
								},
							},
							{
								TenantName: "tenant-2",
								TenantID:   "tenant-2",
								MTLS: &lokiv1.MTLSSpec{
									CA: &lokiv1.CASpec{
										CAKey: "ca.crt",
										CA:    "mcoa-logging-managed-storage-tls",
									},
								},
							},
						},
						Authorization: &lokiv1.AuthorizationSpec{
							Roles: []lokiv1.RoleSpec{
								{
									Name:        "tenant-1-logs",
									Resources:   []string{"logs"},
									Permissions: []lokiv1.PermissionType{"read", "write"},
									Tenants:     []string{"tenant-1"},
								},
								{
									Name:        "tenant-2-logs",
									Resources:   []string{"logs"},
									Permissions: []lokiv1.PermissionType{"read", "write"},
									Tenants:     []string{"tenant-2"},
								},
								{
									Name:        "cluster-reader",
									Resources:   []string{"logs"},
									Permissions: []lokiv1.PermissionType{"read"},
									Tenants:     []string{"tenant-1", "tenant-2"},
								},
							},
							RoleBindings: []lokiv1.RoleBindingsSpec{
								{
									Name:  "tenant-1-logs",
									Roles: []string{"tenant-1-logs"},
									Subjects: []lokiv1.Subject{
										{
											Kind: "group",
											Name: "tenant-1",
										},
									},
								},
								{
									Name:  "tenant-2-logs",
									Roles: []string{"tenant-2-logs"},
									Subjects: []lokiv1.Subject{
										{
											Kind: "group",
											Name: "tenant-2",
										},
									},
								},
								{
									Name:  "cluster-reader",
									Roles: []string{"cluster-reader"},
									Subjects: []lokiv1.Subject{
										{
											Kind: "group",
											Name: "mcoa-logs-admin",
										},
									},
								},
							},
						},
					},
					Limits: &lokiv1.LimitsSpec{
						Global: &lokiv1.LimitsTemplateSpec{
							IngestionLimits: &lokiv1.IngestionLimitSpec{
								IngestionRate: 200,
							},
						},
					},
				},
			}

			initObjects := []client.Object{cmao, managedClusterAddOn, existingCLF, mtlsCollection, existingLS, mtls, objstorage, tenant1, tenant2}

			// Conditionally add existing subscription based on test case
			if tc.existingSubscription {
				existingSubscription := &operatorsv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-logging",
						Namespace: "openshift-logging",
					},
					Spec: &operatorsv1alpha1.SubscriptionSpec{
						Channel:                "stable-latest",
						InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
						CatalogSource:          "redhat-operators",
						CatalogSourceNamespace: "openshift-marketplace",
					},
				}
				initObjects = append(initObjects, existingSubscription)
			}

			// Setup the fake k8s client
			loggingAgentAddon := newLoggingAgentAddon(initObjects, addOnDeploymentConfig)

			// Render manifests and return them as k8s runtime objects
			objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
			require.NoError(t, err)
			require.Equal(t, tc.nbExptedObjects, len(objects))

			for _, obj := range objects {
				switch obj := obj.(type) {
				case *lokiv1.LokiStack:
					require.Equal(t, expectedLS, obj)
					// Check name and namespace to make sure that if we change the helm
					// manifests that we don't break the addon probes
					require.Equal(t, manifests.DefaultStorageLSName, obj.Name)
					require.Equal(t, manifests.LoggingNamespace, obj.Namespace)
				}
			}
		})
	}
}
