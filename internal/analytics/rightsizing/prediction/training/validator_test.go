package training

import (
	"math"
	"testing"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func TestValidateTrainingResult_FirstRun(t *testing.T) {
	if !ValidateTrainingResult(0, 0.42) {
		t.Fatal("oldMAPE=0 should accept any finite newMAPE")
	}
	if !ValidateTrainingResult(0, 1e9) {
		t.Fatal("oldMAPE=0 should accept large finite newMAPE")
	}
}

func TestValidateTrainingResult_Improvement(t *testing.T) {
	if !ValidateTrainingResult(0.10, 0.08) {
		t.Fatal("expected true for >5% MAPE reduction")
	}
}

func TestValidateTrainingResult_InsufficientImprovement(t *testing.T) {
	if ValidateTrainingResult(0.10, 0.097) {
		t.Fatal("expected false when improvement is under 5%")
	}
}

func TestValidateTrainingResult_Worse(t *testing.T) {
	if ValidateTrainingResult(0.10, 0.15) {
		t.Fatal("expected false when new MAPE is worse")
	}
}

func TestComputeMAPE_Known(t *testing.T) {
	ts := time.Unix(0, 0).UTC()
	// Single point: actual 100, predicted 80 → |100-80|/100 = 0.2
	actual := []prediction.DataPoint{{Timestamp: ts, Value: 100}}
	pred := []prediction.DataPoint{{Timestamp: ts, Value: 80}}
	if got := ComputeMAPE(actual, pred); math.Abs(got-0.2) > 1e-9 {
		t.Fatalf("MAPE: got %v, want 0.2", got)
	}

	// Two points: (0 + |50-40|/50) / 2 = 0.1
	actual = []prediction.DataPoint{
		{Timestamp: ts, Value: 100},
		{Timestamp: ts, Value: 50},
	}
	pred = []prediction.DataPoint{
		{Timestamp: ts, Value: 100},
		{Timestamp: ts, Value: 40},
	}
	if got := ComputeMAPE(actual, pred); math.Abs(got-0.1) > 1e-9 {
		t.Fatalf("MAPE two points: got %v, want 0.1", got)
	}
}

func TestComputeMAPE_Empty(t *testing.T) {
	if got := ComputeMAPE(nil, nil); got != 0 {
		t.Fatalf("nil slices: got %v, want 0", got)
	}
	if got := ComputeMAPE([]prediction.DataPoint{}, []prediction.DataPoint{{}}); got != 0 {
		t.Fatalf("empty actual: got %v, want 0", got)
	}
	if got := ComputeMAPE([]prediction.DataPoint{{Value: 1}}, []prediction.DataPoint{}); got != 0 {
		t.Fatalf("empty predicted: got %v, want 0", got)
	}
}
