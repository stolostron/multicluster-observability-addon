package features

import (
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// FeatureVector holds engineered features for one forecast / decision point.
type FeatureVector struct {
	HourOfDay              float64
	DayOfWeek              float64
	IsBusinessHours        float64
	IsWeekend              float64
	WeekOfMonth            float64
	RollingMean            float64
	RollingStdDev          float64
	RollingMedian          float64
	P95                    float64
	P99                    float64
	Skewness               float64
	Kurtosis               float64
	CoefficientOfVariation float64
	LinearSlope            float64
	Acceleration           float64
	ChangePointScore       float64
	BurstFrequency         float64
	BurstMagnitude         float64
	IdleRatio              float64
	UtilizationEfficiency  float64
	CPUMemoryCorrelation   float64
}

// FeatureConfig controls windowing and calendar assumptions.
type FeatureConfig struct {
	WindowSize         int
	BusinessHoursStart int
	BusinessHoursEnd   int
}

// DefaultFeatureConfig returns defaults aligned to hourly-ish windows and 9–17 business hours.
func DefaultFeatureConfig() FeatureConfig {
	return FeatureConfig{
		WindowSize:         12,
		BusinessHoursStart: 9,
		BusinessHoursEnd:   17,
	}
}

// ExtractFeatures computes the full feature vector from the trailing series.
func ExtractFeatures(points []prediction.DataPoint, cfg FeatureConfig) FeatureVector {
	var v FeatureVector
	extractTemporalFeatures(points, cfg, &v)
	extractStatisticalFeatures(points, cfg, &v)
	extractTrendFeatures(points, &v)
	extractWorkloadFeatures(points, cfg, &v)
	// CPUMemoryCorrelation requires paired CPU/memory series; left at 0 here.
	return v
}
