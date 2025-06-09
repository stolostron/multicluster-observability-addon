package addon

import (
	"embed"
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
//go:embed manifests/charts/mcoa/charts/coo/templates/_helpers.tpl

var FS embed.FS
