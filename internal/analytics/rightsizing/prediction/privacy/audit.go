package privacy

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// ConsentViolationsTotal counts consent checks that failed for a provider.
	ConsentViolationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rs_prediction_consent_violations_total",
			Help: "Number of consent validation failures by provider type.",
		},
		[]string{"provider_type"},
	)

	// PredictionAPICallsTotal counts outbound or inbound prediction API usage.
	PredictionAPICallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rs_prediction_api_calls_total",
			Help: "Total prediction API calls by provider and method.",
		},
		[]string{"provider_type", "method"},
	)

	// TrainingRunsTotal counts model training attempts.
	TrainingRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rs_prediction_training_runs_total",
			Help: "Total training runs by status (success or failure).",
		},
		[]string{"status"},
	)

	// LabelsRedactedTotal counts label redaction operations applied to sensitive maps.
	LabelsRedactedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rs_prediction_labels_redacted_total",
			Help: "Total times workload labels were redacted for privacy.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		ConsentViolationsTotal,
		PredictionAPICallsTotal,
		TrainingRunsTotal,
		LabelsRedactedTotal,
	)
}
