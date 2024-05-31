package addon

import (
	"embed"
)

const (
	Name             = "multicluster-observability-addon"
	InstallNamespace = "open-cluster-management"

	McoaChartDir    = "manifests/charts/mcoa"
	LoggingChartDir = "manifests/charts/mcoa/charts/logging"
	TracingChartDir = "manifests/charts/mcoa/charts/tracing"

	AddonDeploymentConfigResource = "addondeploymentconfigs"

	AdcLoggingDisabledKey = "loggingDisabled"
	AdcTracingisabledKey  = "tracingDisabled"

	ClusterLogForwardersResource = "clusterlogforwarders"
	SpokeCLFName                 = "instance"
	SpokeCLFNamespace            = "openshift-logging"
	clfProbeKey                  = "isReady"
	clfProbePath                 = ".status.conditions[?(@.type==\"Ready\")].status"

	OpenTelemetryCollectorsResource = "opentelemetrycollectors"
	SpokeOTELColName                = "spoke-otelcol"
	SpokeOTELColNamespace           = "spoke-otelcol"
	otelColProbeKey                 = "replicas"
	otelColProbePath                = ".spec.replicas"
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/tracing/templates/_helpers.tpl
var FS embed.FS
