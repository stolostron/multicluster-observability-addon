package addon

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

func TestBuildOptions(t *testing.T) {
	testCases := []struct {
		name           string
		addOnDeploy    *addonapiv1alpha1.AddOnDeploymentConfig
		expectedOpts   Options
		expectedErrMsg string
	}{
		{
			name:         "nil AddOnDeploymentConfig",
			addOnDeploy:  nil,
			expectedOpts: Options{},
		},
		{
			name: "empty CustomizedVariables",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{},
			},
			expectedOpts: Options{},
		},
		{
			name: "invalid name key",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{Name: "foo", Value: ""},
					},
				},
			},
			expectedOpts: Options{},
		},
		{
			name: "valid metrics without scheme for hub",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyUserWorkloadMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyMetricsHubHostname, Value: "metrics.example.com"},
						{Name: KeyMetricsAlertManagerHostname, Value: "alerts.example.com"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					Metrics: MetricsOptions{
						CollectionEnabled: true,
						HubEndpoint: url.URL{
							Scheme: "https",
							Host:   "metrics.example.com",
							Path:   "api/metrics/v1/default/api/v1/receive",
						},
						AlertManagerEndpoint: url.URL{
							Scheme: "https",
							Host:   "alerts.example.com",
							Path:   "",
						},
					},
				},
				UserWorkloads: UserWorkloadOptions{
					Enabled: true,
					Metrics: MetricsOptions{
						CollectionEnabled: true,
					},
				},
			},
		},
		{
			name: "valid metrics",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyUserWorkloadMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyMetricsHubHostname, Value: "https://metrics.example.com"},
						{Name: KeyMetricsAlertManagerHostname, Value: "https://alerts.example.com"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					Metrics: MetricsOptions{
						CollectionEnabled: true,
						HubEndpoint: url.URL{
							Scheme: "https",
							Host:   "metrics.example.com",
							Path:   "api/metrics/v1/default/api/v1/receive",
						},
						AlertManagerEndpoint: url.URL{
							Scheme: "https",
							Host:   "alerts.example.com",
							Path:   "",
						},
					},
				},
				UserWorkloads: UserWorkloadOptions{
					Enabled: true,
					Metrics: MetricsOptions{
						CollectionEnabled: true,
					},
				},
			},
		},
		{
			name: "invalid metrics hub hostname",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{Name: KeyMetricsHubHostname, Value: "://invalid-url"},
					},
				},
			},
			expectedErrMsg: "invalid metrics hub hostname: invalid hostname format ':'",
		},
		{
			name: "valid logs",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{Name: KeyOpenShiftLoggingChannel, Value: "stable-6"},
						{Name: KeyPlatformLogsCollection, Value: string(ClusterLogForwarderV1)},
						{Name: KeyUserWorkloadLogsCollection, Value: string(ClusterLogForwarderV1)},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					Logs: LogsOptions{
						CollectionEnabled:   true,
						SubscriptionChannel: "stable-6",
					},
				},
				UserWorkloads: UserWorkloadOptions{
					Enabled: true,
					Logs: LogsOptions{
						CollectionEnabled:   true,
						SubscriptionChannel: "stable-6",
					},
				},
			},
		},
		{
			name: "valid otel",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{Name: KeyUserWorkloadTracesCollection, Value: string(OpenTelemetryCollectorV1beta1)},
						{Name: KeyUserWorkloadInstrumentation, Value: string(InstrumentationV1alpha1)},
					},
				},
			},
			expectedOpts: Options{
				UserWorkloads: UserWorkloadOptions{
					Enabled: true,
					Traces: TracesOptions{
						CollectionEnabled:      true,
						InstrumentationEnabled: true,
					},
				},
			},
		},
		{
			name: "valid incident detection",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
						{Name: KeyPlatformIncidentDetection, Value: string(UIPluginV1alpha1)},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						IncidentDetection: IncidentDetection{
							Enabled: true,
						},
					},
				},
			},
		},
		{
			name: "valid node selector and tolerations",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					NodePlacement: &addonapiv1alpha1.NodePlacement{
						NodeSelector: map[string]string{"node-role.kubernetes.io/infra": ""},
						Tolerations: []corev1.Toleration{
							{
								Key:      "node-role.kubernetes.io/infra",
								Operator: "Exists",
								Effect:   "NoSchedule",
							},
						},
					},
				},
			},
			expectedOpts: Options{
				NodeSelector: map[string]string{"node-role.kubernetes.io/infra": ""},
				Tolerations: []corev1.Toleration{
					{
						Key:      "node-role.kubernetes.io/infra",
						Operator: "Exists",
						Effect:   "NoSchedule",
					},
				},
			},
		},
		{
			name: "valid resource requirements",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					ResourceRequirements: []addonapiv1alpha1.ContainerResourceRequirements{
						{
							ContainerID: "deployments:klusterlet-addon-search:collector",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("3000Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("10m"), // lack of memory request in annotation leads to default of 128Mi
								},
							},
						},
					},
				},
			},
			expectedOpts: Options{
				ResourceReqs: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("3000Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
		{
			name: "valid http proxy and no proxy",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1alpha1.ProxyConfig{
						HTTPProxy: "http://proxy.example.com:8080",
						NoProxy:   "*.example.com",
					},
				},
			},
			expectedOpts: Options{
				ProxyConfig: ProxyConfig{
					ProxyURL: &url.URL{
						Scheme: "http",
						Host:   "proxy.example.com:8080",
					},
					NoProxy: "*.example.com",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := BuildOptions(tc.addOnDeploy)
			if tc.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Equal(t, err.Error(), tc.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedOpts, opts)
			}
		})
	}
}
