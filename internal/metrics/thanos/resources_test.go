package thanos

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGetResourcesForSize(t *testing.T) {
	tests := []struct {
		name         string
		containerID  string
		instanceSize string
		expectNil    bool
		expectCPU    string
		expectMem    string
	}{
		{
			name:         "empty instance size defaults to default",
			containerID:  config.ThanosReceiveIngesterContainerID,
			instanceSize: "",
			expectCPU:    "300m",
			expectMem:    "512Mi",
		},
		{
			name:         "unknown container ID returns nil",
			containerID:  "unknown:container",
			instanceSize: "medium",
			expectNil:    true,
		},
		{
			name:         "unknown size returns nil",
			containerID:  config.ThanosReceiveIngesterContainerID,
			instanceSize: "gigantic",
			expectNil:    true,
		},
		{
			name:         "receive ingester default size",
			containerID:  config.ThanosReceiveIngesterContainerID,
			instanceSize: "default",
			expectCPU:    "300m",
			expectMem:    "512Mi",
		},
		{
			name:         "receive ingester large size",
			containerID:  config.ThanosReceiveIngesterContainerID,
			instanceSize: "large",
			expectCPU:    "5",
			expectMem:    "24Gi",
		},
		{
			name:         "query medium size",
			containerID:  config.ThanosQueryContainerID,
			instanceSize: "medium",
			expectCPU:    "2",
			expectMem:    "6Gi",
		},
		{
			name:         "compact 4xlarge size",
			containerID:  config.ThanosCompactContainerID,
			instanceSize: "4xlarge",
			expectCPU:    "6",
			expectMem:    "18Gi",
		},
		{
			name:         "memcached store large size",
			containerID:  config.MemcachedStoreContainerID,
			instanceSize: "large",
			expectCPU:    "500m",
			expectMem:    "2Gi",
		},
		{
			name:         "memcached query-frontend default size",
			containerID:  config.MemcachedQueryFrontendContainerID,
			instanceSize: "default",
			expectCPU:    "45m",
			expectMem:    "128Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GetResourcesForSize(tt.containerID, tt.instanceSize)
			if tt.expectNil {
				assert.Nil(t, res)
				return
			}
			assert.NotNil(t, res)
			assert.Equal(t, resource.MustParse(tt.expectCPU), res.Requests["cpu"])
			assert.Equal(t, resource.MustParse(tt.expectMem), res.Requests["memory"])
		})
	}
}
