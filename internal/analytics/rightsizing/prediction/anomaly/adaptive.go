package anomaly

import (
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// AdaptiveThresholdDetector uses a rolling percentile of recent values to detect drift.
type AdaptiveThresholdDetector struct {
	cfg DetectorConfig
}

// NewAdaptiveThresholdDetector constructs an AdaptiveThresholdDetector with the given config.
func NewAdaptiveThresholdDetector(cfg DetectorConfig) *AdaptiveThresholdDetector {
	return &AdaptiveThresholdDetector{cfg: cfg}
}

// Detect marks points that exceed a rolling high-percentile threshold (upper-tail drift).
func (d *AdaptiveThresholdDetector) Detect(points []prediction.DataPoint) []prediction.AnomalyResult {
	w := d.cfg.AdaptiveWindowSize
	if w <= 0 {
		w = DefaultDetectorConfig().AdaptiveWindowSize
	}
	pct := d.cfg.AdaptivePercentile
	if pct <= 0 || pct >= 1 {
		pct = DefaultDetectorConfig().AdaptivePercentile
	}
	if len(points) <= w {
		return nil
	}
	out := make([]prediction.AnomalyResult, 0)
	for i := w; i < len(points); i++ {
		window := points[i-w : i]
		vals := valuesFromPoints(window)
		th := quantileSorted(cloneSortedFloats(vals), pct)
		stdW := stdDevPopulation(vals)
		v := points[i].Value
		if v > th {
			denom := stdW
			if denom <= 0 {
				denom = math.Abs(th)
				if denom <= 0 {
					denom = 1e-9
				}
			}
			score := (v - th) / denom
			sev := "warning"
			if score > 2 {
				sev = "critical"
			}
			out = append(out, prediction.AnomalyResult{
				Timestamp: points[i].Timestamp,
				Score:     score,
				Type:      "drift",
				Severity:  sev,
			})
		}
	}
	return out
}
