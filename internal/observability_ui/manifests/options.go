package manifests

import "github.com/stolostron/multicluster-observability-addon/internal/addon"

type Options struct {
	Enabled   bool
	LogsUI    addon.LogsUIOptions
	MetricsUI addon.MetricsUIOptions
}
