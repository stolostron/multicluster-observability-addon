package features

import (
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func extractWorkloadFeatures(points []prediction.DataPoint, cfg FeatureConfig, out *FeatureVector) {
	n := len(points)
	if n == 0 {
		return
	}
	w := cfg.WindowSize
	if w <= 0 || w > n {
		w = n
	}
	start := n - w
	window := make([]float64, w)
	for i := 0; i < w; i++ {
		window[i] = points[start+i].Value
	}
	mean := 0.0
	for _, v := range window {
		mean += v
	}
	mean /= float64(w)

	var sumSq float64
	for _, v := range window {
		d := v - mean
		sumSq += d * d
	}
	std := 0.0
	if w > 1 {
		std = math.Sqrt(sumSq / float64(w-1))
	}

	threshold := mean + 2*std
	var burstCount int
	var burstSum float64
	for _, v := range window {
		if v > threshold {
			burstCount++
			burstSum += v - threshold
		}
	}
	out.BurstFrequency = float64(burstCount) / float64(w)
	if burstCount > 0 {
		out.BurstMagnitude = burstSum / float64(burstCount)
	}

	maxV := window[0]
	for _, v := range window[1:] {
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 0 {
		out.IdleRatio = 0
		out.UtilizationEfficiency = 0
		return
	}
	idleCut := 0.1 * maxV
	var idleN int
	for _, v := range window {
		if v < idleCut {
			idleN++
		}
	}
	out.IdleRatio = float64(idleN) / float64(w)
	out.UtilizationEfficiency = mean / maxV
}
