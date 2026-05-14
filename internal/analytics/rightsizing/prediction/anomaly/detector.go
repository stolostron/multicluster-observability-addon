package anomaly

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// CompositeDetector runs multiple detectors and merges their results.
type CompositeDetector struct {
	zscore   *ZScoreDetector
	rate     *RateOfChangeDetector
	adaptive *AdaptiveThresholdDetector
}

// NewCompositeDetector builds a composite over the default three algorithms.
func NewCompositeDetector(cfg DetectorConfig) *CompositeDetector {
	return &CompositeDetector{
		zscore:   NewZScoreDetector(cfg),
		rate:     NewRateOfChangeDetector(cfg),
		adaptive: NewAdaptiveThresholdDetector(cfg),
	}
}

// Detect merges, deduplicates by typical sampling interval, and sorts by time.
func (c *CompositeDetector) Detect(points []prediction.DataPoint) []prediction.AnomalyResult {
	if len(points) == 0 {
		return nil
	}
	var merged []prediction.AnomalyResult
	merged = append(merged, c.zscore.Detect(points)...)
	merged = append(merged, c.rate.Detect(points)...)
	merged = append(merged, c.adaptive.Detect(points)...)

	if len(merged) == 0 {
		return nil
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp.Before(merged[j].Timestamp)
	})

	interval := typicalInterval(points)
	deduped := dedupeByInterval(merged, interval)
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].Timestamp.Before(deduped[j].Timestamp)
	})
	return deduped
}

func dedupeByInterval(results []prediction.AnomalyResult, interval time.Duration) []prediction.AnomalyResult {
	if len(results) == 0 || interval <= 0 {
		return results
	}
	out := make([]prediction.AnomalyResult, 0, len(results))
	for _, r := range results {
		if len(out) == 0 {
			out = append(out, r)
			continue
		}
		last := &out[len(out)-1]
		dt := r.Timestamp.Sub(last.Timestamp)
		if dt < 0 {
			dt = -dt
		}
		if dt <= interval {
			if r.Score > last.Score {
				*last = r
			}
		} else {
			out = append(out, r)
		}
	}
	return out
}

func typicalInterval(points []prediction.DataPoint) time.Duration {
	if len(points) < 2 {
		return time.Minute
	}
	sorted := append([]prediction.DataPoint(nil), points...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})
	var diffs []int64
	for i := 0; i < len(sorted)-1; i++ {
		d := sorted[i+1].Timestamp.Sub(sorted[i].Timestamp)
		if d > 0 {
			diffs = append(diffs, int64(d))
		}
	}
	if len(diffs) == 0 {
		return time.Minute
	}
	slices.Sort(diffs)
	return medianDuration(diffs)
}

func medianDuration(diffs []int64) time.Duration {
	mid := len(diffs) / 2
	if len(diffs)%2 == 0 {
		return time.Duration((diffs[mid-1] + diffs[mid]) / 2)
	}
	return time.Duration(diffs[mid])
}

func valuesFromPoints(points []prediction.DataPoint) []float64 {
	v := make([]float64, len(points))
	for i := range points {
		v[i] = points[i].Value
	}
	return v
}

func meanValues(points []prediction.DataPoint) float64 {
	return meanFloat(valuesFromPoints(points))
}

func meanFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stdDevPopulation(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := meanFloat(xs)
	var ss float64
	for _, x := range xs {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(xs)))
}

func cloneSortedFloats(v []float64) []float64 {
	out := append([]float64(nil), v...)
	sort.Float64s(out)
	return out
}

func quantileSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	x := p * float64(len(sorted)-1)
	lo := int(math.Floor(x))
	hi := int(math.Ceil(x))
	if lo == hi {
		return sorted[lo]
	}
	w := x - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}
