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
	SpokeCLFName                 = "mcoa-instance"
	SpokeCLFNamespace            = "openshift-logging"
	clfProbeKey                  = "isReady"
	clfProbePath                 = ".status.conditions[?(@.type==\"Ready\")].status"

	OpenTelemetryCollectorsResource = "opentelemetrycollectors"
	SpokeOTELColName                = "mcoa-instance"
	SpokeOTELColNamespace           = "mcoa-opentelemetry"
	otelColProbeKey                 = "replicas"
	otelColProbePath                = ".spec.replicas"
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/tracing/templates/_helpers.tpl
var FS embed.FS
