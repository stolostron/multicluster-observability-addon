package optimizer

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func lowCPUForecast() []prediction.ForecastResult {
	return []prediction.ForecastResult{
		{PredictedValue: 0.5, ConfidenceLower: 0.1, ConfidenceUpper: 1.0},
		{PredictedValue: 0.4, ConfidenceLower: 0.1, ConfidenceUpper: 0.9},
	}
}

func lowMemForecast() []prediction.ForecastResult {
	return []prediction.ForecastResult{
		{PredictedValue: 0.5, ConfidenceLower: 0.1, ConfidenceUpper: 1.0},
		{PredictedValue: 0.4, ConfidenceLower: 0.1, ConfidenceUpper: 0.9},
	}
}

func TestRecommend_BoundsEnforcement(t *testing.T) {
	b := DefaultBoundsConfig()
	r := NewRecommender(b)
	cpuF := lowCPUForecast()
	memF := lowMemForecast()
	res := r.Recommend(10, 10, cpuF, memF, false)
	if res.TargetCPU < b.MinCPUMillicores-1e-6 {
		t.Fatalf("target CPU %v below min %v", res.TargetCPU, b.MinCPUMillicores)
	}
	if res.TargetMemory < b.MinMemoryMiB-1e-6 {
		t.Fatalf("target memory %v below min %v", res.TargetMemory, b.MinMemoryMiB)
	}
}

func TestRecommend_DownscaleCapped(t *testing.T) {
	b := DefaultBoundsConfig()
	r := NewRecommender(b)
	cpuF := []prediction.ForecastResult{
		{PredictedValue: 100, ConfidenceLower: 80, ConfidenceUpper: 110},
		{PredictedValue: 100, ConfidenceLower: 80, ConfidenceUpper: 110},
	}
	memF := []prediction.ForecastResult{
		{PredictedValue: 100, ConfidenceLower: 80, ConfidenceUpper: 110},
	}
	floor := 1000 * (1.0 - b.MaxDownscalePercent/100.0)
	res := r.Recommend(1000, 1000, cpuF, memF, false)
	if res.TargetCPU < floor-1e-6 {
		t.Fatalf("target CPU %v below max-downscale floor %v", res.TargetCPU, floor)
	}
}

func TestRecommend_OOMAddsMemory(t *testing.T) {
	b := DefaultBoundsConfig()
	r := NewRecommender(b)
	cpuF := []prediction.ForecastResult{
		{PredictedValue: 400, ConfidenceLower: 350, ConfidenceUpper: 450},
	}
	memF := []prediction.ForecastResult{
		{PredictedValue: 500, ConfidenceLower: 400, ConfidenceUpper: 600},
	}
	noOOM := r.Recommend(800, 800, cpuF, memF, false)
	withOOM := r.Recommend(800, 800, cpuF, memF, true)
	if withOOM.TargetMemory <= noOOM.TargetMemory+1e-6 {
		t.Fatalf("OOM path should increase memory target: noOOM=%v withOOM=%v",
			noOOM.TargetMemory, withOOM.TargetMemory)
	}
}
