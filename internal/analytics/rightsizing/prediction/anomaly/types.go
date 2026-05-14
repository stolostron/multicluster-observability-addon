package anomaly

// DetectorConfig holds parameters shared by anomaly detectors.
type DetectorConfig struct {
	ZScoreThreshold        float64 // default 3.0
	RateOfChangeMultiplier float64 // default 2.5
	AdaptiveWindowSize     int     // default 24
	AdaptivePercentile     float64 // default 0.95
}

// DefaultDetectorConfig returns default detector settings.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		ZScoreThreshold:        3.0,
		RateOfChangeMultiplier: 2.5,
		AdaptiveWindowSize:     24,
		AdaptivePercentile:     0.95,
	}
}
