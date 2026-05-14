package anomaly

import (
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// ZScoreDetector flags points whose value deviates from the series mean by a Z-score threshold.
type ZScoreDetector struct {
	cfg DetectorConfig
}

// NewZScoreDetector constructs a ZScoreDetector with the given config.
func NewZScoreDetector(cfg DetectorConfig) *ZScoreDetector {
	return &ZScoreDetector{cfg: cfg}
}

// Detect returns spike anomalies where |Z| exceeds the configured threshold.
func (d *ZScoreDetector) Detect(points []prediction.DataPoint) []prediction.AnomalyResult {
	if len(points) == 0 {
		return nil
	}
	mean := meanValues(points)
	std := stdDevPopulation(valuesFromPoints(points))
	if std <= 0 {
		return nil
	}
	th := d.cfg.ZScoreThreshold
	if th <= 0 {
		th = DefaultDetectorConfig().ZScoreThreshold
	}
	out := make([]prediction.AnomalyResult, 0)
	for _, p := range points {
		z := math.Abs(p.Value-mean) / std
		if z > th {
			sev := "warning"
			if z > 2*th {
				sev = "critical"
			}
			out = append(out, prediction.AnomalyResult{
				Timestamp: p.Timestamp,
				Score:     z,
				Type:      "spike",
				Severity:  sev,
			})
		}
	}
	return out
}
