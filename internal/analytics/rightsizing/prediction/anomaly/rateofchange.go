package anomaly

import (
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// RateOfChangeDetector flags abrupt steps using the distribution of first differences.
type RateOfChangeDetector struct {
	cfg DetectorConfig
}

// NewRateOfChangeDetector constructs a RateOfChangeDetector with the given config.
func NewRateOfChangeDetector(cfg DetectorConfig) *RateOfChangeDetector {
	return &RateOfChangeDetector{cfg: cfg}
}

// Detect returns step anomalies where |derivative| exceeds mean + k*stddev of derivatives.
func (d *RateOfChangeDetector) Detect(points []prediction.DataPoint) []prediction.AnomalyResult {
	if len(points) < 2 {
		return nil
	}
	m := d.cfg.RateOfChangeMultiplier
	if m <= 0 {
		m = DefaultDetectorConfig().RateOfChangeMultiplier
	}
	diffs := make([]float64, 0, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		diffs = append(diffs, points[i+1].Value-points[i].Value)
	}
	meanD := meanFloat(diffs)
	stdD := stdDevPopulation(diffs)
	if stdD <= 0 {
		return nil
	}
	limit := meanD + m*stdD
	out := make([]prediction.AnomalyResult, 0)
	for i, dv := range diffs {
		ad := math.Abs(dv)
		if ad > limit {
			score := (ad - meanD) / stdD
			sev := "warning"
			if ad > 2*limit {
				sev = "critical"
			}
			out = append(out, prediction.AnomalyResult{
				Timestamp: points[i+1].Timestamp,
				Score:     score,
				Type:      "step",
				Severity:  sev,
			})
		}
	}
	return out
}
