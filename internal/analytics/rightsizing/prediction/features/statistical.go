package features

import (
	"math"
	"sort"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func extractStatisticalFeatures(points []prediction.DataPoint, cfg FeatureConfig, out *FeatureVector) {
	n := len(points)
	if n == 0 {
		return
	}
	w := cfg.WindowSize
	if w <= 0 {
		w = n
	}
	start := n - w
	if start < 0 {
		start = 0
	}
	window := make([]float64, n-start)
	for i := range window {
		window[i] = points[start+i].Value
	}
	m := len(window)
	if m == 0 {
		return
	}

	mean := 0.0
	for _, v := range window {
		mean += v
	}
	mean /= float64(m)
	out.RollingMean = mean

	var sumSq float64
	for _, v := range window {
		d := v - mean
		sumSq += d * d
	}
	std := 0.0
	if m > 1 {
		std = math.Sqrt(sumSq / float64(m-1))
	}
	out.RollingStdDev = std

	sorted := append([]float64(nil), window...)
	sort.Float64s(sorted)
	out.RollingMedian = medianSorted(sorted)

	p95Idx := int(math.Floor(0.95 * float64(m-1)))
	p99Idx := int(math.Floor(0.99 * float64(m-1)))
	if p95Idx < 0 {
		p95Idx = 0
	}
	if p99Idx < 0 {
		p99Idx = 0
	}
	out.P95 = sorted[p95Idx]
	out.P99 = sorted[p99Idx]

	if m >= 3 && std > 1e-12 {
		var sumCube, sumQuad float64
		for _, v := range window {
			z := (v - mean) / std
			sumCube += z * z * z
			sumQuad += z * z * z * z
		}
		// sample skewness
		out.Skewness = float64(m) / float64((m-1)*(m-2)) * sumCube
		if m >= 4 {
			// sample excess kurtosis
			k := float64(m*(m+1)) / float64((m-1)*(m-2)*(m-3)) * sumQuad
			out.Kurtosis = k - 3.0*float64(m-1)*float64(m-1)/float64((m-2)*(m-3))
		}
	}

	if math.Abs(mean) > 1e-12 {
		out.CoefficientOfVariation = std / mean
	}
}

func medianSorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	mid := n / 2
	if n%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
