package optimizer

// OptimizationResult summarizes a rightsizing recommendation from forecasts.
type OptimizationResult struct {
	TargetCPU               float64
	TargetMemory            float64
	TargetGPU               float64
	TargetGPUMemory         float64
	EstimatedSavingsPercent float64
	Confidence              float64
	Explanation             string
}

// BoundsConfig constrains recommendation targets and rate-of-change limits.
type BoundsConfig struct {
	MinCPUMillicores    float64
	MaxCPUMillicores    float64
	MinMemoryMiB        float64
	MaxMemoryMiB        float64
	MinGPU              float64 // default 0 (GPUs are whole units but forecasts are fractional utilization %)
	MaxGPU              float64 // default 100 (utilization %)
	MinGPUMemoryMiB     float64 // default 0
	MaxGPUMemoryMiB     float64 // default 81920 (80 GiB, e.g. A100)
	MaxDownscalePercent float64
	MaxUpscalePercent   float64
	SafetyMarginPercent float64
}

// DefaultBoundsConfig returns conservative defaults for CPU/memory limits and deltas.
func DefaultBoundsConfig() BoundsConfig {
	return BoundsConfig{
		MinCPUMillicores:    50,
		MaxCPUMillicores:    8000,
		MinMemoryMiB:        64,
		MaxMemoryMiB:        16384,
		MinGPU:              0,
		MaxGPU:              100,
		MinGPUMemoryMiB:     0,
		MaxGPUMemoryMiB:     81920,
		MaxDownscalePercent: 30,
		MaxUpscalePercent:   100,
		SafetyMarginPercent: 15,
	}
}
