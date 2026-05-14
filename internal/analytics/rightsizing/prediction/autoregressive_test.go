package prediction

import (
	"math"
	"testing"
	"time"
)

func TestARModel_FitAndForecast(t *testing.T) {
	const n = 200
	points := make([]DataPoint, n)
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		t0 := start.Add(5 * time.Minute * time.Duration(i))
		// Simple linear trend (deterministic, easy to sanity-check)
		points[i] = DataPoint{Timestamp: t0, Value: 10.0 + 0.35*float64(i)}
	}

	cfg := DefaultModelConfig()
	cfg.MaxOrder = 8
	m := NewARModel(cfg)
	if err := m.Fit(points); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	out := m.Forecast(10)
	if len(out) != 10 {
		t.Fatalf("forecast len: got %d want 10", len(out))
	}
	last := points[len(points)-1].Value
	for i, fr := range out {
		if math.IsNaN(fr.PredictedValue) || math.IsInf(fr.PredictedValue, 0) {
			t.Fatalf("step %d: invalid prediction %v", i, fr.PredictedValue)
		}
		if math.IsNaN(fr.ConfidenceLower) || math.IsInf(fr.ConfidenceLower, 0) ||
			math.IsNaN(fr.ConfidenceUpper) || math.IsInf(fr.ConfidenceUpper, 0) {
			t.Fatalf("step %d: invalid CI bounds", i)
		}
		// Continuation of a steep trend should stay in a generous band
		if fr.PredictedValue < last*0.5 || fr.PredictedValue > last*3.0 {
			t.Fatalf("step %d: predicted %v implausible vs last obs %v", i, fr.PredictedValue, last)
		}
	}
}

func TestARModel_ShortSeries(t *testing.T) {
	points := []DataPoint{
		{Timestamp: time.Unix(0, 0), Value: 1},
		{Timestamp: time.Unix(60, 0), Value: 2},
		{Timestamp: time.Unix(120, 0), Value: 3},
	}
	m := NewARModel(DefaultModelConfig())
	if err := m.Fit(points); err == nil {
		t.Fatal("expected error for short series")
	}
}

func TestARModel_ConstantSeries(t *testing.T) {
	const v = 42.7
	const n = 100
	points := make([]DataPoint, n)
	for i := 0; i < n; i++ {
		points[i] = DataPoint{
			Timestamp: time.Date(2024, 6, 1, 0, 0, i, 0, time.UTC),
			Value:     v,
		}
	}
	m := NewARModel(DefaultModelConfig())
	if err := m.Fit(points); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	out := m.Forecast(10)
	if len(out) != 10 {
		t.Fatalf("forecast len %d", len(out))
	}
	for i, fr := range out {
		if math.Abs(fr.PredictedValue-v) > 0.01 {
			t.Fatalf("step %d: predicted %v, want near %v", i, fr.PredictedValue, v)
		}
	}
}
