package logging

import (
	"context"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
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
	_ = loggingv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = addonapiv1alpha1.AddToScheme(scheme.Scheme)
	_ = certmanagerv1.AddToScheme(scheme.Scheme)
	_ = lokiv1.AddToScheme(scheme.Scheme)
)

func fakeGetValues(k8s client.Client, isHub bool) addonfactory.GetValuesFunc {
	return func(
		_ *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
		keys := addon.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addon.AddonDeploymentConfigResource)
		if err := k8s.Get(context.TODO(), keys[0], aodc, &client.GetOptions{}); err != nil {
			return nil, err
		}
		addonOpts, err := addon.BuildOptions(aodc)
		if err != nil {
			return nil, err
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

func Test_Logging_Unmanaged_Collection(t *testing.T) {
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
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient, false)).
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
			require.Equal(t, addon.SpokeUnmanagedCLFName, obj.Name)
			require.Equal(t, addon.LoggingNamespace, obj.Namespace)
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

func Test_Logging_Managed_Collection(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedCluster      *clusterv1.ManagedCluster
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig

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
				Namespace: "local-cluster",
				Name:      "default-stack-instance-bar",
			},
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

	existingCLF := &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-stack-instance-bar",
			Namespace: "local-cluster",
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

	expectedCLF := &loggingv1.ClusterLogForwarder{
		TypeMeta: metav1.TypeMeta{
			APIVersion: loggingv1.GroupVersion.String(),
			Kind:       "ClusterLogForwarder",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.SpokeDefaultStackCLFName,
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
						URL: "https://mcoa-managed-instance-openshift-logging.apps.myhub.foo.com/api/logs/v1/cluster-1/otlp/v1/logs",
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

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(addOnDeploymentConfig, existingCLF, mtls).
		Build()

	// Setup the fake addon client
	fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.LoggingChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient, false)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

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
			require.Equal(t, addon.SpokeDefaultStackCLFName, obj.Name)
			require.Equal(t, addon.LoggingNamespace, obj.Namespace)
		}
	}
}

func Test_Logging_Managed_Storage(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedCluster      *clusterv1.ManagedCluster
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig

		// Test clients
		fakeKubeClient  client.Client
		fakeAddonClient *fakeaddon.Clientset
	)

	// Setup a managed cluster
	managedCluster = addontesting.NewManagedCluster("local-cluster")

	// Register the addon for the managed cluster
	managedClusterAddOn = addontesting.NewAddon("test", "local-cluster")
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
				Group:    "loki.grafana.com",
				Resource: "lokistacks",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "local-cluster",
				Name:      "default-stack-instance-bar",
			},
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
					Value: "stable-latest",
				},
				{
					Name:  "platformLogsDefault",
					Value: "true",
				},
			},
		},
	}

	existingLS := &lokiv1.LokiStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-stack-instance-bar",
			Namespace: "local-cluster",
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
			Name:      addon.SpokeDefaultStackLSName,
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
					OPA: &lokiv1.OPASpec{},
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
					OTLP: &lokiv1.OTLPSpec{
						StreamLabels: &lokiv1.OTLPStreamLabelSpec{
							ResourceAttributes: []lokiv1.OTLPAttributeReference{
								{Name: "k8s.namespace.name"},
								{Name: "kubernetes.namespace_name"},
								{Name: "log_source"},
								{Name: "log_type"},
								{Name: "openshift.cluster.uid"},
								{Name: "openshift.log.source"},
								{Name: "openshift.log.type"},
								{Name: "k8s.container.name"},
								{Name: "k8s.cronjob.name"},
								{Name: "k8s.daemonset.name"},
								{Name: "k8s.deployment.name"},
								{Name: "k8s.job.name"},
								{Name: "k8s.node.name"},
								{Name: "k8s.pod.name"},
								{Name: "k8s.statefulset.name"},
								{Name: "kubernetes.container_name"},
								{Name: "kubernetes.host"},
								{Name: "kubernetes.pod_name"},
								{Name: "service.name"},
							},
						},
						StructuredMetadata: &lokiv1.OTLPMetadataSpec{
							ResourceAttributes: []lokiv1.OTLPAttributeReference{
								{Name: "k8s.node.uid"},
								{Name: "k8s.pod.uid"},
								{Name: "k8s.replicaset.name"},
								{Name: "process.command_line"},
								{Name: "process.executable.name"},
								{Name: "process.executable.path"},
								{Name: "process.pid"},
								{Name: `k8s\.pod\.labels\..+`, Regex: true},
								{Name: `openshift\.labels\..+`, Regex: true},
							},
							LogAttributes: []lokiv1.OTLPAttributeReference{
								{Name: "k8s.event.level"},
								{Name: "k8s.event.object_ref.api.group"},
								{Name: "k8s.event.object_ref.api.version"},
								{Name: "k8s.event.object_ref.name"},
								{Name: "k8s.event.object_ref.resource"},
								{Name: "k8s.event.request.uri"},
								{Name: "k8s.event.response.code"},
								{Name: "k8s.event.stage"},
								{Name: "k8s.event.user_agent"},
								{Name: "k8s.user.groups"},
								{Name: "k8s.user.username"},
								{Name: "level"},
								{Name: "log.iostream"},
								{Name: `k8s\.event\.annotations\..+`, Regex: true},
								{Name: `systemd\.t\..+`, Regex: true},
								{Name: `systemd\.u\..+`, Regex: true},
							},
						},
					},
				},
			},
		},
	}

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(addOnDeploymentConfig, existingLS, mtls, objstorage, tenant1, tenant2).
		Build()

	// Setup the fake addon client
	fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.LoggingChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient, true)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	// Render manifests and return them as k8s runtime objects
	objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Equal(t, 6, len(objects))

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *corev1.ServiceAccount:
			require.Equal(t, "mcoa-logging-managed-collector", obj.Name)
		case *operatorsv1alpha1.Subscription:
			require.Equal(t, "stable-latest", obj.Spec.Channel)
		case *lokiv1.LokiStack:
			require.Equal(t, expectedLS, obj)
			// Check name and namespace to make sure that if we change the helm
			// manifests that we don't break the addon probes
			require.Equal(t, addon.SpokeDefaultStackCLFName, obj.Name)
			require.Equal(t, addon.LoggingNamespace, obj.Namespace)
		}
	}
}
