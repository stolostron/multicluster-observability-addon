package addon

import (
	"embed"
	"errors"
)

const (
	Name              = "multicluster-observability-addon"
	LabelOCMAddonName = "open-cluster-management.io/addon-name"
	InstallNamespace  = "open-cluster-management-observability"
	HubNamespace      = "local-cluster"

	McoaChartDir              = "manifests/charts/mcoa"
	MetricsChartDir           = "manifests/charts/mcoa/charts/metrics"
	LoggingChartDir           = "manifests/charts/mcoa/charts/logging"
	TracingChartDir           = "manifests/charts/mcoa/charts/tracing"
	IncidentDetectionChartDir = "manifests/charts/mcoa/charts/analytics/charts/incident-detection"

	AddonDeploymentConfigResource = "addondeploymentconfigs"

	cooSubscriptionName      = "cluster-observability-operator"
	cooSubscriptionNamespace = "openshift-cluster-observability-operator"
	cooSubscriptionChannel   = "stable"

	paProbeKey  = "isAvailable"
	paProbePath = ".status.conditions[?(@.type==\"Available\")].status"

	ClusterLogForwardersResource = "clusterlogforwarders"
	LokiStacksResource           = "lokistacks"
	SpokeUnmanagedCLFName        = "mcoa-instance"
	SpokeDefaultStackCLFName     = "mcoa-managed-instance"
	SpokeDefaultStackLSName      = "mcoa-managed-instance"
	LoggingNamespace             = "openshift-logging"
	clfProbeKey                  = "isReady"
	clfProbePath                 = ".status.conditions[?(@.type==\"Ready\")].status"

	OpenTelemetryCollectorsResource = "opentelemetrycollectors"
	InstrumentationResource         = "instrumentations"
	SpokeOTELColName                = "mcoa-instance"
	SpokeInstrumentationName        = "mcoa-instance"
	IDetectionUIPluginName          = "monitoring"
	SpokeOTELColNamespace           = "mcoa-opentelemetry"
	otelColProbeKey                 = "replicas"
	otelColProbePath                = ".spec.replicas"

	uiPluginsResource = "uiplugins"
	uipProbeKey       = "isAvailable"
	uipProbePath      = ".status.conditions[?(@.type==\"Available\")].status"

	DefaultStackPrefix = "default-stack-instance"
)

var (
	errInvalidMetricsHubHostname  = errors.New("invalid metrics hub hostname")
	errInvalidSubscriptionChannel = errors.New("current version of the cluster-observability-operator installed doesn't match the supported MCOA version")
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/charts/unmanaged/charts/collection/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/charts/managed/charts/collection/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/charts/managed/charts/storage/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/tracing/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/analytics/charts/incident-detection/templates/_helpers.tpl
var FS embed.FS
