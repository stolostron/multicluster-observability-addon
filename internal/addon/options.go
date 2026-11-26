package addon

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

const (
	// Operator Subscription Channels
	KeyOpenShiftLoggingChannel   = "openshiftLoggingChannel"
	KeyCOOCatalogSource          = "cooCatalogSource"
	KeyCOOCatalogSourceNamespace = "cooCatalogSourceNamespace"

	// Platform Observability Keys
	KeyPlatformMetricsCollection         = "platformMetricsCollection"
	KeyPlatformLogsCollection            = "platformLogsCollection"
	KeyPlatformIncidentDetection         = "platformIncidentDetection"
	KeyPlatformNamespaceRightSizing      = "platformNamespaceRightSizing"
	KeyPlatformVirtualizationRightSizing = "platformVirtualizationRightSizing"
	KeyPlatformWorkloadRightSizing       = "platformWorkloadRightSizing"
	KeyPlatformPodRightSizing            = "platformPodRightSizing"
	KeyRightSizingDelegated              = "rightSizingDelegated"
	KeyMetricsHubHostname                = "metricsHubHostname"
	KeyNodeExporterHostPort              = "nodeExporterHostPort"
	KeyNodeExporterInternalPort          = "nodeExporterInternalPort"
	KeyPlatformMetricsAlerts             = "platformMetricsAlerts"

	// User Workloads Observability Keys
	KeyUserWorkloadMetricsCollection = "userWorkloadMetricsCollection"
	KeyUserWorkloadLogsCollection    = "userWorkloadLogsCollection"
	KeyUserWorkloadTracesCollection  = "userWorkloadTracesCollection"
	KeyUserWorkloadInstrumentation   = "userWorkloadInstrumentation"
	KeyUserWorkloadMetricsAlerts     = "userWorkloadMetricsAlerts"

	KeyPlatformMetricsUI          = "platformMetricsUI"
	KeyMetricsStorageInstanceSize = "metricsStorageInstanceSize"
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
	CollectionEnabled bool
	HubEndpoint       url.URL
	UI                MetricsUIOptions
	NodeExporter      NodeExporterOptions
	AlertsEnabled     bool
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
	Delegated             bool
	NamespaceEnabled      bool
	VirtualizationEnabled bool
	WorkloadEnabled       bool
	PodEnabled            bool
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
	HTTPProxy  *url.URL
	HTTPSProxy *url.URL
	CABundle   []byte
	NoProxy    string
}

// ProxyURLForTarget returns the appropriate proxy URL (*url.URL) for a given target URL string.
// If the target URL scheme is explicitly "http" (case-insensitive), it returns p.HTTPProxy (falling back to p.HTTPSProxy).
// Otherwise (for "https" or default/empty target scheme), it returns p.HTTPSProxy (falling back to p.HTTPProxy).
func (p ProxyConfig) ProxyURLForTarget(targetURL string) *url.URL {
	var scheme string
	if targetURL != "" {
		if u, err := url.Parse(targetURL); err == nil && u.Scheme != "" {
			scheme = strings.ToLower(u.Scheme)
		}
	}

	if scheme == "http" {
		if p.HTTPProxy != nil {
			return p.HTTPProxy
		}
		return p.HTTPSProxy
	}

	if p.HTTPSProxy != nil {
		return p.HTTPSProxy
	}
	return p.HTTPProxy
}

func parseProxyURL(rawURL string) (*url.URL, error) {
	if rawURL == "" {
		return nil, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", addoncfg.ErrInvalidProxyURL, err.Error())
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%w: missing scheme or host in proxy URL %q", addoncfg.ErrInvalidProxyURL, rawURL)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" {
		return nil, fmt.Errorf("%w: unsupported scheme %q in proxy URL %q (expected http, https, or socks5)", addoncfg.ErrInvalidProxyURL, u.Scheme, rawURL)
	}
	return u, nil
}

func validateCABundle(bundle []byte) error {
	var certCount int
	rest := bundle
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return fmt.Errorf("%w: expected CERTIFICATE block, got %s", addoncfg.ErrInvalidProxyCABundle, block.Type)
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return fmt.Errorf("%w: %s", addoncfg.ErrInvalidProxyCABundle, err.Error())
		}
		certCount++
	}
	if certCount == 0 {
		return fmt.Errorf("%w: no certificates found in CA bundle", addoncfg.ErrInvalidProxyCABundle)
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return fmt.Errorf("%w: trailing unparsed data in CA bundle", addoncfg.ErrInvalidProxyCABundle)
	}
	return nil
}

type COOOptions struct {
	CatalogSource          string
	CatalogSourceNamespace string
}

type Options struct {
	Platform              PlatformOptions
	UserWorkloads         UserWorkloadOptions
	COO                   COOOptions
	InstallNamespace      string
	InstanceSize          string
	Tolerations           []corev1.Toleration
	NodeSelector          map[string]string
	ResourceReqs          []addonapiv1beta1.ContainerResourceRequirements
	ProxyConfig           ProxyConfig
	Registries            []addonapiv1beta1.ImageMirror
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
		return fmt.Errorf("%w: %w: %s is required when metrics collection is enabled", addoncfg.ErrMissingDependency, addoncfg.ErrInvalidMetricsHubHostname, KeyMetricsHubHostname)
	}

	return nil
}

// BuildOptions parses an AddOnDeploymentConfig into addon Options.
func BuildOptions(addOnDeployment *addonapiv1beta1.AddOnDeploymentConfig) (Options, error) {
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

	var err error
	opts.ProxyConfig.HTTPProxy, err = parseProxyURL(addOnDeployment.Spec.ProxyConfig.HTTPProxy)
	if err != nil {
		return opts, err
	}

	opts.ProxyConfig.HTTPSProxy, err = parseProxyURL(addOnDeployment.Spec.ProxyConfig.HTTPSProxy)
	if err != nil {
		return opts, err
	}

	if len(addOnDeployment.Spec.ProxyConfig.CABundle) > 0 {
		if err := validateCABundle(addOnDeployment.Spec.ProxyConfig.CABundle); err != nil {
			return opts, err
		}
		opts.ProxyConfig.CABundle = bytes.Clone(addOnDeployment.Spec.ProxyConfig.CABundle)
	}

	opts.ProxyConfig.NoProxy = addOnDeployment.Spec.ProxyConfig.NoProxy
	opts.Registries = addOnDeployment.Spec.Registries
	opts.ThanosOperatorEnabled = addOnDeployment.Annotations["mcoa-thanos-operator"] == "true"

	// Default alerts to disabled
	opts.Platform.Metrics.AlertsEnabled = false
	opts.UserWorkloads.Metrics.AlertsEnabled = false

	// Do NOT return early when CustomizedVariables is nil. The for-range
	// loop below is a safe no-op on a nil slice, and we must always fall
	// through to the default right-sizing logic that follows it.

	// Track if right-sizing keys were explicitly set
	nsRSExplicitlySet := false
	virtRSExplicitlySet := false

	for _, keyvalue := range addOnDeployment.Spec.CustomizedVariables {
		switch keyvalue.Name {
		// Operator Subscriptions
		case KeyOpenShiftLoggingChannel:
			opts.Platform.Logs.SubscriptionChannel = keyvalue.Value
			opts.UserWorkloads.Logs.SubscriptionChannel = keyvalue.Value
		case KeyCOOCatalogSource:
			opts.COO.CatalogSource = keyvalue.Value
		case KeyCOOCatalogSourceNamespace:
			opts.COO.CatalogSourceNamespace = keyvalue.Value
		// Platform Observability Options
		case KeyMetricsHubHostname:
			if strings.TrimSpace(keyvalue.Value) == "" {
				continue
			}
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
		case KeyPlatformMetricsAlerts:
			enabled, err := parseBoolFlag(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.Metrics.AlertsEnabled = enabled
		case KeyUserWorkloadMetricsAlerts:
			enabled, err := parseBoolFlag(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.UserWorkloads.Metrics.AlertsEnabled = enabled
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
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(PrometheusAgentV1alpha1))
			if err != nil {
				return opts, err
			}
			opts.Platform.Metrics.CollectionEnabled = enabled
			if enabled {
				opts.Platform.Enabled = true
			}
		case KeyPlatformLogsCollection:
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(ClusterLogForwarderV1))
			if err != nil {
				return opts, err
			}
			opts.Platform.Logs.CollectionEnabled = enabled
			if enabled {
				opts.Platform.Enabled = true
			}
		case KeyPlatformIncidentDetection:
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(UIPluginV1alpha1))
			if err != nil {
				return opts, err
			}
			opts.Platform.AnalyticsOptions.IncidentDetection.Enabled = enabled
			if enabled {
				opts.Platform.Enabled = true
			}
		case KeyRightSizingDelegated:
			enabled, err := parseBoolFlag(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.AnalyticsOptions.RightSizing.Delegated = enabled
		case KeyPlatformNamespaceRightSizing:
			nsRSExplicitlySet = true
			// Always mark platform as enabled when RS key is present (even "disabled").
			// This ensures the rendering pipeline runs so the addon framework can prune
			// stale ManifestWork content when both RS features are disabled.
			opts.Platform.Enabled = true
			enabled, err := parseBoolFlag(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled = enabled
		case KeyPlatformVirtualizationRightSizing:
			virtRSExplicitlySet = true
			opts.Platform.Enabled = true
			enabled, err := parseBoolFlag(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled = enabled
		case KeyPlatformWorkloadRightSizing:
			opts.Platform.Enabled = true
			enabled, err := parseBoolFlag(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.AnalyticsOptions.RightSizing.WorkloadEnabled = enabled
		case KeyPlatformPodRightSizing:
			opts.Platform.Enabled = true
			enabled, err := parseBoolFlag(keyvalue.Name, keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.Platform.AnalyticsOptions.RightSizing.PodEnabled = enabled
		// User Workload Observability Options
		case KeyUserWorkloadMetricsCollection:
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(PrometheusAgentV1alpha1))
			if err != nil {
				return opts, err
			}
			opts.UserWorkloads.Metrics.CollectionEnabled = enabled
			if enabled {
				opts.UserWorkloads.Enabled = true
			}
		case KeyUserWorkloadLogsCollection:
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(ClusterLogForwarderV1))
			if err != nil {
				return opts, err
			}
			opts.UserWorkloads.Logs.CollectionEnabled = enabled
			if enabled {
				opts.UserWorkloads.Enabled = true
			}
		case KeyUserWorkloadTracesCollection:
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(OpenTelemetryCollectorV1beta1))
			if err != nil {
				return opts, err
			}
			opts.UserWorkloads.Traces.CollectionEnabled = enabled
			if enabled {
				opts.UserWorkloads.Enabled = true
			}
		case KeyUserWorkloadInstrumentation:
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(InstrumentationV1alpha1))
			if err != nil {
				return opts, err
			}
			opts.UserWorkloads.Traces.InstrumentationEnabled = enabled
			if enabled {
				opts.UserWorkloads.Enabled = true
			}
		// Observability UI Options
		case KeyPlatformMetricsUI:
			enabled, err := parseFeatureFlag(keyvalue.Name, keyvalue.Value, string(UIPluginV1alpha1))
			if err != nil {
				return opts, err
			}
			opts.Platform.Metrics.UI.Enabled = enabled
		case KeyMetricsStorageInstanceSize:
			opts.InstanceSize = keyvalue.Value
			// Note on unknown keys:
			// Unknown customized variables are intentionally ignored (no default error in this switch)
			// to preserve forward compatibility with newer MCO versions writing keys unrecognized by this MCOA build.
		}
	}

	// Auto-enable namespace and virtualization right-sizing when their keys are
	// not yet present in ADC.
	//
	// This is a bootstrap safety net for the window between ADC creation (by MCO
	// main controller) and first analytics controller reconcile (which calls
	// syncRightSizingStateToADC to explicitly set the keys).
	//
	// Once syncRightSizingStateToADC runs, the keys are always present:
	// - MCO mode: all keys set to "disabled" (MCO manages RS via Policy)
	// - MCOA mode: keys set to "enabled"/"disabled" based on MCO CR spec
	//
	// Workload and pod right-sizing are opt-in: they stay disabled until MCO
	// writes their keys as "enabled".
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

// parseBoolFlag evaluates boolean/status strings for keys that do not have a legacy Kubernetes API string.
func parseBoolFlag(key, value string) (bool, error) {
	return parseFeatureFlag(key, value, "")
}

// parseFeatureFlag evaluates boolean/status strings for AddOnDeploymentConfig variables.
// To ensure backward compatibility with earlier MCOA releases, it accepts legacy Kubernetes
// API resource strings (e.g. "prometheusagents.v1alpha1.monitoring.rhobs") as enabled values,
// alongside intuitive truth values ("true", "enabled") and false values ("false", "disabled").
// An empty value ("" or whitespace only) is treated as false to prevent fatal errors when
// variables are defined without an explicit value in the AddOnDeploymentConfig.
// Unrecognized non-empty values return ErrInvalidConfigurationValue to avoid silent misconfiguration.
func parseFeatureFlag(key, value, legacyValue string) (bool, error) {
	val := strings.TrimSpace(value)
	if legacyValue != "" && (val == legacyValue || strings.EqualFold(val, legacyValue)) {
		return true, nil
	}

	switch strings.ToLower(val) {
	case "true", "enabled":
		return true, nil
	case "false", "disabled", "":
		return false, nil
	default:
		return false, fmt.Errorf("%w: unrecognized value %q for %s", addoncfg.ErrInvalidConfigurationValue, value, key)
	}
}
