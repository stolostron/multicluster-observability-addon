package addon

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

func generateTestCertificate(t *testing.T, commonName string) []byte {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}

func TestBuildOptions(t *testing.T) {
	validCertPEM1 := generateTestCertificate(t, "Proxy Root CA 1")
	validCertPEM2 := generateTestCertificate(t, "Proxy Root CA 2")
	validMultiCertPEM := append(validCertPEM1, validCertPEM2...)
	testCases := []struct {
		name           string
		addOnDeploy    *addonapiv1beta1.AddOnDeploymentConfig
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
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
			},
		},
		{
			name: "invalid name key",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: "foo", Value: ""},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
			},
		},
		{
			name: "valid metrics without scheme for hub",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyUserWorkloadMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyMetricsHubHostname, Value: "metrics.example.com"},
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
					},
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
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
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyUserWorkloadMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
						{Name: KeyMetricsHubHostname, Value: "https://metrics.example.com"},
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
					},
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
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
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyMetricsHubHostname, Value: "://invalid-url"},
					},
				},
			},
			expectedErrMsg: "invalid metrics hub hostname: invalid hostname format ':'",
		},
		{
			name: "valid logs",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
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
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
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
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyUserWorkloadTracesCollection, Value: string(OpenTelemetryCollectorV1beta1)},
						{Name: KeyUserWorkloadInstrumentation, Value: string(InstrumentationV1alpha1)},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
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
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
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
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
			},
		},
		{
			name: "right-sizing enabled explicitly",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "enabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "enabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
			},
		},
		{
			name: "right-sizing disabled explicitly",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					},
				},
			},
			// Platform.Enabled is true because RS keys are present (even "disabled").
			// This ensures the rendering pipeline runs so the addon framework can prune
			// stale ManifestWork content when both RS features are disabled.
			expectedOpts: Options{
				Platform: PlatformOptions{Enabled: true},
			},
		},
		{
			name: "valid node selector and tolerations",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					NodePlacement: &addonapiv1beta1.NodePlacement{
						NodeSelector: map[string]string{"node-role.kubernetes.io/infra": ""},
						Tolerations: []corev1.Toleration{
							{
								Key:      "node-role.kubernetes.io/infra",
								Operator: "Exists",
								Effect:   "NoSchedule",
							},
						},
					},
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform:     PlatformOptions{Enabled: true},
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
			name: "valid instance size",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyMetricsStorageInstanceSize, Value: "medium"},
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					},
				},
			},
			expectedOpts: Options{
				InstanceSize: "medium",
				Platform:     PlatformOptions{Enabled: true},
			},
		},
		{
			name: "valid resource requirements",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ResourceRequirements: []addonapiv1beta1.ContainerResourceRequirements{
						{
							ContainerID: "deployments:platform-metrics:collector",
							Resources: corev1.ResourceRequirements{
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
				},
			},
			expectedOpts: Options{
				ResourceReqs: []addonapiv1beta1.ContainerResourceRequirements{
					{
						ContainerID: "deployments:platform-metrics:collector",
						Resources: corev1.ResourceRequirements{
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
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
			},
		},
		{
			name: "valid http proxy and no proxy",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPProxy: "http://proxy.example.com:8080",
						NoProxy:   "*.example.com",
					},
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{Enabled: true},
				ProxyConfig: ProxyConfig{
					HTTPProxy: &url.URL{
						Scheme: "http",
						Host:   "proxy.example.com:8080",
					},
					NoProxy: "*.example.com",
				},
			},
		},
		{
			name: "valid https proxy only",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "https://secure-proxy.example.com:8443",
						NoProxy:    "*.example.com",
					},
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{Enabled: true},
				ProxyConfig: ProxyConfig{
					HTTPSProxy: &url.URL{
						Scheme: "https",
						Host:   "secure-proxy.example.com:8443",
					},
					NoProxy: "*.example.com",
				},
			},
		},
		{
			name: "valid http and https proxy with single cert ca bundle",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPProxy:  "http://proxy.example.com:8080",
						HTTPSProxy: "https://secure-proxy.example.com:8443",
						CABundle:   validCertPEM1,
						NoProxy:    "*.example.com",
					},
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{Enabled: true},
				ProxyConfig: ProxyConfig{
					HTTPProxy: &url.URL{
						Scheme: "http",
						Host:   "proxy.example.com:8080",
					},
					HTTPSProxy: &url.URL{
						Scheme: "https",
						Host:   "secure-proxy.example.com:8443",
					},
					CABundle: validCertPEM1,
					NoProxy:  "*.example.com",
				},
			},
		},
		{
			name: "valid multi cert ca bundle",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "https://secure-proxy.example.com:8443",
						CABundle:   validMultiCertPEM,
					},
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{Enabled: true},
				ProxyConfig: ProxyConfig{
					HTTPSProxy: &url.URL{
						Scheme: "https",
						Host:   "secure-proxy.example.com:8443",
					},
					CABundle: validMultiCertPEM,
				},
			},
		},
		{
			name: "valid node exporter ports",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyNodeExporterHostPort, Value: "19100"},
						{Name: KeyNodeExporterInternalPort, Value: "19101"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					Metrics: MetricsOptions{
						NodeExporter: NodeExporterOptions{
							HostPort:     19100,
							InternalPort: 19101,
						},
					},
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
			},
		},
		{
			name: "enabled alerts",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsAlerts, Value: "enabled"},
						{Name: KeyUserWorkloadMetricsAlerts, Value: "enabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					Metrics: MetricsOptions{
						AlertsEnabled: true,
					},
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							NamespaceEnabled:      true,
							VirtualizationEnabled: true,
						},
					},
				},
				UserWorkloads: UserWorkloadOptions{
					Metrics: MetricsOptions{
						AlertsEnabled: true,
					},
				},
			},
		},
		{
			name: "invalid node exporter host port - format",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyNodeExporterHostPort, Value: "abc"},
					},
				},
			},
			expectedErrMsg: "invalid port format for nodeExporterHostPort",
		},
		{
			name: "invalid node exporter host port - out of bounds high",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyNodeExporterHostPort, Value: "65536"},
					},
				},
			},
			expectedErrMsg: "invalid port: 65536 for nodeExporterHostPort must be between 1 and 65535",
		},
		{
			name: "invalid node exporter port - out of bounds low",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyNodeExporterInternalPort, Value: "0"},
					},
				},
			},
			expectedErrMsg: "invalid port: 0 for nodeExporterInternalPort must be between 1 and 65535",
		},
		{
			name: "invalid https proxy url",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "https://invalid proxy url with spaces",
					},
				},
			},
			expectedErrMsg: "invalid proxy URL",
		},
		{
			name: "invalid https proxy missing scheme or host",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "only-host-no-scheme",
					},
				},
			},
			expectedErrMsg: "invalid proxy URL",
		},
		{
			name: "invalid ca bundle - non-pem garbage",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "https://proxy.example.com:8443",
						CABundle:   []byte("this is not a pem certificate"),
					},
				},
			},
			expectedErrMsg: "invalid proxy CA bundle",
		},
		{
			name: "invalid ca bundle - non-certificate pem block",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "https://proxy.example.com:8443",
						CABundle: pem.EncodeToMemory(&pem.Block{
							Type:  "RSA PRIVATE KEY",
							Bytes: []byte("fake-key-data"),
						}),
					},
				},
			},
			expectedErrMsg: "invalid proxy CA bundle",
		},
		{
			name: "invalid ca bundle - corrupted certificate DER bytes",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "https://proxy.example.com:8443",
						CABundle: pem.EncodeToMemory(&pem.Block{
							Type:  "CERTIFICATE",
							Bytes: []byte("corrupted der bytes"),
						}),
					},
				},
			},
			expectedErrMsg: "invalid proxy CA bundle",
		},
		{
			name: "invalid ca bundle - valid certificate followed by truncated certificate",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "https://proxy.example.com:8443",
						CABundle:   append(generateTestCertificate(t, "valid-ca"), []byte("\n-----BEGIN CERTIFICATE-----\nincomplete-data")...),
					},
				},
			},
			expectedErrMsg: "invalid proxy CA bundle",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := BuildOptions(tc.addOnDeploy)
			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				if tc.addOnDeploy != nil {
					expectedPlatformAlerts := false
					expectedUserWorkloadAlerts := false
					for _, kv := range tc.addOnDeploy.Spec.CustomizedVariables {
						if kv.Name == KeyPlatformMetricsAlerts && kv.Value == "enabled" {
							expectedPlatformAlerts = true
						}
						if kv.Name == KeyUserWorkloadMetricsAlerts && kv.Value == "enabled" {
							expectedUserWorkloadAlerts = true
						}
					}
					tc.expectedOpts.Platform.Metrics.AlertsEnabled = expectedPlatformAlerts
					tc.expectedOpts.UserWorkloads.Metrics.AlertsEnabled = expectedUserWorkloadAlerts
				}
				assert.Equal(t, tc.expectedOpts, opts)
			}
		})
	}
}

func TestProxyConfig_ProxyURLForTarget(t *testing.T) {
	httpProxyURL, err := url.Parse("http://http-proxy.example.com:8080")
	require.NoError(t, err)
	httpsProxyURL, err := url.Parse("https://https-proxy.example.com:8443")
	require.NoError(t, err)

	tests := []struct {
		name        string
		proxyConfig ProxyConfig
		targetURL   string
		expectedURL *url.URL
	}{
		{
			name: "http target with both proxies set selects httpProxy",
			proxyConfig: ProxyConfig{
				HTTPProxy:  httpProxyURL,
				HTTPSProxy: httpsProxyURL,
			},
			targetURL:   "http://endpoint.example.com:9090",
			expectedURL: httpProxyURL,
		},
		{
			name: "http target with only httpsProxy set falls back to httpsProxy",
			proxyConfig: ProxyConfig{
				HTTPSProxy: httpsProxyURL,
			},
			targetURL:   "http://endpoint.example.com:9090",
			expectedURL: httpsProxyURL,
		},
		{
			name: "https target with both proxies set selects httpsProxy",
			proxyConfig: ProxyConfig{
				HTTPProxy:  httpProxyURL,
				HTTPSProxy: httpsProxyURL,
			},
			targetURL:   "https://endpoint.example.com:9090",
			expectedURL: httpsProxyURL,
		},
		{
			name: "https target with only httpProxy set falls back to httpProxy",
			proxyConfig: ProxyConfig{
				HTTPProxy: httpProxyURL,
			},
			targetURL:   "https://endpoint.example.com:9090",
			expectedURL: httpProxyURL,
		},
		{
			name: "empty target URL defaults to httpsProxy",
			proxyConfig: ProxyConfig{
				HTTPProxy:  httpProxyURL,
				HTTPSProxy: httpsProxyURL,
			},
			targetURL:   "",
			expectedURL: httpsProxyURL,
		},
		{
			name: "empty target URL with only httpProxy falls back to httpProxy",
			proxyConfig: ProxyConfig{
				HTTPProxy: httpProxyURL,
			},
			targetURL:   "",
			expectedURL: httpProxyURL,
		},
		{
			name: "no proxies set returns nil",
			proxyConfig: ProxyConfig{
				NoProxy: "*.example.com",
			},
			targetURL:   "https://endpoint.example.com",
			expectedURL: nil,
		},
		{
			name: "case insensitive scheme HTTP",
			proxyConfig: ProxyConfig{
				HTTPProxy:  httpProxyURL,
				HTTPSProxy: httpsProxyURL,
			},
			targetURL:   "HTTP://ENDPOINT.EXAMPLE.COM",
			expectedURL: httpProxyURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.proxyConfig.ProxyURLForTarget(tc.targetURL)
			assert.Equal(t, tc.expectedURL, actual)
		})
	}
}
