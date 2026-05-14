package features

import (
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func extractTrendFeatures(points []prediction.DataPoint, out *FeatureVector) {
	n := len(points)
	if n < 2 {
		return
	}
	vals := make([]float64, n)
	for i := range points {
		vals[i] = points[i].Value
	}
	out.LinearSlope = leastSquaresSlope(vals)

	if n >= 4 {
		half := n / 2
		s1 := leastSquaresSlope(vals[:half])
		s2 := leastSquaresSlope(vals[half:])
		out.Acceleration = s2 - s1
	}

	out.ChangePointScore = changePointScoreFromSeries(vals)
}

func leastSquaresSlope(y []float64) float64 {
	n := len(y)
	if n < 2 {
		return 0
	}
	// x = 0,1,...,n-1
	sumX := float64(n*(n-1)) / 2.0
	var sumY float64
	for _, v := range y {
		sumY += v
	}
	var sumXY float64
	for i, v := range y {
		sumXY += float64(i) * v
	}
	sumX2 := float64(n*(n-1)*(2*n-1)) / 6.0
	denom := float64(n)*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return 0
	}
	num := float64(n)*sumXY - sumX*sumY
	return num / denom
}

func rollingMeans(y []float64, win int) []float64 {
	n := len(y)
	if n == 0 || win <= 0 || win > n {
		return nil
	}
	out := make([]float64, 0, n-win+1)
	var sum float64
	for i := 0; i < win; i++ {
		sum += y[i]
	}
	out = append(out, sum/float64(win))
	for i := win; i < n; i++ {
		sum += y[i]
		sum -= y[i-win]
		out = append(out, sum/float64(win))
	}
	return out
}

func changePointScoreFromSeries(y []float64) float64 {
	n := len(y)
	if n < 3 {
		return 0
	}
	w := 12
	if w > n {
		w = n
	}
	if w < 2 {
		w = 2
	}
	rm := rollingMeans(y, w)
	if len(rm) < 2 {
		return 0
	}
	maxDiff := 0.0
	for i := 1; i < len(rm); i++ {
		d := math.Abs(rm[i] - rm[i-1])
		if d > maxDiff {
			maxDiff = d
		}
	}
	return maxDiff
}
