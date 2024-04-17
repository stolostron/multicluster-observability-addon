package addon

import (
	"embed"
)

const (
	Name             = "multicluster-observability-addon"
	InstallNamespace = "open-cluster-management"

	McoaChartDir          = "manifests/charts/mcoa"
	MetricsChartDir       = "manifests/charts/mcoa/charts/metrics"
	LoggingChartDir       = "manifests/charts/mcoa/charts/logging"
	OpenTelemetryChartDir = "manifests/charts/mcoa/charts/opentelemetry"

	ConfigMapResource             = "configmaps"
	SecretResource                = "secrets"
	AddonDeploymentConfigResource = "addondeploymentconfigs"

	AdcMetricsDisabledKey       = "metricsDisabled"
	AdcLoggingDisabledKey       = "loggingDisabled"
	AdcOpenTelemetryDisabledKey = "opentelemetryDisabled"

	SignalLabelKey        = "mcoa.openshift.io/signal"
	Metrics        Signal = "metrics"
	Logging        Signal = "logging"
	OpenTelemetry  Signal = "opentelemetry"
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/opentelemetry/templates/_helpers.tpl
var FS embed.FS
