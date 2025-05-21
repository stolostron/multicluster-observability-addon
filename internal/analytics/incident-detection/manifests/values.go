package manifests

import "github.com/stolostron/multicluster-observability-addon/internal/addon"

type IncidentDetectionValues struct {
	Enabled bool `json:"enabled"`
}

func EnableUI(opts addon.IncidentDetection) *IncidentDetectionValues {
	if !opts.Enabled {
		return nil
	}
	return &IncidentDetectionValues{
		Enabled: true,
	}
}
