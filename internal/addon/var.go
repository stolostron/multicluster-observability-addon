package addon

import (
	"embed"
)

const (
	Name              = "multicluster-observability-addon"
	LabelOCMAddonName = "open-cluster-management.io/addon-name"
	InstallNamespace  = "open-cluster-management-observability"
	HubNamespace      = "local-cluster"

	McoaChartDir    = "manifests/charts/mcoa"
	LoggingChartDir = "manifests/charts/mcoa/charts/logging"
	TracingChartDir = "manifests/charts/mcoa/charts/tracing"

	AddonDeploymentConfigResource = "addondeploymentconfigs"
	ClusterLogForwardersResource  = "clusterlogforwarders"
	LokiStacksResource            = "lokistacks"
	SpokeUnmanagedCLFName         = "mcoa-instance"
	SpokeManagedCLFName           = "mcoa-managed-instance"
	SpokeManagedLSName            = "mcoa-managed-instance"
	LoggingNamespace              = "openshift-logging"
	clfProbeKey                   = "isReady"
	// TODO @JoaoBraveCoding this most likely needs to be updated to reflect the new path
	clfProbePath = ".status.conditions[?(@.type==\"Ready\")].status"

	OpenTelemetryCollectorsResource = "opentelemetrycollectors"
	InstrumentationResource         = "instrumentations"
	SpokeOTELColName                = "mcoa-instance"
	SpokeInstrumentationName        = "mcoa-instance"
	SpokeOTELColNamespace           = "mcoa-opentelemetry"
	otelColProbeKey                 = "replicas"
	otelColProbePath                = ".spec.replicas"

	DefaultStackPrefix = "default-stack-instance"
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/charts/unmanaged/charts/collection/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/charts/managed/charts/collection/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/charts/managed/charts/storage/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/tracing/templates/_helpers.tpl
var FS embed.FS
