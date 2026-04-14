package addon

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

const (
	// Operator Subscription Channels
	KeyOpenShiftLoggingChannel = "openshiftLoggingChannel"

	// Platform Observability Keys
	KeyPlatformMetricsCollection         = "platformMetricsCollection"
	KeyPlatformLogsCollection            = "platformLogsCollection"
	KeyPlatformIncidentDetection         = "platformIncidentDetection"
	KeyPlatformNamespaceRightSizing      = "platformNamespaceRightSizing"
	KeyPlatformVirtualizationRightSizing = "platformVirtualizationRightSizing"
	KeyMetricsHubHostname                = "metricsHubHostname"
	KeyMetricsAlertManagerHostname       = "metricsAlertManagerHostname"
	KeyNodeExporterHostPort              = "nodeExporterHostPort"
	KeyNodeExporterInternalPort          = "nodeExporterInternalPort"

	// User Workloads Observability Keys
	KeyUserWorkloadMetricsCollection = "userWorkloadMetricsCollection"
	KeyUserWorkloadLogsCollection    = "userWorkloadLogsCollection"
	KeyUserWorkloadTracesCollection  = "userWorkloadTracesCollection"
	KeyUserWorkloadInstrumentation   = "userWorkloadInstrumentation"

	KeyPlatformMetricsUI = "platformMetricsUI"
)

type CollectionKind string

const (
	ClusterLogForwarderV1         CollectionKind = "clusterlogforwarders.v1.observability.openshift.io"
	OpenTelemetryCollectorV1beta1 CollectionKind = "opentelemetrycollectors.v1beta1.opentelemetry.io"
	PrometheusAgentV1alpha1       CollectionKind = "prometheusagents.v1alpha1.monitoring.rhobs"
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
	CollectionEnabled    bool
	HubEndpoint          url.URL
	AlertManagerEndpoint url.URL
	UI                   MetricsUIOptions
	NodeExporter         NodeExporterOptions
}

type NodeExporterOptions struct {
	HostPort     int32
	InternalPort int32
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

type RightSizingOptions struct {
	NamespaceEnabled      bool
	VirtualizationEnabled bool
}

type AnalyticsOptions struct {
	IncidentDetection IncidentDetection
	RightSizing       RightSizingOptions
}

type UserWorkloadOptions struct {
	Enabled bool
	Metrics MetricsOptions
	Logs    LogsOptions
	Traces  TracesOptions
}

type MetricsUIOptions struct {
	Enabled bool
}

type ProxyConfig struct {
	ProxyURL *url.URL
	NoProxy  string
}

type Options struct {
	Platform              PlatformOptions
	UserWorkloads         UserWorkloadOptions
	InstallNamespace      string
	Tolerations           []corev1.Toleration
	NodeSelector          map[string]string
	ResourceReqs          []addonapiv1alpha1.ContainerResourceRequirements
	ProxyConfig           ProxyConfig
	Registries            []addonapiv1alpha1.ImageMirror
	ThanosOperatorEnabled bool
}

func (o Options) validate() error {
	if err := o.validateMetrics(); err != nil {
		return err
	}

	return nil
}

func (o Options) validateMetrics() error {
	if !o.Platform.Metrics.CollectionEnabled && !o.UserWorkloads.Metrics.CollectionEnabled {
		return nil
	}

	if o.Platform.Metrics.HubEndpoint.Host == "" {
		return addoncfg.ErrInvalidMetricsHubHostname
	}

	return nil
}

func BuildOptions(addOnDeployment *addonapiv1alpha1.AddOnDeploymentConfig) (Options, error) {
	var opts Options
	if addOnDeployment == nil {
		return opts, nil
	}

	opts.InstallNamespace = addOnDeployment.Spec.AgentInstallNamespace
	if addOnDeployment.Spec.NodePlacement != nil {
		opts.NodeSelector = addOnDeployment.Spec.NodePlacement.NodeSelector
		opts.Tolerations = addOnDeployment.Spec.NodePlacement.Tolerations
	}

	if addOnDeployment.Spec.ResourceRequirements != nil {
		opts.ResourceReqs = addOnDeployment.Spec.ResourceRequirements
	}

	if addOnDeployment.Spec.ProxyConfig.HTTPProxy != "" {
		proxyURL, err := url.Parse(addOnDeployment.Spec.ProxyConfig.HTTPProxy)
		if err != nil {
			return opts, fmt.Errorf("%w: %s", addoncfg.ErrInvalidProxyURL, err.Error())
		}
		opts.ProxyConfig.ProxyURL = proxyURL
	}

	opts.ProxyConfig.NoProxy = addOnDeployment.Spec.ProxyConfig.NoProxy
	opts.Registries = addOnDeployment.Spec.Registries

	// Do NOT return early when CustomizedVariables is nil. The for-range
	// loop below is a safe no-op on a nil slice, and we must always fall
	// through to the auto-enable right-sizing logic that follows it.

	// Track if right-sizing keys were explicitly set
	nsRSExplicitlySet := false
	virtRSExplicitlySet := false

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
				return opts, fmt.Errorf("%w: %s", addoncfg.ErrInvalidMetricsHubHostname, err.Error())
			}
			url = url.JoinPath("/api/metrics/v1/default/api/v1/receive")

			// Hostname validation:
			// - Check if host is empty
			// - Check for invalid hostname formats like ":"
			if strings.TrimSpace(url.Host) == "" || url.Host == ":" || strings.HasPrefix(url.Host, ":") {
				return opts, fmt.Errorf("%w: invalid hostname format '%s'", addoncfg.ErrInvalidMetricsHubHostname, url.Host)
			}

			opts.Platform.Metrics.HubEndpoint = *url
		case KeyMetricsAlertManagerHostname:
			val := keyvalue.Value
			if !strings.HasPrefix(val, "http") {
				val = "https://" + val
			}
			url, err := url.Parse(val)
			if err != nil {
				return opts, fmt.Errorf("%w: %s", addoncfg.ErrInvalidMetricsAlertManagerHostname, err.Error())
			}

			// Hostname validation:
			// - Check if host is empty
			// - Check for invalid hostname formats like ":"
			if strings.TrimSpace(url.Host) == "" || url.Host == ":" || strings.HasPrefix(url.Host, ":") {
				return opts, fmt.Errorf("%w: invalid hostname format '%s'", addoncfg.ErrInvalidMetricsAlertManagerHostname, url.Host)
			}

			opts.Platform.Metrics.AlertManagerEndpoint = *url
		case KeyNodeExporterHostPort:
			port, err := parsePort(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.Metrics.NodeExporter.HostPort = port
		case KeyNodeExporterInternalPort:
			port, err := parsePort(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.Metrics.NodeExporter.InternalPort = port
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
		case KeyPlatformNamespaceRightSizing:
			nsRSExplicitlySet = true
			// Always mark platform as enabled when RS key is present (even "disabled").
			// This ensures the rendering pipeline runs so the addon framework can prune
			// stale ManifestWork content when both RS features are disabled.
			opts.Platform.Enabled = true
			if keyvalue.Value == "enabled" {
				opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled = true
			}
		case KeyPlatformVirtualizationRightSizing:
			virtRSExplicitlySet = true
			opts.Platform.Enabled = true
			if keyvalue.Value == "enabled" {
				opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled = true
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
		case KeyPlatformMetricsUI:
			if keyvalue.Value == string(UIPluginV1alpha1) {
				opts.Platform.Metrics.UI.Enabled = true
			}
		}
	}

	// Auto-enable right-sizing by default when keys are not yet present in ADC.
	//
	// This is a bootstrap safety net for the window between ADC creation (by MCO
	// main controller) and first analytics controller reconcile (which calls
	// syncRightSizingStateToADC to explicitly set the keys).
	//
	// Once syncRightSizingStateToADC runs, the keys are always present:
	// - MCO mode: both keys set to "disabled" (MCO manages RS via Policy)
	// - MCOA mode: keys set to "enabled"/"disabled" based on MCO CR spec
	//
	// The auto-enable ensures right-sizing works immediately on fresh installs
	// before the analytics controller has had time to sync state to ADC.
	if !nsRSExplicitlySet {
		opts.Platform.Enabled = true
		opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled = true
	}
	if !virtRSExplicitlySet {
		opts.Platform.Enabled = true
		opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled = true
	}

	if !opts.Platform.Metrics.CollectionEnabled {
		opts.Platform.Metrics.UI.Enabled = false
	}

	return opts, opts.validate()
}

func parsePort(name, value string) (int32, error) {
	port, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid port format for %s: %w", name, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%w: %d for %s must be between 1 and 65535", addoncfg.ErrInvalidPort, port, name)
	}
	return int32(port), nil
}
