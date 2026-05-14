package provider

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func TestCreate_Builtin(t *testing.T) {
	p, err := Create(prediction.ProviderConfig{Type: "builtin"})
	if err != nil {
		t.Fatalf("Create builtin: %v", err)
	}
	if p.ProviderType() != ProviderBuiltin {
		t.Fatalf("ProviderType=%v, want %v", p.ProviderType(), ProviderBuiltin)
	}
}

func TestCreate_Unknown(t *testing.T) {
	_, err := Create(prediction.ProviderConfig{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}
