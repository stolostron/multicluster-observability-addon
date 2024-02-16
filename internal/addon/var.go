package addon

import (
	"embed"
)

const (
	Name             = "multicluster-observability-addon"
	InstallNamespace = "open-cluster-management"

	McoaChartDir    = "manifests/charts/mcoa"
	MetricsChartDir = "manifests/charts/mcoa/charts/metrics"
	LoggingChartDir = "manifests/charts/mcoa/charts/logging"
	TracingChartDir = "manifests/charts/mcoa/charts/tracing"
	NetflowChartDir = "manifests/charts/mcoa/charts/netflow"

	ConfigMapResource             = "configmaps"
	SecretResource                = "secrets"
	AddonDeploymentConfigResource = "addondeploymentconfigs"

	AdcMetricsDisabledKey = "metricsDisabled"
	AdcLoggingDisabledKey = "loggingDisabled"
	AdcTracingisabledKey  = "tracingDisabled"
	AdcNetflowDisabledKey = "netflowDisabled"

	SignalLabelKey        = "mcoa.openshift.io/signal"
	Metrics        Signal = "metrics"
	Logging        Signal = "logging"
	Tracing        Signal = "tracing"
	Netflow        Signal = "netflow"
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/tracing/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/netflow/templates/_helpers.tpl
var FS embed.FS
