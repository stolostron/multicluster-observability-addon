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
//go:embed manifests/charts/mcoa/charts/analytics/charts/incident-detection/templates/_helpers.tpl
var FS embed.FS
