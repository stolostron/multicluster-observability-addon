package prediction

import (
	"math"
	"testing"
	"time"
)

func generateSinusoidal(n int, period int, amplitude, base float64) []DataPoint {
	points := make([]DataPoint, n)
	for i := range n {
		t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Add(5 * time.Minute * time.Duration(i))
		v := base + amplitude*math.Sin(2*math.Pi*float64(i)/float64(period))
		points[i] = DataPoint{Timestamp: t, Value: v}
	}
	return points
}

func forecastMAPE(actual []DataPoint, pred []ForecastResult) float64 {
	if len(actual) == 0 || len(pred) == 0 {
		return math.NaN()
	}
	m := min(len(pred), len(actual))
	eps := 1e-9
	var sum float64
	for i := range m {
		a := actual[i].Value
		p := pred[i].PredictedValue
		den := math.Abs(a)
		if den < eps {
			den = eps
		}
		sum += math.Abs(a-p) / den
	}
	return sum / float64(m)
}

func TestHoltWintersFit_ShortSeries(t *testing.T) {
	cfg := DefaultModelConfig()
	cfg.SeasonalPeriod = 288
	m := cfg.SeasonalPeriod
	// Fewer than 2*m points → simple exponential path
	short := generateSinusoidal(2*m-1, 288, 10, 100)

	h := NewHoltWintersModel(cfg)
	if err := h.Fit(short); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !h.fitted {
		t.Fatal("expected fitted after short series")
	}
	if len(h.seasonal) > 0 {
		t.Fatalf("expected no seasonal component for short series, got len=%d", len(h.seasonal))
	}
	out := h.Forecast(8)
	if len(out) != 8 {
		t.Fatalf("short-series fallback should still forecast, got len=%d", len(out))
	}
	for i, fr := range out {
		if math.IsNaN(fr.PredictedValue) || math.IsInf(fr.PredictedValue, 0) {
			t.Fatalf("step %d: invalid prediction %v", i, fr.PredictedValue)
		}
	}
}

func TestHoltWintersForecast_Sinusoidal(t *testing.T) {
	cfg := DefaultModelConfig()
	cfg.SeasonalPeriod = 288
	const period = 288
	const trainN = 600
	const horizon = 12
	base := 50.0
	amp := 25.0

	train := generateSinusoidal(trainN, period, amp, base)
	h := NewHoltWintersModel(cfg)
	if err := h.Fit(train); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	pred := h.Forecast(horizon)
	if len(pred) != horizon {
		t.Fatalf("expected %d forecast steps, got %d", horizon, len(pred))
	}

	actual := make([]DataPoint, horizon)
	for i := range horizon {
		idx := trainN + i
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Add(5 * time.Minute * time.Duration(idx))
		v := base + amp*math.Sin(2*math.Pi*float64(idx)/float64(period))
		actual[i] = DataPoint{Timestamp: t0, Value: v}
	}

	mape := forecastMAPE(actual, pred)
	if mape > 0.25 || math.IsNaN(mape) {
		t.Fatalf("MAPE want < 25%%, got %.4f", mape*100)
	}
}

func TestHoltWintersForecast_Constant(t *testing.T) {
	cfg := DefaultModelConfig()
	n := 400
	val := 123.45
	points := make([]DataPoint, n)
	for i := range n {
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Add(5 * time.Minute * time.Duration(i))
		points[i] = DataPoint{Timestamp: t0, Value: val}
	}
	h := NewHoltWintersModel(cfg)
	if err := h.Fit(points); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	out := h.Forecast(24)
	if len(out) != 24 {
		t.Fatalf("forecast len: %d", len(out))
	}
	for i, fr := range out {
		if math.Abs(fr.PredictedValue-val) > 1e-2 {
			t.Fatalf("step %d: predicted %v, want ~%v", i, fr.PredictedValue, val)
		}
	}
}

func TestHoltWintersForecast_Empty(t *testing.T) {
	h := NewHoltWintersModel(DefaultModelConfig())
	if err := h.Fit(nil); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if out := h.Forecast(5); out != nil {
		t.Fatalf("expected nil forecast for empty fit, got len=%d", len(out))
	}
}
