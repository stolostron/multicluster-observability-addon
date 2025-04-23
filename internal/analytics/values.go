package analytics

import "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"

type AnalyticsValues struct {
	IncidentDetectionValues manifests.IncidentDetectionValues `json:"incidentDetection"`
}
