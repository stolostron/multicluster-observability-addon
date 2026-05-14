package features

import (
	"testing"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func TestExtractFeatures_Empty(t *testing.T) {
	v := ExtractFeatures(nil, DefaultFeatureConfig())
	if v != (FeatureVector{}) {
		t.Fatalf("expected zero FeatureVector, got %#v", v)
	}
}

func TestExtractFeatures_BusinessHours(t *testing.T) {
	// Wednesday 2024-01-03 10:00 UTC; default business hours 09–17 UTC
	ts := time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC)
	pts := []prediction.DataPoint{{Timestamp: ts, Value: 1.0}}
	v := ExtractFeatures(pts, DefaultFeatureConfig())
	if v.IsBusinessHours != 1.0 {
		t.Fatalf("IsBusinessHours=%v, want 1.0", v.IsBusinessHours)
	}
}
