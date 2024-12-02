package addon

import (
	"strings"

	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

const (
	// Operator Subscription Channels
	KeyOpenShiftLoggingChannel = "openshiftLoggingChannel"

	// Platform Observability Keys
	KeyPlatformMetricsCollection = "platformMetricsCollection"
	KeyPlatformLogsCollection    = "platformLogsCollection"
	KeyMetricsHubHostname        = "metricsHubHostname"

	// User Workloads Observability Keys
	KeyUserWorkloadMetricsCollection = "userWorkloadMetricsCollection"
	KeyUserWorkloadLogsCollection    = "userWorkloadLogsCollection"
	KeyUserWorkloadTracesCollection  = "userWorkloadTracesCollection"
	KeyUserWorkloadInstrumentation   = "userWorkloadInstrumentation"
)

type CollectionKind string

const (
	ClusterLogForwarderV1                   CollectionKind = "clusterlogforwarders.v1.observability.openshift.io"
	OpenTelemetryCollectorV1beta1           CollectionKind = "opentelemetrycollectors.v1beta1.opentelemetry.io"
	PrometheusAgentMetricsCollectorV1alpha1 CollectionKind = "prometheusagents.v1alpha1.monitoring.coreos.com"
)

type InstrumentationKind string

const (
	InstrumentationV1alpha1 InstrumentationKind = "instrumentations.v1alpha1.opentelemetry.io"
)

type MetricsOptions struct {
	CollectionEnabled bool
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
	Enabled     bool
	Metrics     MetricsOptions
	Logs        LogsOptions
	HubEndpoint string
}

type UserWorkloadOptions struct {
	Enabled bool
	Metrics MetricsOptions
	Logs    LogsOptions
	Traces  TracesOptions
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
			if !strings.HasPrefix(keyvalue.Value, "http") {
				opts.Platform.HubEndpoint = "https://" + keyvalue.Value
			} else {
				opts.Platform.HubEndpoint = keyvalue.Value
			}
		case KeyPlatformMetricsCollection:
			if keyvalue.Value == string(PrometheusAgentMetricsCollectorV1alpha1) {
				opts.Platform.Enabled = true
				opts.Platform.Metrics.CollectionEnabled = true
			}
		case KeyPlatformLogsCollection:
			if keyvalue.Value == string(ClusterLogForwarderV1) {
				opts.Platform.Enabled = true
				opts.Platform.Metrics.CollectionEnabled = true
			}
		// User Workload Observability Options
		case KeyUserWorkloadMetricsCollection:
			if keyvalue.Value == string(PrometheusAgentMetricsCollectorV1alpha1) {
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
		}
	}
	return opts, nil
}
