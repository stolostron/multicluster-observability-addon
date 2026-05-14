package anomaly

import (
	"math"
	"testing"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func TestZScore_DetectsSpike(t *testing.T) {
	t0 := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	points := make([]prediction.DataPoint, 100)
	for i := range points {
		points[i] = prediction.DataPoint{Timestamp: t0.Add(time.Minute * time.Duration(i)), Value: 50}
	}
	points[50].Value = 500

	d := NewZScoreDetector(DefaultDetectorConfig())
	out := d.Detect(points)
	var found bool
	for _, r := range out {
		if r.Type == "spike" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected >=1 spike anomaly; got %#v", out)
	}
}

func TestComposite_EmptySeries(t *testing.T) {
	c := NewCompositeDetector(DefaultDetectorConfig())
	if out := c.Detect(nil); out != nil {
		t.Fatalf("nil series: want nil, got %#v", out)
	}
}

func TestComposite_NormalData(t *testing.T) {
	t0 := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	points := make([]prediction.DataPoint, 100)
	for i := range points {
		v := 50 + 0.05*math.Sin(2*math.Pi*float64(i)/40)
		points[i] = prediction.DataPoint{Timestamp: t0.Add(5 * time.Minute * time.Duration(i)), Value: v}
	}
	c := NewCompositeDetector(DefaultDetectorConfig())
	out := c.Detect(points)
	if len(out) > 20 {
		t.Fatalf("expected few anomalies on smooth data, got %d: %#v", len(out), out)
	}
}
