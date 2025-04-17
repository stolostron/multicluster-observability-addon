package addon

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
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
			name: "valid metrics",
			addOnDeploy: &addonapiv1alpha1.AddOnDeploymentConfig{
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
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
						HubEndpoint: &url.URL{
							Scheme: "https",
							Host:   "metrics.example.com",
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
			expectedErrMsg: "invalid metrics hub hostname: parse \"://invalid-url\": missing protocol scheme",
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
