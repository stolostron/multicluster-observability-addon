package addon

import (
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

const (
	KeyHubHostname = "hubHostname"

	// Operator Subscription Channels
	KeyOpenShiftLoggingChannel = "openshiftLoggingChannel"

	// Platform Observability Keys
	KeyPlatformLogsCollection = "platformLogsCollection"
	KeyPlatformLogsDefault    = "platformLogsDefault"

	// User Workloads Observability Keys
	KeyUserWorkloadLogsCollection   = "userWorkloadLogsCollection"
	KeyUserWorkloadTracesCollection = "userWorkloadTracesCollection"
	KeyUserWorkloadInstrumentation  = "userWorkloadInstrumentation"
)

type CollectionKind string

const (
	ClusterLogForwarderV1         CollectionKind = "clusterlogforwarders.v1.observability.openshift.io"
	OpenTelemetryCollectorV1beta1 CollectionKind = "opentelemetrycollectors.v1beta1.opentelemetry.io"
)

type InstrumentationKind string

const (
	InstrumentationV1alpha1 InstrumentationKind = "instrumentations.v1alpha1.opentelemetry.io"
)

type LogsOptions struct {
	CollectionEnabled   bool
	SubscriptionChannel string
	ManagedStack        bool
}

type TracesOptions struct {
	CollectionEnabled      bool
	InstrumentationEnabled bool
	SubscriptionChannel    string
}

type PlatformOptions struct {
	Enabled bool
	Logs    LogsOptions
}

type UserWorkloadOptions struct {
	Enabled bool
	Logs    LogsOptions
	Traces  TracesOptions
}

type Options struct {
	HubHostname   string
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
		case KeyHubHostname:
			opts.HubHostname = keyvalue.Value
		// Operator Subscriptions
		case KeyOpenShiftLoggingChannel:
			opts.Platform.Logs.SubscriptionChannel = keyvalue.Value
			opts.UserWorkloads.Logs.SubscriptionChannel = keyvalue.Value
		// Platform Observability Options
		case KeyPlatformLogsCollection:
			if keyvalue.Value == string(ClusterLogForwarderV1) {
				opts.Platform.Enabled = true
				opts.Platform.Logs.CollectionEnabled = true
			}
		case KeyPlatformLogsDefault:
			// TODO(JoaoBraveCoding): we need to review what the value should be
			if keyvalue.Value == "true" {
				opts.Platform.Enabled = true
				opts.Platform.Logs.ManagedStack = true
			}
		// User Workload Observability Options
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
