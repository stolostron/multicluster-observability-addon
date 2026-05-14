package prediction

import (
	"fmt"
	"math"
	"time"
)

// ARModel is a univariate autoregressive model with automatic order selection.
type ARModel struct {
	config       ModelConfig
	coefficients []float64
	order        int
	mean         float64
	variance     float64
	tail         []float64 // last `order` original values before forecast (most recent last)
	interval     time.Duration
	lastTS       time.Time
}

// NewARModel creates an empty AR model using cfg.MaxOrder as the search upper bound.
func NewARModel(cfg ModelConfig) *ARModel {
	return &ARModel{config: cfg}
}

func (a *ARModel) maxOrder(n int) int {
	pmax := a.config.MaxOrder
	if pmax <= 0 {
		pmax = DefaultModelConfig().MaxOrder
	}
	if pmax >= n-2 {
		pmax = n - 3
	}
	if pmax < 1 {
		pmax = 1
	}
	return pmax
}

// Fit estimates the mean, selects AR order by BIC via Yule–Walker (Levinson–Durbin), and stores coefficients.
func (a *ARModel) Fit(points []DataPoint) error {
	if len(points) < 4 {
		return fmt.Errorf("ar: need at least 4 points")
	}
	n := len(points)
	values := make([]float64, n)
	for i, p := range points {
		values[i] = p.Value
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	z := make([]float64, n)
	for i, v := range values {
		z[i] = v - mean
	}

	a.mean = mean
	a.lastTS = points[n-1].Timestamp
	if n >= 2 {
		a.interval = points[n-1].Timestamp.Sub(points[n-2].Timestamp)
	}

	acfMax := a.maxOrder(n)
	if acfMax < 1 {
		return fmt.Errorf("ar: series too short for AR(1)")
	}
	acf := sampleAutocorr(z, acfMax)
	if acf[0] < 1e-18 {
		a.order = 0
		a.coefficients = nil
		a.variance = 1e-12
		a.tail = []float64{values[n-1]}
		return nil
	}

	bestP := 1
	bestBIC := math.Inf(1)
	var bestPhi []float64
	var bestVar float64

	for p := 1; p <= acfMax; p++ {
		r := acf[:p+1]
		if r[0] < 1e-18 {
			continue
		}
		phi, ok := yuleWalkerCoeffs(r, p)
		if !ok || len(phi) != p {
			continue
		}
		sigma2 := arInnovationVar(values, phi, mean)
		if sigma2 <= 0 || math.IsNaN(sigma2) {
			continue
		}
		nEff := float64(n)
		bic := nEff*math.Log(sigma2) + float64(p)*math.Log(nEff)
		if bic < bestBIC {
			bestBIC = bic
			bestP = p
			bestPhi = append([]float64(nil), phi...)
			bestVar = sigma2
		}
	}

	a.order = bestP
	a.coefficients = bestPhi
	a.variance = bestVar
	if bestP > 0 {
		a.tail = append([]float64(nil), values[n-bestP:]...)
	} else {
		a.tail = []float64{values[n-1]}
	}
	return nil
}

// Forecast generates horizon steps using the last `order` observed values (or prior forecasts) as state.
func (a *ARModel) Forecast(horizon int) []ForecastResult {
	if horizon <= 0 {
		return nil
	}
	if len(a.tail) == 0 {
		out := make([]ForecastResult, horizon)
		return out
	}
	z := 1.645
	margin := z * math.Sqrt(math.Max(a.variance, 0))

	state := append([]float64(nil), a.tail...)
	if a.order == 0 || len(a.coefficients) == 0 {
		v := a.mean
		out := make([]ForecastResult, horizon)
		for h := 1; h <= horizon; h++ {
			ts := a.lastTS.Add(a.interval * time.Duration(h))
			out[h-1] = ForecastResult{
				PredictedValue:  v,
				ConfidenceLower: v - margin,
				ConfidenceUpper: v + margin,
				Timestamp:       ts,
				DominantModel:   "ar",
			}
		}
		return out
	}

	out := make([]ForecastResult, horizon)
	p := a.order
	for h := 1; h <= horizon; h++ {
		var pred float64 = a.mean
		for i := 0; i < p; i++ {
			// state[len-1] is most recent lag-1
			xLag := state[len(state)-1-i]
			pred += a.coefficients[i] * (xLag - a.mean)
		}
		state = append(state, pred)
		ts := a.lastTS.Add(a.interval * time.Duration(h))
		out[h-1] = ForecastResult{
			PredictedValue:  pred,
			ConfidenceLower: pred - margin,
			ConfidenceUpper: pred + margin,
			Timestamp:       ts,
			DominantModel:   "ar",
		}
	}
	return out
}

func sampleAutocorr(z []float64, maxLag int) []float64 {
	n := len(z)
	out := make([]float64, maxLag+1)
	var denom float64
	for _, v := range z {
		denom += v * v
	}
	if denom < 1e-18 {
		return out
	}
	out[0] = 1
	for lag := 1; lag <= maxLag; lag++ {
		var num float64
		for t := 0; t < n-lag; t++ {
			num += z[t] * z[t+lag]
		}
		out[lag] = num / denom
	}
	return out
}

// yuleWalkerCoeffs solves the Yule–Walker system using the ACF slice r[0..p] with r[0]=1.
func yuleWalkerCoeffs(r []float64, p int) ([]float64, bool) {
	if p < 1 || len(r) < p+1 {
		return nil, false
	}
	mat := make([][]float64, p)
	for i := 0; i < p; i++ {
		row := make([]float64, p+1)
		for j := 0; j < p; j++ {
			row[j] = r[absInt(i-j)]
		}
		row[p] = r[i+1]
		mat[i] = row
	}
	if !gaussianPartialPivot(mat) {
		return nil, false
	}
	phi := make([]float64, p)
	for i := 0; i < p; i++ {
		phi[i] = mat[i][p]
	}
	return phi, true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func gaussianPartialPivot(a [][]float64) bool {
	n := len(a)
	if n == 0 {
		return false
	}
	cols := len(a[0])
	for i := 0; i < n; i++ {
		piv := i
		for k := i + 1; k < n; k++ {
			if math.Abs(a[k][i]) > math.Abs(a[piv][i]) {
				piv = k
			}
		}
		if math.Abs(a[piv][i]) < 1e-18 {
			return false
		}
		a[i], a[piv] = a[piv], a[i]
		div := a[i][i]
		for j := i; j < cols; j++ {
			a[i][j] /= div
		}
		for k := 0; k < n; k++ {
			if k == i {
				continue
			}
			f := a[k][i]
			for j := i; j < cols; j++ {
				a[k][j] -= f * a[i][j]
			}
		}
	}
	return true
}

func arInnovationVar(values []float64, phi []float64, mean float64) float64 {
	n := len(values)
	p := len(phi)
	if n <= p {
		return 0
	}
	var sse float64
	for t := p; t < n; t++ {
		var pred float64 = mean
		for i := 0; i < p; i++ {
			pred += phi[i] * (values[t-1-i] - mean)
		}
		e := values[t] - pred
		sse += e * e
	}
	df := float64(n - p)
	if df <= 0 {
		return 0
	}
	return sse / df
}
