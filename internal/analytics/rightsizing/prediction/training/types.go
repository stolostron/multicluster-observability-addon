package training

import (
	"fmt"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

type ResourceType string

const (
	ResourceNamespaceCPU    ResourceType = "namespace_cpu"
	ResourceNamespaceMemory ResourceType = "namespace_memory"
	ResourceWorkloadCPU     ResourceType = "workload_cpu"
	ResourceWorkloadMemory  ResourceType = "workload_memory"
	ResourceGPUUtilization  ResourceType = "gpu_utilization"
	ResourceGPUMemory       ResourceType = "gpu_memory"
	ResourceVMCPU           ResourceType = "vm_cpu"
	ResourceVMMemory        ResourceType = "vm_memory"
)

// TrainingConfig controls how often historical data is pulled and how models are trained.
type TrainingConfig struct {
	IntervalHours int
	HistoryDays   int
	ProviderType  string
	ModelConfig   prediction.ModelConfig

	NamespaceEnabled bool
	WorkloadEnabled  bool
	GPUEnabled       bool
	VMEnabled        bool

	// Optional: used when ProviderType is "external" or "custom" (see prediction.ProviderConfig).
	ExternalAPIKey     string
	CustomEndpointURL  string
	ConsentGiven       bool
}

// WorkloadKey identifies a workload resource dimension for caching and persistence.
type WorkloadKey struct {
	Cluster   string
	Namespace string
	Workload  string
	Resource  string
}

// String returns cluster/namespace/workload/resource.
func (k WorkloadKey) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", k.Cluster, k.Namespace, k.Workload, k.Resource)
}

// ShardMetadata describes one persisted shard of model state (optional bookkeeping).
type ShardMetadata struct {
	ShardIndex    int
	WorkloadCount int
	SizeBytes     int
}

// DefaultTrainingConfig returns task defaults.
func DefaultTrainingConfig() TrainingConfig {
	return TrainingConfig{
		IntervalHours:    6,
		HistoryDays:      30,
		ProviderType:     "",
		ModelConfig:      prediction.DefaultModelConfig(),
		NamespaceEnabled: true,
	}
}
