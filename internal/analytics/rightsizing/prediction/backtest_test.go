package prediction

import (
	"math"
	"testing"
	"time"
)

func TestBacktest_ThreeModels(t *testing.T) {
	pts := generateSinusoidal(600, 288, 12, 80)
	res := Backtest(pts, DefaultModelConfig())
	if len(res) != 3 {
		t.Fatalf("expected 3 backtest results, got %d: %#v", len(res), res)
	}
	names := map[string]bool{}
	for _, r := range res {
		names[r.ModelName] = true
	}
	for _, want := range []string{"holt_winters", "stl", "ar"} {
		if !names[want] {
			t.Fatalf("missing model %q in %#v", want, res)
		}
	}
}

func TestBacktest_ShortSeries(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	short := []DataPoint{
		{Timestamp: t0, Value: 1},
		{Timestamp: t0.Add(time.Minute), Value: 2},
		{Timestamp: t0.Add(2 * time.Minute), Value: 3},
	}
	if got := Backtest(short, DefaultModelConfig()); got != nil {
		t.Fatalf("expected nil for short series, got %#v", got)
	}
}

func TestBacktest_MAPEPositive(t *testing.T) {
	pts := generateSinusoidal(600, 288, 10, 55)
	res := Backtest(pts, DefaultModelConfig())
	if len(res) == 0 {
		t.Fatal("expected backtest results")
	}
	for _, r := range res {
		if r.MAPE < 0 || math.IsNaN(r.MAPE) {
			t.Fatalf("model %q: MAPE=%v must be >= 0 and finite", r.ModelName, r.MAPE)
		}
	}
}
