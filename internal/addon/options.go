package addon

import (
	"fmt"
	"net/url"
	"strings"

	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

const (
	// Operator Subscription Channels
	KeyOpenShiftLoggingChannel = "openshiftLoggingChannel"

	// Platform Observability Keys
	KeyPlatformMetricsCollection = "platformMetricsCollection"
	KeyPlatformLogsCollection    = "platformLogsCollection"
	KeyPlatformIncidentDetection = "platformIncidentDetection"
	KeyMetricsHubHostname        = "metricsHubHostname"

	// User Workloads Observability Keys
	KeyUserWorkloadMetricsCollection = "userWorkloadMetricsCollection"
	KeyUserWorkloadLogsCollection    = "userWorkloadLogsCollection"
	KeyUserWorkloadTracesCollection  = "userWorkloadTracesCollection"
	KeyUserWorkloadInstrumentation   = "userWorkloadInstrumentation"

	KeyObservabilityUIMetrics = "observabilityUIMetrics"
)

type CollectionKind string

const (
	ClusterLogForwarderV1         CollectionKind = "clusterlogforwarders.v1.observability.openshift.io"
	OpenTelemetryCollectorV1beta1 CollectionKind = "opentelemetrycollectors.v1beta1.opentelemetry.io"
	PrometheusAgentV1alpha1       CollectionKind = "prometheusagents.v1alpha1.monitoring.coreos.com"
)

type InstrumentationKind string

const (
	InstrumentationV1alpha1 InstrumentationKind = "instrumentations.v1alpha1.opentelemetry.io"
)

type UIKind string

const (
	UIPluginV1alpha1 UIKind = "uiplugins.v1alpha1.observability.openshift.io"
)

type MetricsOptions struct {
	CollectionEnabled bool
	HubEndpoint       *url.URL
	UI                MetricsUIOptions
}

type IncidentDetection struct {
	Enabled bool
}

type LogsOptions struct {
	CollectionEnabled   bool
	SubscriptionChannel string
}

type TracesOptions struct {
	CollectionEnabled      bool
	InstrumentationEnabled bool
	SubscriptionChannel    string
}

type PlatformOptions struct {
	Enabled          bool
	Metrics          MetricsOptions
	Logs             LogsOptions
	AnalyticsOptions AnalyticsOptions
}

type AnalyticsOptions struct {
	IncidentDetection IncidentDetection
}

type UserWorkloadOptions struct {
	Enabled bool
	Metrics MetricsOptions
	Logs    LogsOptions
	Traces  TracesOptions
}

type MetricsUIOptions struct {
	Enabled bool
	ACM     ACMOptions
	Perses  PersesOptions
}

type ACMOptions struct {
	Enabled bool
}

type PersesOptions struct {
	Enabled bool
}

type Options struct {
	Platform      PlatformOptions
	UserWorkloads UserWorkloadOptions
}

func BuildOptions(addOnDeployment *addonapiv1alpha1.AddOnDeploymentConfig) (Options, error) {
	var opts Options
	if addOnDeployment == nil {
		return opts, nil
	}

	if addOnDeployment.Spec.CustomizedVariables == nil {
		return opts, nil
	}

	for _, keyvalue := range addOnDeployment.Spec.CustomizedVariables {
		switch keyvalue.Name {
		// Operator Subscriptions
		case KeyOpenShiftLoggingChannel:
			opts.Platform.Logs.SubscriptionChannel = keyvalue.Value
			opts.UserWorkloads.Logs.SubscriptionChannel = keyvalue.Value
		// Platform Observability Options
		case KeyMetricsHubHostname:
			val := keyvalue.Value
			if !strings.HasPrefix(val, "http") {
				val = "https://" + val
			}
			url, err := url.Parse(val)
			if err != nil {
				return opts, fmt.Errorf("%w: %s", errInvalidMetricsHubHostname, err.Error())
			}
			url = url.JoinPath("/api/metrics/v1/default/api/v1/receive")

			// Hostname validation:
			// - Check if host is empty
			// - Check for invalid hostname formats like ":"
			if strings.TrimSpace(url.Host) == "" || url.Host == ":" || strings.HasPrefix(url.Host, ":") {
				return opts, fmt.Errorf("%w: invalid hostname format '%s'", errInvalidMetricsHubHostname, url.Host)
			}

			opts.Platform.Metrics.HubEndpoint = url
		case KeyPlatformMetricsCollection:
			if keyvalue.Value == string(PrometheusAgentV1alpha1) {
				opts.Platform.Enabled = true
				opts.Platform.Metrics.CollectionEnabled = true
			}
		case KeyPlatformLogsCollection:
			if keyvalue.Value == string(ClusterLogForwarderV1) {
				opts.Platform.Enabled = true
				opts.Platform.Logs.CollectionEnabled = true
			}
		case KeyPlatformIncidentDetection:
			if keyvalue.Value == string(UIPluginV1alpha1) {
				opts.Platform.Enabled = true
				opts.Platform.AnalyticsOptions.IncidentDetection.Enabled = true
			}
		// User Workload Observability Options
		case KeyUserWorkloadMetricsCollection:
			if keyvalue.Value == string(PrometheusAgentV1alpha1) {
				opts.UserWorkloads.Enabled = true
				opts.UserWorkloads.Metrics.CollectionEnabled = true
			}
		case KeyUserWorkloadLogsCollection:
			if keyvalue.Value == string(ClusterLogForwarderV1) {
				opts.UserWorkloads.Enabled = true
				opts.UserWorkloads.Logs.CollectionEnabled = true
			}
		case KeyUserWorkloadTracesCollection:
			if keyvalue.Value == string(OpenTelemetryCollectorV1beta1) {
				opts.UserWorkloads.Enabled = true
				opts.UserWorkloads.Traces.CollectionEnabled = true
			}
		case KeyUserWorkloadInstrumentation:
			if keyvalue.Value == string(InstrumentationV1alpha1) {
				opts.UserWorkloads.Enabled = true
				opts.UserWorkloads.Traces.InstrumentationEnabled = true
			}
			// Observability UI Options
		case KeyObservabilityUIMetrics:
			if keyvalue.Value == string(UIPluginV1alpha1) && opts.Platform.Metrics.CollectionEnabled {
				opts.Platform.Metrics.UI.Enabled = true
				opts.Platform.Metrics.UI.ACM.Enabled = true
				opts.Platform.Metrics.UI.Perses.Enabled = true
			}
		}
	}
	return opts, nil
}
