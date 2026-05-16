package training

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func TestWorkloadKey_String(t *testing.T) {
	k := WorkloadKey{
		Cluster:   "cluster",
		Namespace: "ns",
		Workload:  "wl",
		Resource:  "cpu",
	}
	if got, want := k.String(), "cluster/ns/wl/cpu"; got != want {
		t.Fatalf("String(): got %q, want %q", got, want)
	}
}

func TestDefaultTrainingConfig(t *testing.T) {
	cfg := DefaultTrainingConfig()
	if !cfg.NamespaceEnabled {
		t.Error("NamespaceEnabled: want true")
	}
	if cfg.WorkloadEnabled {
		t.Error("WorkloadEnabled: want false")
	}
	if cfg.GPUEnabled {
		t.Error("GPUEnabled: want false")
	}
	if cfg.VMEnabled {
		t.Error("VMEnabled: want false")
	}
	if cfg.IntervalHours != 1 {
		t.Errorf("IntervalHours: got %d, want 1", cfg.IntervalHours)
	}
	if cfg.HistoryDays != 7 {
		t.Errorf("HistoryDays: got %d, want 7", cfg.HistoryDays)
	}
	// Default model config should be wired
	defaultModel := prediction.DefaultModelConfig()
	if cfg.ModelConfig != defaultModel {
		t.Errorf("ModelConfig differs from prediction.DefaultModelConfig(): %+v vs %+v", cfg.ModelConfig, defaultModel)
	}
}

func TestResourceType_Constants(t *testing.T) {
	cases := []struct {
		rt   ResourceType
		want string
	}{
		{ResourceNamespaceCPU, "namespace_cpu"},
		{ResourceNamespaceMemory, "namespace_memory"},
		{ResourceWorkloadCPU, "workload_cpu"},
		{ResourceWorkloadMemory, "workload_memory"},
		{ResourceGPUUtilization, "gpu_utilization"},
		{ResourceGPUMemory, "gpu_memory"},
		{ResourceVMCPU, "vm_cpu"},
		{ResourceVMMemory, "vm_memory"},
	}
	for _, tc := range cases {
		if got := string(tc.rt); got != tc.want {
			t.Errorf("ResourceType constant: got %q, want %q", got, tc.want)
		}
	}
}
