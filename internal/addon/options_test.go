package addon

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
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
		expectedErr    error
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
			name: "valid COO catalog source override",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyCOOCatalogSource, Value: "cs-redhat-operator-index-v4-20"},
						{Name: KeyCOOCatalogSourceNamespace, Value: "openshift-marketplace"},
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
				COO: COOOptions{
					CatalogSource:          "cs-redhat-operator-index-v4-20",
					CatalogSourceNamespace: "openshift-marketplace",
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
			name: "workload and pod right-sizing enabled explicitly",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "disabled"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
						{Name: KeyPlatformWorkloadRightSizing, Value: "enabled"},
						{Name: KeyPlatformPodRightSizing, Value: "enabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							WorkloadEnabled: true,
							PodEnabled:      true,
						},
					},
				},
			},
		},
		{
			name: "namespace and virtualization disabled independently",
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
				Platform: PlatformOptions{Enabled: true, AnalyticsOptions: AnalyticsOptions{RightSizing: RightSizingOptions{}}},
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
				Platform:     PlatformOptions{Enabled: true, AnalyticsOptions: AnalyticsOptions{RightSizing: RightSizingOptions{}}},
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
				Platform: PlatformOptions{Enabled: true, AnalyticsOptions: AnalyticsOptions{RightSizing: RightSizingOptions{}}},
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
			name: "valid socks5 proxy",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPProxy: "socks5://socks-proxy.example.com:1080",
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
						Scheme: "socks5",
						Host:   "socks-proxy.example.com:1080",
					},
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
			name: "unsupported proxy scheme",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					ProxyConfig: addonapiv1beta1.ProxyConfig{
						HTTPSProxy: "ftp://proxy.example.com:21",
					},
				},
			},
			expectedErrMsg: "unsupported scheme \"ftp\" in proxy URL",
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
		{
			name: "valid options using true and enabled values",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: "true"},
						{Name: KeyMetricsHubHostname, Value: "https://metrics.example.com"},
						{Name: KeyPlatformLogsCollection, Value: "enabled"},
						{Name: KeyOpenShiftLoggingChannel, Value: "stable-6"},
						{Name: KeyPlatformIncidentDetection, Value: "true"},
						{Name: KeyPlatformNamespaceRightSizing, Value: "true"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "enabled"},
						{Name: KeyRightSizingDelegated, Value: "true"},
						{Name: KeyUserWorkloadMetricsCollection, Value: "enabled"},
						{Name: KeyUserWorkloadLogsCollection, Value: "true"},
						{Name: KeyUserWorkloadTracesCollection, Value: "enabled"},
						{Name: KeyUserWorkloadInstrumentation, Value: "true"},
						{Name: KeyPlatformMetricsUI, Value: "true"},
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
						UI: MetricsUIOptions{
							Enabled: true,
						},
					},
					Logs: LogsOptions{
						CollectionEnabled:   true,
						SubscriptionChannel: "stable-6",
					},
					AnalyticsOptions: AnalyticsOptions{
						IncidentDetection: IncidentDetection{
							Enabled: true,
						},
						RightSizing: RightSizingOptions{
							Delegated:             true,
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
					Logs: LogsOptions{
						CollectionEnabled:   true,
						SubscriptionChannel: "stable-6",
					},
					Traces: TracesOptions{
						CollectionEnabled:      true,
						InstrumentationEnabled: true,
					},
				},
			},
		},
		{
			name: "valid options using false and disabled values",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: "false"},
						{Name: KeyPlatformLogsCollection, Value: "disabled"},
						{Name: KeyPlatformIncidentDetection, Value: "false"},
						{Name: KeyPlatformNamespaceRightSizing, Value: "false"},
						{Name: KeyPlatformVirtualizationRightSizing, Value: "disabled"},
						{Name: KeyRightSizingDelegated, Value: "disabled"},
						{Name: KeyUserWorkloadMetricsCollection, Value: "disabled"},
						{Name: KeyUserWorkloadLogsCollection, Value: "false"},
						{Name: KeyUserWorkloadTracesCollection, Value: "disabled"},
						{Name: KeyUserWorkloadInstrumentation, Value: "false"},
						{Name: KeyPlatformMetricsUI, Value: "disabled"},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							Delegated:             false,
							NamespaceEnabled:      false,
							VirtualizationEnabled: false,
						},
					},
				},
				UserWorkloads: UserWorkloadOptions{},
			},
		},
		{
			name: "empty string values in customizedVariables cleanly disable features",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: ""},
						{Name: KeyPlatformLogsCollection, Value: ""},
						{Name: KeyPlatformIncidentDetection, Value: ""},
						{Name: KeyPlatformNamespaceRightSizing, Value: ""},
						{Name: KeyPlatformVirtualizationRightSizing, Value: ""},
						{Name: KeyRightSizingDelegated, Value: ""},
						{Name: KeyPlatformWorkloadRightSizing, Value: ""},
						{Name: KeyPlatformPodRightSizing, Value: ""},
						{Name: KeyUserWorkloadMetricsCollection, Value: ""},
						{Name: KeyUserWorkloadLogsCollection, Value: ""},
						{Name: KeyUserWorkloadTracesCollection, Value: ""},
						{Name: KeyUserWorkloadInstrumentation, Value: ""},
						{Name: KeyPlatformMetricsUI, Value: ""},
						{Name: KeyPlatformMetricsAlerts, Value: ""},
						{Name: KeyUserWorkloadMetricsAlerts, Value: ""},
					},
				},
			},
			expectedOpts: Options{
				Platform: PlatformOptions{
					Enabled: true,
					AnalyticsOptions: AnalyticsOptions{
						RightSizing: RightSizingOptions{
							Delegated:             false,
							NamespaceEnabled:      false,
							VirtualizationEnabled: false,
							WorkloadEnabled:       false,
							PodEnabled:            false,
						},
					},
				},
				UserWorkloads: UserWorkloadOptions{},
			},
		},
		{
			name: "platformMetricsUI enabled via true is overridden to false when metrics collection is disabled",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsUI, Value: "true"},
						{Name: KeyPlatformMetricsCollection, Value: "false"},
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
					Metrics: MetricsOptions{
						CollectionEnabled: false,
						UI:                MetricsUIOptions{Enabled: false},
					},
				},
			},
		},
		{
			name: "invalid value for userWorkloadMetricsCollection (yes)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyUserWorkloadMetricsCollection, Value: "yes"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "yes" for userWorkloadMetricsCollection`,
		},
		{
			name: "invalid value for platformMetricsCollection (on)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: "on"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "on" for platformMetricsCollection`,
		},
		{
			name: "invalid value for platformLogsCollection (typo)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformLogsCollection, Value: "typo"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "typo" for platformLogsCollection`,
		},
		{
			name: "invalid value for platformIncidentDetection (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformIncidentDetection, Value: "invalid"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "invalid" for platformIncidentDetection`,
		},
		{
			name: "invalid value for platformNamespaceRightSizing (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformNamespaceRightSizing, Value: "1"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "1" for platformNamespaceRightSizing`,
		},
		{
			name: "invalid value for platformVirtualizationRightSizing (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformVirtualizationRightSizing, Value: "maybe"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "maybe" for platformVirtualizationRightSizing`,
		},
		{
			name: "invalid value for rightSizingDelegated (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyRightSizingDelegated, Value: "not-a-bool"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "not-a-bool" for rightSizingDelegated`,
		},
		{
			name: "invalid value for platformWorkloadRightSizing (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformWorkloadRightSizing, Value: "bad"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "bad" for platformWorkloadRightSizing`,
		},
		{
			name: "invalid value for platformPodRightSizing (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformPodRightSizing, Value: "bad"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "bad" for platformPodRightSizing`,
		},
		{
			name: "invalid value for platformMetricsAlerts (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsAlerts, Value: "bad"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "bad" for platformMetricsAlerts`,
		},
		{
			name: "invalid value for userWorkloadMetricsAlerts (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyUserWorkloadMetricsAlerts, Value: "bad"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "bad" for userWorkloadMetricsAlerts`,
		},
		{
			name: "invalid value for userWorkloadLogsCollection (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyUserWorkloadLogsCollection, Value: "oui"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "oui" for userWorkloadLogsCollection`,
		},
		{
			name: "invalid value for userWorkloadTracesCollection (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyUserWorkloadTracesCollection, Value: "trueee"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "trueee" for userWorkloadTracesCollection`,
		},
		{
			name: "invalid value for userWorkloadInstrumentation (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyUserWorkloadInstrumentation, Value: "nope"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "nope" for userWorkloadInstrumentation`,
		},
		{
			name: "invalid value for platformMetricsUI (invalid)",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsUI, Value: "sure"},
					},
				},
			},
			expectedErr:    addoncfg.ErrInvalidConfigurationValue,
			expectedErrMsg: `unrecognized value "sure" for platformMetricsUI`,
		},
		{
			name: "platform metrics enabled via true but metricsHubHostname omitted",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: "true"},
					},
				},
			},
			expectedErr:    addoncfg.ErrMissingDependency,
			expectedErrMsg: "metricsHubHostname is required when metrics collection is enabled",
		},
		{
			name: "platform metrics enabled via legacy string but metricsHubHostname omitted",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: string(PrometheusAgentV1alpha1)},
					},
				},
			},
			expectedErr:    addoncfg.ErrMissingDependency,
			expectedErrMsg: "metricsHubHostname is required when metrics collection is enabled",
		},
		{
			name: "user workload metrics enabled via enabled but metricsHubHostname omitted",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyUserWorkloadMetricsCollection, Value: "enabled"},
					},
				},
			},
			expectedErr:    addoncfg.ErrMissingDependency,
			expectedErrMsg: "metricsHubHostname is required when metrics collection is enabled",
		},
		{
			name: "platform metrics enabled via true but metricsHubHostname is empty",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: "true"},
						{Name: KeyMetricsHubHostname, Value: ""},
					},
				},
			},
			expectedErr:    addoncfg.ErrMissingDependency,
			expectedErrMsg: "metricsHubHostname is required when metrics collection is enabled",
		},
		{
			name: "metrics disabled and metricsHubHostname is empty",
			addOnDeploy: &addonapiv1beta1.AddOnDeploymentConfig{
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1beta1.CustomizedVariable{
						{Name: KeyPlatformMetricsCollection, Value: "false"},
						{Name: KeyMetricsHubHostname, Value: ""},
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
					Metrics: MetricsOptions{
						CollectionEnabled: false,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := BuildOptions(tc.addOnDeploy)
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
			}
			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectedErrMsg)
			}
			if tc.expectedErr != nil || tc.expectedErrMsg != "" {
				return
			}
			require.NoError(t, err)
			if tc.addOnDeploy != nil {
				expectedPlatformAlerts := false
				expectedUserWorkloadAlerts := false
				for _, kv := range tc.addOnDeploy.Spec.CustomizedVariables {
					if kv.Name == KeyPlatformMetricsAlerts && (kv.Value == "enabled" || kv.Value == "true") {
						expectedPlatformAlerts = true
					}
					if kv.Name == KeyUserWorkloadMetricsAlerts && (kv.Value == "enabled" || kv.Value == "true") {
						expectedUserWorkloadAlerts = true
					}
				}
				tc.expectedOpts.Platform.Metrics.AlertsEnabled = expectedPlatformAlerts
				tc.expectedOpts.UserWorkloads.Metrics.AlertsEnabled = expectedUserWorkloadAlerts
			}
			assert.Equal(t, tc.expectedOpts, opts)
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

// TestIndependentRightSizingOptions verifies that workload and pod right-sizing
// are opt-in: they default to disabled when the keys are absent and are only
// enabled when MCO explicitly writes the corresponding key as "enabled".
func TestIndependentRightSizingOptions(t *testing.T) {
	for _, tt := range []struct {
		name          string
		variables     []addonapiv1beta1.CustomizedVariable
		workload, pod bool
	}{
		{"defaults", nil, false, false},
		{"workload enabled", []addonapiv1beta1.CustomizedVariable{{Name: KeyPlatformWorkloadRightSizing, Value: "enabled"}}, true, false},
		{"pod enabled", []addonapiv1beta1.CustomizedVariable{{Name: KeyPlatformPodRightSizing, Value: "enabled"}}, false, true},
		{"both disabled", []addonapiv1beta1.CustomizedVariable{{Name: KeyPlatformWorkloadRightSizing, Value: "disabled"}, {Name: KeyPlatformPodRightSizing, Value: "disabled"}}, false, false},
		{"both enabled", []addonapiv1beta1.CustomizedVariable{{Name: KeyPlatformWorkloadRightSizing, Value: "enabled"}, {Name: KeyPlatformPodRightSizing, Value: "enabled"}}, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := BuildOptions(&addonapiv1beta1.AddOnDeploymentConfig{Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{CustomizedVariables: tt.variables}})
			require.NoError(t, err)
			require.Equal(t, tt.workload, opts.Platform.AnalyticsOptions.RightSizing.WorkloadEnabled)
			require.Equal(t, tt.pod, opts.Platform.AnalyticsOptions.RightSizing.PodEnabled)
		})
	}
}

func TestParseFeatureFlag(t *testing.T) {
	testCases := []struct {
		name        string
		key         string
		value       string
		legacyValue string
		expected    bool
		expectErr   bool
	}{
		// Truth values
		{name: "lowercase true", key: "testKey", value: "true", expected: true},
		{name: "uppercase true", key: "testKey", value: "TRUE", expected: true},
		{name: "mixed case true", key: "testKey", value: "True", expected: true},
		{name: "trimmed true", key: "testKey", value: "  true  ", expected: true},
		{name: "lowercase enabled", key: "testKey", value: "enabled", expected: true},
		{name: "uppercase enabled", key: "testKey", value: "ENABLED", expected: true},
		{name: "mixed case enabled", key: "testKey", value: "Enabled", expected: true},
		{name: "trimmed enabled", key: "testKey", value: "  enabled  ", expected: true},

		// False values
		{name: "lowercase false", key: "testKey", value: "false", expected: false},
		{name: "uppercase false", key: "testKey", value: "FALSE", expected: false},
		{name: "mixed case false", key: "testKey", value: "False", expected: false},
		{name: "trimmed false", key: "testKey", value: "  false  ", expected: false},
		{name: "lowercase disabled", key: "testKey", value: "disabled", expected: false},
		{name: "uppercase disabled", key: "testKey", value: "DISABLED", expected: false},
		{name: "mixed case disabled", key: "testKey", value: "Disabled", expected: false},
		{name: "trimmed disabled", key: "testKey", value: "  disabled  ", expected: false},
		{name: "empty string", key: "testKey", value: "", expected: false},
		{name: "whitespace only", key: "testKey", value: "   ", expected: false},

		// Legacy values
		{
			name:        "matching legacy value",
			key:         KeyPlatformMetricsCollection,
			value:       string(PrometheusAgentV1alpha1),
			legacyValue: string(PrometheusAgentV1alpha1),
			expected:    true,
		},
		{
			name:        "matching legacy value uppercase (case-insensitive)",
			key:         KeyPlatformMetricsCollection,
			value:       strings.ToUpper(string(PrometheusAgentV1alpha1)),
			legacyValue: string(PrometheusAgentV1alpha1),
			expected:    true,
		},
		{
			name:        "matching legacy value with whitespace",
			key:         KeyPlatformLogsCollection,
			value:       " " + string(ClusterLogForwarderV1) + " ",
			legacyValue: string(ClusterLogForwarderV1),
			expected:    true,
		},
		{
			name:        "mismatched legacy value",
			key:         KeyPlatformMetricsCollection,
			value:       string(ClusterLogForwarderV1),
			legacyValue: string(PrometheusAgentV1alpha1),
			expectErr:   true,
		},

		// Invalid values
		{name: "invalid on", key: "testKey", value: "on", expectErr: true},
		{name: "invalid off", key: "testKey", value: "off", expectErr: true},
		{name: "invalid yes", key: "testKey", value: "yes", expectErr: true},
		{name: "invalid no", key: "testKey", value: "no", expectErr: true},
		{name: "invalid 1", key: "testKey", value: "1", expectErr: true},
		{name: "invalid 0", key: "testKey", value: "0", expectErr: true},
		{name: "invalid typo", key: "testKey", value: "typo", expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseFeatureFlag(tc.key, tc.value, tc.legacyValue)
			if tc.expectErr {
				require.Error(t, err)
				require.ErrorIs(t, err, addoncfg.ErrInvalidConfigurationValue)
				require.ErrorContains(t, err, fmt.Sprintf("unrecognized value %q for %s", tc.value, tc.key))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestParseBoolFlag(t *testing.T) {
	enabled, err := parseBoolFlag("testBoolKey", "true")
	require.NoError(t, err)
	assert.True(t, enabled)

	disabled, err := parseBoolFlag("testBoolKey", "false")
	require.NoError(t, err)
	assert.False(t, disabled)

	empty, err := parseBoolFlag("testBoolKey", "")
	require.NoError(t, err)
	assert.False(t, empty)

	_, err = parseBoolFlag("testBoolKey", "invalid-val")
	require.Error(t, err)
	require.ErrorIs(t, err, addoncfg.ErrInvalidConfigurationValue)
	require.ErrorContains(t, err, `unrecognized value "invalid-val" for testBoolKey`)
}
