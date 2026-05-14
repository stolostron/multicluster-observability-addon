package prediction

import (
	"encoding/json"
	"time"
)

// ProviderConfig carries the provider discriminator plus type-specific JSON configuration.
type ProviderConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config,omitempty"`
}

// DataPoint is a single observation in a time series.
type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// ForecastRequest describes inputs for a multi-step forecast.
type ForecastRequest struct {
	Points   []DataPoint
	Horizon  int
	Interval time.Duration
}

// ForecastResult is one step of a forecast with optional uncertainty and metadata.
type ForecastResult struct {
	PredictedValue    float64
	ConfidenceLower   float64
	ConfidenceUpper   float64
	Timestamp         time.Time
	DominantModel     string
	FeatureImportance map[string]float64
}

// AnomalyResult describes a detected anomaly in the input series.
type AnomalyResult struct {
	Timestamp time.Time
	Score     float64
	Type      string
	Severity  string
}

// ModelConfig holds parameters used by statistical and decomposition models.
type ModelConfig struct {
	Alpha          float64 // HW level smoothing
	Beta           float64 // HW trend smoothing
	Gamma          float64 // HW seasonal smoothing
	MaxOrder       int     // AR order placeholder
	SeasonalPeriod int     // Season length for HW / STL
	Iterations     int     // STL iterations placeholder
}

// DefaultModelConfig returns default smoothing and season settings.
func DefaultModelConfig() ModelConfig {
	return ModelConfig{
		Alpha:          0.2,
		Beta:           0.1,
		Gamma:          0.05,
		MaxOrder:       10,
		SeasonalPeriod: 288,
		Iterations:     3,
	}
}
