package prediction

import (
	"math"
	"testing"
	"time"
)

func TestEnsemble_EqualWeights(t *testing.T) {
	e := NewEnsembleForecaster(DefaultModelConfig())
	w := e.Weights()
	const third = 1.0 / 3.0
	for _, k := range []string{"holt_winters", "stl", "ar"} {
		v, ok := w[k]
		if !ok {
			t.Fatalf("missing weight %q", k)
		}
		if math.Abs(v-third) > 0.02 {
			t.Fatalf("weight %q=%v, want ~%.3f", k, v, third)
		}
	}
}

func TestEnsemble_ForecastCI(t *testing.T) {
	cfg := DefaultModelConfig()
	cfg.SeasonalPeriod = 288
	pts := generateSinusoidal(600, 288, 15, 80)
	e := NewEnsembleForecaster(cfg)
	out := e.Forecast(ForecastRequest{Points: pts, Horizon: 12, Interval: 5 * time.Minute})
	if len(out) != 12 {
		t.Fatalf("horizon: got %d want 12", len(out))
	}
	for i, fr := range out {
		if fr.ConfidenceLower > fr.PredictedValue || fr.PredictedValue > fr.ConfidenceUpper {
			t.Fatalf("step %d: want Lower <= Predicted <= Upper, got %v / %v / %v",
				i, fr.ConfidenceLower, fr.PredictedValue, fr.ConfidenceUpper)
		}
	}
}

func TestEnsemble_ForecastNotEmpty(t *testing.T) {
	e := NewEnsembleForecaster(DefaultModelConfig())
	cfg := DefaultModelConfig()
	cfg.SeasonalPeriod = 288
	pts := generateSinusoidal(600, 288, 10, 50)
	horizon := 12
	out := e.Forecast(ForecastRequest{Points: pts, Horizon: horizon, Interval: 5 * time.Minute})
	if out == nil {
		t.Fatal("expected non-nil forecast")
	}
	if len(out) != horizon {
		t.Fatalf("len: got %d want %d", len(out), horizon)
	}
}
