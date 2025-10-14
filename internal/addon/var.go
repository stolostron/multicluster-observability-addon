package addon

import (
	"embed"
)

//go:embed manifests
//go:embed manifests/charts/mcoa
//go:embed manifests/charts/mcoa/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/logging/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/tracing/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/coo/templates/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/non-ocp/monitoring/kube-state-metrics/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/non-ocp/monitoring/node-exporter/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/non-ocp/monitoring/prometheus/rules/_helpers.tpl
//go:embed manifests/charts/mcoa/charts/metrics/templates/non-ocp/monitoring/prometheus/server/_helpers.tpl

var FS embed.FS
