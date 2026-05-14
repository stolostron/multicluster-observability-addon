package prediction

import (
	"math"
	"time"
)

// EnsembleForecaster combines average-weighted Holt–Winters, STL, and AR forecasts.
type EnsembleForecaster struct {
	config ModelConfig

	weights map[string]float64

	hw  *HoltWintersModel
	stl *STLModel
	ar  *ARModel

	prevMinMAPE float64
}

// NewEnsembleForecaster wires three sub-models with equal initial weights.
func NewEnsembleForecaster(cfg ModelConfig) *EnsembleForecaster {
	w := map[string]float64{
		"holt_winters": 1.0 / 3.0,
		"stl":          1.0 / 3.0,
		"ar":           1.0 / 3.0,
	}
	return &EnsembleForecaster{
		config:      cfg,
		weights:     w,
		hw:          NewHoltWintersModel(cfg),
		stl:         NewSTLModel(cfg),
		ar:          NewARModel(cfg),
		prevMinMAPE: math.Inf(1),
	}
}

// Train fits each component, runs a back-validation, and optionally reweights by inverse MAPE.
func (e *EnsembleForecaster) Train(points []DataPoint) error {
	if len(points) == 0 {
		return nil
	}
	_ = e.hw.Fit(points)
	_ = e.stl.Decompose(points)
	_ = e.ar.Fit(points)

	results := Backtest(points, e.config)
	if len(results) == 0 {
		return nil
	}

	newMin := math.Inf(1)
	for _, r := range results {
		if r.MAPE < newMin {
			newMin = r.MAPE
		}
	}

	shouldRefresh := math.IsInf(e.prevMinMAPE, 1) || (e.prevMinMAPE > 0 && newMin < e.prevMinMAPE*0.95)
	if shouldRefresh {
		inv := make(map[string]float64)
		var sum float64
		for _, r := range results {
			den := r.MAPE
			if den < 1e-9 {
				den = 1e-9
			}
			v := 1.0 / den
			inv[r.ModelName] = v
			sum += v
		}
		if sum <= 0 {
			return nil
		}
		for k, v := range inv {
			e.weights[k] = v / sum
		}
		e.prevMinMAPE = newMin
	}
	return nil
}

// Forecast blends per-model trajectories using weighted marginal variances for uncertainty.
func (e *EnsembleForecaster) Forecast(req ForecastRequest) []ForecastResult {
	h := req.Horizon
	if h <= 0 || len(req.Points) == 0 {
		return nil
	}
	_ = e.hw.Fit(req.Points)
	hwOut := e.hw.Forecast(h)
	stlF := e.stl.Forecast(req.Points, h)
	_ = e.ar.Fit(req.Points)
	arF := e.ar.Forecast(h)

	interval := req.Interval
	if interval <= 0 && len(req.Points) >= 2 {
		interval = req.Points[len(req.Points)-1].Timestamp.Sub(req.Points[len(req.Points)-2].Timestamp)
	}
	lastTS := req.Points[len(req.Points)-1].Timestamp

	dom := dominantWeight(e.weights)

	out := make([]ForecastResult, h)
	for t := 0; t < h; t++ {
		var mean, varMix float64
		if t < len(hwOut) {
			w := e.weights["holt_winters"]
			mean += w * hwOut[t].PredictedValue
			varMix += w * sigmaFromInterval(hwOut[t])
		}
		if t < len(stlF) {
			w := e.weights["stl"]
			mean += w * stlF[t].PredictedValue
			varMix += w * sigmaFromInterval(stlF[t])
		}
		if t < len(arF) {
			w := e.weights["ar"]
			mean += w * arF[t].PredictedValue
			varMix += w * sigmaFromInterval(arF[t])
		}
		sd := math.Sqrt(math.Max(varMix, 0))
		margin := 1.645 * sd
		ts := lastTS.Add(interval * time.Duration(t+1))
		out[t] = ForecastResult{
			PredictedValue:    mean,
			ConfidenceLower:   mean - margin,
			ConfidenceUpper:   mean + margin,
			Timestamp:         ts,
			DominantModel:     dom,
			FeatureImportance: cloneWeights(e.weights),
		}
	}
	return out
}

// Weights returns a defensive copy of ensemble member weights.
func (e *EnsembleForecaster) Weights() map[string]float64 {
	return cloneWeights(e.weights)
}

func sigmaFromInterval(fr ForecastResult) float64 {
	half := (fr.ConfidenceUpper - fr.ConfidenceLower) / 2
	if half <= 0 {
		return 0
	}
	return half / 1.645
}

func cloneWeights(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func dominantWeight(w map[string]float64) string {
	var bestK string
	var bestV float64 = math.Inf(-1)
	for k, v := range w {
		if v > bestV {
			bestV = v
			bestK = k
		}
	}
	return bestK
}
