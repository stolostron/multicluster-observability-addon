package metrics_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/handlers"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/manifests"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestHelmBuild_Metrics_All(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	assert.NoError(t, prometheusalpha1.AddToScheme(scheme))
	assert.NoError(t, prometheusv1.AddToScheme(scheme))

	installNamespace := "open-cluster-management-addon-observability"
	clientObjects := []client.Object{
		newPrometheusAgent(addon.PlatformPrometheusAgentName, "open-cluster-management-observability"),
		newSecret("observability-controller-open-cluster-management.io-observability-signer-client-cert", "open-cluster-management-observability"),
		newSecret("observability-managed-cluster-certs", "open-cluster-management-observability"),
	}
	// Setup the fake k8s client
	client := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(clientObjects...).
		Build()

	// Setup the fake addon client
	addonClient := fakeaddon.NewSimpleClientset(newAddonDeploymentConfig())
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	agentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.MetricsChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(client)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	// Setup a managed cluster
	managedCluster := addontesting.NewManagedCluster("cluster-1")

	// Register the addon for the managed cluster
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Spec.InstallNamespace = installNamespace
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
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "monitoring.coreos.com",
				Resource: addon.PrometheusAgentResource,
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      addon.PlatformPrometheusAgentName,
			},
		},
	}

	// Render manifests and return them as k8s runtime objects
	objects, err := agentAddon.Manifests(managedCluster, managedClusterAddOn)
	assert.NoError(t, err)

	for _, obj := range objects {
		accessor, err := meta.Accessor(obj)
		assert.NoError(t, err)

		// if not a global object, check namespace
		if !slices.Contains([]string{"ClusterRole", "ClusterRoleBinding"}, obj.GetObjectKind().GroupVersionKind().Kind) {
			assert.Equal(t, installNamespace, accessor.GetNamespace(), fmt.Sprintf("Object: %s/%s", obj.GetObjectKind().GroupVersionKind(), accessor.GetName()))
		}

		// Write out the object to a file
		data, err := yaml.Marshal(obj)
		assert.NoError(t, err)

		yamlData, err := yaml.JSONToYAML(data)
		assert.NoError(t, err)

		// write data to file in subdirectory
		err = os.MkdirAll("output", 0755)
		assert.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("output/%s-%s.yaml", obj.GetObjectKind().GroupVersionKind().Kind, accessor.GetName()), yamlData, 0644)
		assert.NoError(t, err)
		// fmt.Printf("---- Object Name: %s ----\n", obj.GetObjectKind())
		// fmt.Printf("Object: %v\n", obj)
		// t.Logf("Object: %v", obj)
	}
}

func newPrometheusAgent(name, ns string) *prometheusalpha1.PrometheusAgent {
	intPtr := func(i int32) *int32 {
		return &i
	}
	return &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				Replicas:           intPtr(1),
				LogLevel:           "debug",
				ServiceAccountName: "metrics-collector-agent",
				ArbitraryFSAccessThroughSMs: prometheusv1.ArbitraryFSAccessThroughSMsConfig{
					Deny: true,
				},
				ServiceMonitorSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "acm-platform-metrics-collector",
					},
				},
				ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
				ScrapeConfigSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "acm-platform-metrics-collector",
					},
				},
				ScrapeConfigNamespaceSelector: &metav1.LabelSelector{},
				ScrapeInterval:                "15s",
				ScrapeTimeout:                 "10s",
				ExternalLabels: map[string]string{
					"clusterID": "0e2c6b1b-fdae-4581-8009-119349edac7e",
					"cluster":   "spoke",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				Secrets: []string{
					"observability-controller-open-cluster-management.io-observability-signer-client-cert",
					"observability-managed-cluster-certs",
				},
				ConfigMaps: []string{
					"prom-server-ca",
				},
				RemoteWrite: []prometheusv1.RemoteWriteSpec{
					{
						URL: "https://observatorium-api-open-cluster-management-observability.apps.sno-4xlarge-416-lqsr2.dev07.red-chesterfield.com/api/metrics/v1/default/api/v1/receive",
						TLSConfig: &prometheusv1.TLSConfig{
							CAFile:   "/etc/prometheus/secrets/observability-managed-cluster-certs/ca.crt",
							CertFile: "/etc/prometheus/secrets/observability-controller-open-cluster-management.io-observability-signer-client-cert/tls.crt",
							KeyFile:  "/etc/prometheus/secrets/observability-controller-open-cluster-management.io-observability-signer-client-cert/tls.key",
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "haproxy",
						Image: "haproxy:latest",
						Ports: []corev1.ContainerPort{
							{
								Name:          "healthz",
								ContainerPort: 8081,
							},
							{
								Name:          "metrics",
								ContainerPort: 8082,
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("healthz"),
								},
							},
							InitialDelaySeconds: 2,
							PeriodSeconds:       5,
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("healthz"),
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
						},

						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
						},
						// 				command: ["/bin/sh", "-c"]
						// args:
						// - |
						//   export BEARER_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) && \
						//   haproxy -f /usr/local/etc/haproxy/haproxy.cfg
						Command: []string{"/bin/sh", "-c"},
						Args: []string{
							"export BEARER_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) && haproxy -f /etc/haproxy/haproxy.cfg",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "haproxy-config",
								MountPath: "/etc/haproxy",
							},
							{
								Name:      "prom-server-ca",
								MountPath: "/etc/haproxy/certs",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "haproxy-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "haproxy-config",
								},
							},
						},
					},
					{
						Name: "prom-server-ca",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "prom-server-ca",
								},
							},
						},
					},
				},
			},
		},
	}
}

func newAddonDeploymentConfig() *addonapiv1alpha1.AddOnDeploymentConfig {
	return &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "loggingSubscriptionChannel",
					Value: "stable-5.9",
				},
			},
		},
	}
}

func newSecret(name, ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}
}

func fakeGetValues(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		_ *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		opts, err := handlers.BuildOptions(context.TODO(), k8s, mcAddon, addon.MetricsOptions{CollectionEnabled: true}, addon.MetricsOptions{})
		if err != nil {
			return nil, err
		}

		helmValues, err := manifests.BuildValues(opts)
		if err != nil {
			return nil, err
		}

		return addonfactory.JsonStructToValues(helmValues)
	}
}
