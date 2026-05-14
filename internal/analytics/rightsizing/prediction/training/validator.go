package training

import (
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// ValidateTrainingResult returns true if newMAPE improves oldMAPE by at least 5%,
// or if oldMAPE is zero / infinite (no prior training).
func ValidateTrainingResult(oldMAPE, newMAPE float64) bool {
	if math.IsNaN(newMAPE) || math.IsInf(newMAPE, 1) {
		return false
	}
	if oldMAPE == 0 || math.IsInf(oldMAPE, 0) {
		return true
	}
	return newMAPE < oldMAPE*0.95
}

// ComputeMAPE computes mean absolute percentage error over aligned actual vs predicted points.
func ComputeMAPE(actual, predicted []prediction.DataPoint) float64 {
	if len(actual) == 0 || len(predicted) == 0 {
		return math.Inf(1)
	}
	n := len(actual)
	if len(predicted) < n {
		n = len(predicted)
	}
	eps := 1e-9
	var sum float64
	for i := 0; i < n; i++ {
		a := actual[i].Value
		p := predicted[i].Value
		den := math.Abs(a)
		if den < eps {
			den = eps
		}
		sum += math.Abs(a-p) / den
	}
	return sum / float64(n)
}
