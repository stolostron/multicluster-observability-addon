package rightsizing

import (
	"strings"
	"testing"
)

func TestGenerateScrapeConfig_AllEnabled(t *testing.T) {
	cfg := GenerateScrapeConfig(true, true, true, true, true)
	if cfg == nil {
		t.Fatal("expected non-nil ScrapeConfig")
	}
	match := cfg.Spec.Params["match[]"]
	joined := strings.Join(match, " ")

	for _, m := range NamespaceMetrics {
		if !strings.Contains(joined, m) {
			t.Errorf("expected namespace metric %q in match[]", m)
		}
	}
	for _, m := range VirtualizationMetrics {
		if !strings.Contains(joined, m) {
			t.Errorf("expected virtualization metric %q in match[]", m)
		}
	}
	for _, m := range WorkloadPodMetrics {
		if !strings.Contains(joined, m) {
			t.Errorf("expected workload/pod metric %q in match[]", m)
		}
	}
	for _, m := range GPUMetrics {
		if !strings.Contains(joined, m) {
			t.Errorf("expected GPU metric %q in match[]", m)
		}
	}
	for _, m := range PredictionMetrics {
		if !strings.Contains(joined, m) {
			t.Errorf("expected prediction metric %q in match[]", m)
		}
	}
}

func TestGenerateScrapeConfig_NoneEnabled(t *testing.T) {
	cfg := GenerateScrapeConfig(false, false, false, false, false)
	if cfg != nil {
		t.Fatalf("expected nil, got ScrapeConfig with %d match params", len(cfg.Spec.Params["match[]"]))
	}
}

func TestGenerateScrapeConfig_PredictionOnly(t *testing.T) {
	cfg := GenerateScrapeConfig(false, false, false, false, true)
	if cfg == nil {
		t.Fatal("expected non-nil ScrapeConfig")
	}
	match := cfg.Spec.Params["match[]"]
	if got := len(match); got != len(PredictionMetrics) {
		t.Fatalf("match[] len: got %d, want %d", got, len(PredictionMetrics))
	}
}

func TestGenerateScrapeConfig_NamespaceOnly(t *testing.T) {
	cfg := GenerateScrapeConfig(true, false, false, false, false)
	if cfg == nil {
		t.Fatal("expected non-nil ScrapeConfig")
	}
	match := cfg.Spec.Params["match[]"]
	if got := len(match); got != len(NamespaceMetrics) {
		t.Fatalf("match[] len: got %d, want %d", got, len(NamespaceMetrics))
	}
}

func TestGenerateScrapeConfig_WorkloadAndGPU(t *testing.T) {
	cfg := GenerateScrapeConfig(false, false, true, true, false)
	if cfg == nil {
		t.Fatal("expected non-nil ScrapeConfig")
	}
	match := cfg.Spec.Params["match[]"]
	want := len(WorkloadPodMetrics) + len(GPUMetrics)
	if got := len(match); got != want {
		t.Fatalf("match[] len: got %d, want %d", got, want)
	}
	joined := strings.Join(match, " ")
	for _, m := range WorkloadPodMetrics {
		if !strings.Contains(joined, m) {
			t.Errorf("expected workload metric %q in match[]", m)
		}
	}
	for _, m := range GPUMetrics {
		if !strings.Contains(joined, m) {
			t.Errorf("expected GPU metric %q in match[]", m)
		}
	}
}
