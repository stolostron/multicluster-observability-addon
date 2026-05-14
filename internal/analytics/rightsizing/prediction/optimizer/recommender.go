package optimizer

import (
	"fmt"
	"math"
	"sort"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// Recommender turns forecasts into bounded CPU/memory targets.
type Recommender struct {
	bounds BoundsConfig
}

// NewRecommender returns a recommender using the given bounds.
func NewRecommender(bounds BoundsConfig) *Recommender {
	return &Recommender{bounds: bounds}
}

// Recommend derives target resources from current allocation and forecast series.
func (r *Recommender) Recommend(
	currentCPU, currentMemory float64,
	cpuForecast, memForecast []prediction.ForecastResult,
	oomHistory bool,
) OptimizationResult {
	b := r.bounds
	margin := 1.0 + b.SafetyMarginPercent/100.0

	cpuRaw := rawTargetFromForecast(currentCPU, cpuForecast, margin)
	memRaw := rawTargetFromForecast(currentMemory, memForecast, margin)

	if oomHistory {
		memRaw *= 1.25
	}

	targetCPU := applyRateLimitsAndBounds(cpuRaw, currentCPU, b.MinCPUMillicores, b.MaxCPUMillicores, b.MaxDownscalePercent, b.MaxUpscalePercent)
	targetMem := applyRateLimitsAndBounds(memRaw, currentMemory, b.MinMemoryMiB, b.MaxMemoryMiB, b.MaxDownscalePercent, b.MaxUpscalePercent)

	savingsPct := 0.0
	if currentCPU > 0 {
		savingsPct = (currentCPU - targetCPU) / currentCPU * 100.0
	}

	conf := avgForecastConfidence(cpuForecast, memForecast)

	explanation := fmt.Sprintf(
		"targets cpu=%.0fm mem=%.0fMiB from forecasts (margin %.1f%%; OOM headroom %v); savings vs current CPU ~%.1f%%",
		targetCPU, targetMem, b.SafetyMarginPercent, oomHistory, savingsPct,
	)

	return OptimizationResult{
		TargetCPU:               targetCPU,
		TargetMemory:            targetMem,
		EstimatedSavingsPercent: savingsPct,
		Confidence:              conf,
		Explanation:             explanation,
	}
}

// RecommendWithGPU extends Recommend with GPU utilization and GPU memory targets from forecasts.
func (r *Recommender) RecommendWithGPU(
	currentCPU, currentMemory float64,
	cpuForecast, memForecast []prediction.ForecastResult,
	oomHistory bool,
	currentGPU, currentGPUMemory float64,
	gpuForecast, gpuMemForecast []prediction.ForecastResult,
) OptimizationResult {
	result := r.Recommend(currentCPU, currentMemory, cpuForecast, memForecast, oomHistory)
	b := r.bounds
	margin := 1.0 + b.SafetyMarginPercent/100.0

	gpuRaw := rawTargetFromForecast(currentGPU, gpuForecast, margin)
	gpuMemRaw := rawTargetFromForecast(currentGPUMemory, gpuMemForecast, margin)

	result.TargetGPU = applyRateLimitsAndBounds(gpuRaw, currentGPU, b.MinGPU, b.MaxGPU, b.MaxDownscalePercent, b.MaxUpscalePercent)
	result.TargetGPUMemory = applyRateLimitsAndBounds(gpuMemRaw, currentGPUMemory, b.MinGPUMemoryMiB, b.MaxGPUMemoryMiB, b.MaxDownscalePercent, b.MaxUpscalePercent)
	result.Confidence = avgForecastConfidence(cpuForecast, memForecast, gpuForecast, gpuMemForecast)
	result.Explanation = fmt.Sprintf(
		"%s; gpu=%.4f (util %% bounds [%.0f,%.0f]) gpu_mem=%.0fMiB",
		result.Explanation,
		result.TargetGPU,
		b.MinGPU, b.MaxGPU,
		result.TargetGPUMemory,
	)

	return result
}

func rawTargetFromForecast(current float64, forecast []prediction.ForecastResult, margin float64) float64 {
	maxPred, maxUpper, p99 := forecastStats(forecast, current)
	return math.Max(math.Max(maxPred*margin, maxUpper), p99)
}

func forecastStats(forecast []prediction.ForecastResult, current float64) (maxPred, maxUpper, p99 float64) {
	if len(forecast) == 0 {
		return current, current, current
	}
	maxPred = forecast[0].PredictedValue
	maxUpper = forecast[0].ConfidenceUpper
	preds := make([]float64, 0, len(forecast))
	for i := range forecast {
		fr := forecast[i]
		if fr.PredictedValue > maxPred {
			maxPred = fr.PredictedValue
		}
		if fr.ConfidenceUpper > maxUpper {
			maxUpper = fr.ConfidenceUpper
		}
		preds = append(preds, fr.PredictedValue)
	}
	p99 = percentile99(preds)
	return maxPred, maxUpper, p99
}

func percentile99(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	idx := max(int(math.Round(0.99*float64(len(sorted)-1))), 0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func applyRateLimitsAndBounds(target, current, minBound, maxBound, maxDownPct, maxUpPct float64) float64 {
	minAllowed := current * (1.0 - maxDownPct/100.0)
	maxAllowed := current * (1.0 + maxUpPct/100.0)
	out := target
	if out < minAllowed {
		out = minAllowed
	}
	if out > maxAllowed {
		out = maxAllowed
	}
	if out < minBound {
		out = minBound
	}
	if out > maxBound {
		out = maxBound
	}
	return out
}

func avgForecastConfidence(forecastGroups ...[]prediction.ForecastResult) float64 {
	type widthAccum struct {
		sum   float64
		count float64
	}
	var w widthAccum
	addWidths := func(forecast []prediction.ForecastResult) {
		for i := range forecast {
			fr := forecast[i]
			width := fr.ConfidenceUpper - fr.ConfidenceLower
			if width < 0 {
				width = -width
			}
			w.sum += width
			w.count++
		}
	}
	for _, fg := range forecastGroups {
		addWidths(fg)
	}
	if w.count == 0 {
		return 0.5
	}
	avgWidth := w.sum / w.count

	var meanPred float64
	var n float64
	for _, fg := range forecastGroups {
		for _, fr := range fg {
			meanPred += math.Abs(fr.PredictedValue)
			n++
		}
	}
	if n == 0 {
		return 0.5
	}
	meanPred /= n
	scale := meanPred
	if scale < 1e-9 {
		scale = 1e-9
	}
	normalizedWidth := avgWidth / scale
	// Narrower intervals (small normalized width) → confidence closer to 1.
	return 1.0 / (1.0 + normalizedWidth)
}
