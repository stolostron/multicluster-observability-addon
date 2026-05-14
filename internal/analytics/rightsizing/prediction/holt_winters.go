package prediction

import (
	"math"
	"time"
)

// HoltWintersModel performs additive triple exponential smoothing with
// automatic fallback to simple exponential smoothing when the series is
// shorter than two seasonal periods.
type HoltWintersModel struct {
	config   ModelConfig
	level    float64
	trend    float64
	seasonal []float64
	fitted   bool
	points   []DataPoint
}

// NewHoltWintersModel constructs a Holt-Winters forecaster.
func NewHoltWintersModel(cfg ModelConfig) *HoltWintersModel {
	return &HoltWintersModel{config: cfg}
}

// Fit ingests historical data, initializes components, and runs the
// smoothing pass.  Returns nil on success.
func (h *HoltWintersModel) Fit(points []DataPoint) error {
	h.fitted = false
	h.points = append([]DataPoint(nil), points...)
	if len(points) == 0 {
		return nil
	}

	m := h.config.SeasonalPeriod
	if m <= 1 {
		m = DefaultModelConfig().SeasonalPeriod
	}

	if len(points) < 2*m {
		h.fitSimpleExponential(points)
		return nil
	}

	h.fitFullHW(points, m)
	return nil
}

func (h *HoltWintersModel) fitSimpleExponential(points []DataPoint) {
	alpha := h.config.Alpha
	n := len(points)
	h.level = points[0].Value
	h.trend = 0
	h.seasonal = nil
	for i := 1; i < n; i++ {
		h.level = alpha*points[i].Value + (1-alpha)*h.level
	}
	h.fitted = true
}

func (h *HoltWintersModel) fitFullHW(points []DataPoint, m int) {
	alpha := h.config.Alpha
	beta := h.config.Beta
	gamma := h.config.Gamma
	n := len(points)

	vals := make([]float64, n)
	for i, p := range points {
		vals[i] = p.Value
	}

	// Initial level: mean of first season
	var sum float64
	for i := 0; i < m; i++ {
		sum += vals[i]
	}
	level := sum / float64(m)

	// Initial trend: average slope across the first two seasons
	var trendSum float64
	for i := 0; i < m; i++ {
		trendSum += (vals[m+i] - vals[i]) / float64(m)
	}
	trend := trendSum / float64(m)

	// Initial seasonal: deviation from level in first season
	seasonal := make([]float64, m)
	for i := 0; i < m; i++ {
		seasonal[i] = vals[i] - level
	}

	// Smoothing pass
	for i := m; i < n; i++ {
		y := vals[i]
		sIdx := i % m
		prevLevel := level
		level = alpha*(y-seasonal[sIdx]) + (1-alpha)*(prevLevel+trend)
		trend = beta*(level-prevLevel) + (1-beta)*trend
		seasonal[sIdx] = gamma*(y-level) + (1-gamma)*seasonal[sIdx]
	}

	h.level = level
	h.trend = trend
	h.seasonal = seasonal
	h.fitted = true
}

// Forecast projects horizon steps ahead. Call Fit first.
func (h *HoltWintersModel) Forecast(horizon int) []ForecastResult {
	if horizon <= 0 || !h.fitted || len(h.points) == 0 {
		return nil
	}

	n := len(h.points)
	lastTS := h.points[n-1].Timestamp
	var step time.Duration
	if n >= 2 {
		step = h.points[n-1].Timestamp.Sub(h.points[n-2].Timestamp)
	}

	resStd := h.residualStdDev()
	z := 1.96

	out := make([]ForecastResult, horizon)
	for i := 1; i <= horizon; i++ {
		pred := h.level + float64(i)*h.trend
		if len(h.seasonal) > 0 {
			m := len(h.seasonal)
			sIdx := (n - 1 + i) % m
			if sIdx < 0 {
				sIdx += m
			}
			pred += h.seasonal[sIdx]
		}
		margin := z * resStd * math.Sqrt(float64(i))
		ts := lastTS.Add(step * time.Duration(i))
		out[i-1] = ForecastResult{
			PredictedValue:  pred,
			ConfidenceLower: pred - margin,
			ConfidenceUpper: pred + margin,
			Timestamp:       ts,
			DominantModel:   "holt_winters",
		}
	}
	return out
}

func (h *HoltWintersModel) residualStdDev() float64 {
	if len(h.points) == 0 || !h.fitted {
		return 0
	}

	m := h.config.SeasonalPeriod
	if m <= 1 {
		m = DefaultModelConfig().SeasonalPeriod
	}
	useSeasonal := len(h.seasonal) > 0 && len(h.points) >= 2*m

	alpha := h.config.Alpha
	beta := h.config.Beta
	gamma := h.config.Gamma
	n := len(h.points)

	vals := make([]float64, n)
	for i, p := range h.points {
		vals[i] = p.Value
	}

	if !useSeasonal {
		lv := vals[0]
		var sse float64
		var count int
		for i := 1; i < n; i++ {
			predicted := lv
			lv = alpha*vals[i] + (1-alpha)*lv
			e := vals[i] - predicted
			sse += e * e
			count++
		}
		if count <= 1 {
			return 0
		}
		return math.Sqrt(sse / float64(count))
	}

	var sum float64
	for i := 0; i < m; i++ {
		sum += vals[i]
	}
	level := sum / float64(m)
	var trendSum float64
	for i := 0; i < m; i++ {
		trendSum += (vals[m+i] - vals[i]) / float64(m)
	}
	trend := trendSum / float64(m)
	seasonal := make([]float64, m)
	for i := 0; i < m; i++ {
		seasonal[i] = vals[i] - level
	}

	var sse float64
	var count int
	for i := m; i < n; i++ {
		sIdx := i % m
		predicted := level + trend + seasonal[sIdx]
		e := vals[i] - predicted
		sse += e * e
		count++
		prevLevel := level
		level = alpha*(vals[i]-seasonal[sIdx]) + (1-alpha)*(prevLevel+trend)
		trend = beta*(level-prevLevel) + (1-beta)*trend
		seasonal[sIdx] = gamma*(vals[i]-level) + (1-gamma)*seasonal[sIdx]
	}
	if count <= 1 {
		return 0
	}
	return math.Sqrt(sse / float64(count))
}
