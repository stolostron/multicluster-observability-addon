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

	ClfGroup         = "logging.openshift.io"
	ClfResource      = "clusterlogforwarders"
	ClfName          = "instance"
	ClusterLoggingNS = "openshift-logging"

	OtelcolGroup    = "opentelemetry.io"
	OtelcolResource = "opentelemetrycollectors"
	OtelcolName     = "spoke-otelcol"
	OtelcolNS       = "spoke-otelcol"
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/tracing/templates/_helpers.tpl
var FS embed.FS
