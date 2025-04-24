package manifests

import (
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
)

type UIValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(obsUI addon.ObsUIOptions) (UIValues, error) {
	return UIValues{obsUI.Enabled}, nil
}
