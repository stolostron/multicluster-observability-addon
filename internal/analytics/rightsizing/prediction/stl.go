package prediction

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// STLModel performs seasonal-trend decomposition via LOESS-based STL iterations.
type STLModel struct {
	config     ModelConfig
	trend      []float64
	seasonal   []float64
	residual   []float64
	values     []float64
	lastAnchor time.Time
	interval   time.Duration
}

// NewSTLModel constructs a decomposition model with the given configuration.
func NewSTLModel(cfg ModelConfig) *STLModel {
	return &STLModel{config: cfg}
}

func (s *STLModel) seasonalPeriod(n int) int {
	m := s.config.SeasonalPeriod
	if m <= 0 {
		m = DefaultModelConfig().SeasonalPeriod
	}
	if m > n/2 {
		m = n / 2
	}
	if m < 2 {
		m = 2
	}
	return m
}

func (s *STLModel) iterations() int {
	it := s.config.Iterations
	if it <= 0 {
		it = DefaultModelConfig().Iterations
	}
	return it
}

// Decompose fits trend, seasonal, and residual for the given series.
func (s *STLModel) Decompose(points []DataPoint) error {
	if len(points) < 8 {
		return fmt.Errorf("stl: need at least 8 points, got %d", len(points))
	}
	n := len(points)
	y := make([]float64, n)
	for i, p := range points {
		y[i] = p.Value
	}
	s.values = y
	s.lastAnchor = points[n-1].Timestamp
	s.interval = inferInterval(points)

	m := s.seasonalPeriod(n)
	if n < 2*m {
		return fmt.Errorf("stl: series shorter than two seasonal periods")
	}

	iters := s.iterations()
	x := indices(n)

	// Initial seasonal: y - centered MA (window m)
	ma0 := centeredMovingAverage(y, m)
	seasonal := make([]float64, n)
	for i := range y {
		seasonal[i] = y[i] - ma0[i]
	}

	var trend []float64
	bw := loessBandwidthFrac(len(points))

	for iter := 0; iter < iters; iter++ {
		deseason := make([]float64, n)
		for i := range y {
			deseason[i] = y[i] - seasonal[i]
		}
		trend = make([]float64, n)
		for i := 0; i < n; i++ {
			trend[i] = loess(x, deseason, x[i], bw)
		}
		detrended := make([]float64, n)
		for i := range y {
			detrended[i] = y[i] - trend[i]
		}
		// Average seasonal pattern across cycles; re-expand to length n
		seasonal = expandSeasonalCycle(detrended, m)
	}

	resid := make([]float64, n)
	for i := range y {
		resid[i] = y[i] - trend[i] - seasonal[i]
	}

	s.trend = trend
	s.seasonal = seasonal
	s.residual = resid
	return nil
}

// Forecast runs Decompose on points then extrapolates trend and repeats seasonality.
func (s *STLModel) Forecast(points []DataPoint, horizon int) []ForecastResult {
	if horizon <= 0 {
		return nil
	}
	if err := s.Decompose(points); err != nil {
		out := make([]ForecastResult, horizon)
		for h := range out {
			out[h] = ForecastResult{}
		}
		return out
	}
	n := len(s.trend)
	m := s.seasonalPeriod(len(points))
	interval := s.interval
	if interval <= 0 {
		interval = inferInterval(points)
	}

	slope, intercept := linearFitLastPeriod(s.trend, m)

	var resStd float64
	if len(s.residual) > 0 {
		var sum, sumsq float64
		for _, e := range s.residual {
			sum += e
			sumsq += e * e
		}
		mu := sum / float64(len(s.residual))
		variance := sumsq/float64(len(s.residual)) - mu*mu
		if variance > 0 {
			resStd = math.Sqrt(variance)
		}
	}
	z := 1.645 // ~90% two-sided margin per tail used symmetrically in CI

	out := make([]ForecastResult, horizon)
	for h := 1; h <= horizon; h++ {
		tglob := float64(n-1) + float64(h)
		trendHat := intercept + slope*tglob
		phase := (n - 1 + h) % m
		if phase < 0 {
			phase += m
		}
		seasonHat := seasonalAtPhase(s.seasonal, m, phase, n)
		pred := trendHat + seasonHat
		margin := z * resStd
		ts := s.lastAnchor.Add(interval * time.Duration(h))
		out[h-1] = ForecastResult{
			PredictedValue:  pred,
			ConfidenceLower: pred - margin,
			ConfidenceUpper: pred + margin,
			Timestamp:       ts,
			DominantModel:   "stl",
		}
	}
	return out
}

func inferInterval(points []DataPoint) time.Duration {
	if len(points) < 2 {
		return 0
	}
	d := points[len(points)-1].Timestamp.Sub(points[len(points)-2].Timestamp)
	if d <= 0 {
		return 0
	}
	return d
}

func indices(n int) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = float64(i)
	}
	return x
}

func centeredMovingAverage(y []float64, window int) []float64 {
	n := len(y)
	if window < 1 {
		window = 1
	}
	if window%2 == 0 {
		window++
	}
	half := window / 2
	out := make([]float64, n)
	for i := range y {
		var sum float64
		count := 0
		for j := i - half; j <= i+half; j++ {
			if j >= 0 && j < n {
				sum += y[j]
				count++
			}
		}
		if count > 0 {
			out[i] = sum / float64(count)
		} else {
			out[i] = y[i]
		}
	}
	return out
}

// expandSeasonalCycle replaces detrended series by its cyclic seasonal mean (length n).
func expandSeasonalCycle(detrended []float64, m int) []float64 {
	n := len(detrended)
	cycleMean := make([]float64, m)
	counts := make([]int, m)
	for i := 0; i < n; i++ {
		r := i % m
		cycleMean[r] += detrended[i]
		counts[r]++
	}
	for j := 0; j < m; j++ {
		if counts[j] > 0 {
			cycleMean[j] /= float64(counts[j])
		}
	}
	// De-mean seasonal to reduce double-counting with trend
	var ssum float64
	for j := 0; j < m; j++ {
		ssum += cycleMean[j]
	}
	meanS := ssum / float64(m)
	for j := 0; j < m; j++ {
		cycleMean[j] -= meanS
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = cycleMean[i%m]
	}
	return out
}

func seasonalAtPhase(fullSeasonal []float64, period, phase, n int) float64 {
	if len(fullSeasonal) == 0 {
		return 0
	}
	if n > 0 && len(fullSeasonal) >= n && phase >= 0 && phase < period {
		// Use last full cycle slice for stable phase estimate when available
		base := n - period
		if base < 0 {
			base = 0
		}
		idx := base + phase
		if idx < len(fullSeasonal) {
			return fullSeasonal[idx]
		}
	}
	return fullSeasonal[phase%len(fullSeasonal)]
}

func linearFitLastPeriod(trend []float64, m int) (slope, intercept float64) {
	n := len(trend)
	if n == 0 {
		return 0, 0
	}
	if m < 2 {
		m = 2
	}
	start := n - m
	if start < 0 {
		start = 0
	}
	var sumX, sumY, sumXX, sumXY float64
	k := 0
	for i := start; i < n; i++ {
		x := float64(i)
		y := trend[i]
		sumX += x
		sumY += y
		sumXX += x * x
		sumXY += x * y
		k++
	}
	if k < 2 {
		return 0, trend[n-1]
	}
	denom := float64(k)*sumXX - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return 0, sumY / float64(k)
	}
	slope = (float64(k)*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / float64(k)
	return slope, intercept
}

func loessBandwidthFrac(n int) float64 {
	if n < 10 {
		return 0.6
	}
	return 0.3
}

// loess performs local linear regression at xNew with tricube weights.
func loess(x, y []float64, xNew float64, bandwidth float64) float64 {
	n := len(x)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return y[0]
	}
	// distance-based neighbor window
	type idxDist struct {
		i int
		d float64
	}
	arr := make([]idxDist, n)
	for i := 0; i < n; i++ {
		arr[i] = idxDist{i: i, d: math.Abs(x[i] - xNew)}
	}
	sort.Slice(arr, func(a, b int) bool { return arr[a].d < arr[b].d })
	span := int(math.Max(3, math.Ceil(bandwidth*float64(n))))
	if span > n {
		span = n
	}
	maxD := arr[span-1].d
	if maxD < 1e-12 {
		var sum float64
		for k := 0; k < span; k++ {
			sum += y[arr[k].i]
		}
		return sum / float64(span)
	}
	var sw, swx, swy, swxx, swxy float64
	for k := 0; k < span; k++ {
		xi := x[arr[k].i]
		yi := y[arr[k].i]
		u := math.Abs(xi-xNew) / maxD
		if u >= 1 {
			continue
		}
		w := math.Pow(1-math.Pow(u, 3), 3)
		sw += w
		swx += w * xi
		swy += w * yi
		swxx += w * xi * xi
		swxy += w * xi * yi
	}
	den := sw*swxx - swx*swx
	if math.Abs(den) < 1e-15 {
		if sw > 0 {
			return swy / sw
		}
		return 0
	}
	beta := (sw*swxy - swx*swy) / den
	alpha := (swy - beta*swx) / sw
	return alpha + beta*xNew
}
