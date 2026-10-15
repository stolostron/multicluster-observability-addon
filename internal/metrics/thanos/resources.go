package thanos

import (
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type resourceSizeMap map[string]string

var componentResources = map[string]struct {
	cpuRequest resourceSizeMap
	memRequest resourceSizeMap
}{
	config.ThanosReceiveIngesterContainerID: {
		cpuRequest: resourceSizeMap{
			"default": "300m",
			"small":   "2",
			"medium":  "4",
			"large":   "5",
			"xlarge":  "5",
			"2xlarge": "6",
			"4xlarge": "10",
		},
		memRequest: resourceSizeMap{
			"default": "512Mi",
			"small":   "6Gi",
			"medium":  "12Gi",
			"large":   "24Gi",
			"xlarge":  "36Gi",
			"2xlarge": "52Gi",
			"4xlarge": "128Gi",
		},
	},
	config.ThanosReceiveRouterContainerID: {
		cpuRequest: resourceSizeMap{
			"default": "100m",
			"small":   "250m",
			"medium":  "500m",
			"large":   "500m",
			"xlarge":  "1",
			"2xlarge": "1",
			"4xlarge": "1500m",
		},
		memRequest: resourceSizeMap{
			"default": "256Mi",
			"small":   "512Mi",
			"medium":  "1Gi",
			"large":   "1Gi",
			"xlarge":  "2Gi",
			"2xlarge": "2Gi",
			"4xlarge": "4Gi",
		},
	},
	config.ThanosQueryContainerID: {
		cpuRequest: resourceSizeMap{
			"default": "300m",
			"small":   "1500m",
			"medium":  "2",
			"large":   "4",
			"xlarge":  "6",
			"2xlarge": "6",
			"4xlarge": "7",
		},
		memRequest: resourceSizeMap{
			"default": "1Gi",
			"small":   "4Gi",
			"medium":  "6Gi",
			"large":   "8Gi",
			"xlarge":  "10Gi",
			"2xlarge": "15Gi",
			"4xlarge": "18Gi",
		},
	},
	config.ThanosQueryFrontendContainerID: {
		cpuRequest: resourceSizeMap{
			"default": "100m",
			"small":   "1",
			"medium":  "1",
			"large":   "2",
			"xlarge":  "2",
			"2xlarge": "4",
			"4xlarge": "4",
		},
		memRequest: resourceSizeMap{
			"default": "256Mi",
			"small":   "512Mi",
			"medium":  "2Gi",
			"large":   "5Gi",
			"xlarge":  "8Gi",
			"2xlarge": "10Gi",
			"4xlarge": "12Gi",
		},
	},
	config.ThanosRulerContainerID: {
		cpuRequest: resourceSizeMap{
			"default": "50m",
			"small":   "500m",
			"medium":  "1",
			"large":   "3",
			"xlarge":  "3",
			"2xlarge": "4",
			"4xlarge": "6",
		},
		memRequest: resourceSizeMap{
			"default": "512Mi",
			"small":   "1Gi",
			"medium":  "2Gi",
			"large":   "4Gi",
			"xlarge":  "6Gi",
			"2xlarge": "10Gi",
			"4xlarge": "15Gi",
		},
	},
	config.ThanosCompactContainerID: {
		cpuRequest: resourceSizeMap{
			"default": "500m",
			"small":   "500m",
			"medium":  "1",
			"large":   "3",
			"xlarge":  "3",
			"2xlarge": "4",
			"4xlarge": "6",
		},
		memRequest: resourceSizeMap{
			"default": "1Gi",
			"small":   "1Gi",
			"medium":  "2Gi",
			"large":   "4Gi",
			"xlarge":  "8Gi",
			"2xlarge": "12Gi",
			"4xlarge": "18Gi",
		},
	},
	config.ThanosStoreContainerID: {
		cpuRequest: resourceSizeMap{
			"default": "100m",
			"small":   "2",
			"medium":  "3",
			"large":   "3",
			"xlarge":  "3",
			"2xlarge": "4",
			"4xlarge": "6",
		},
		memRequest: resourceSizeMap{
			"default": "1Gi",
			"small":   "4Gi",
			"medium":  "6Gi",
			"large":   "8Gi",
			"xlarge":  "12Gi",
			"2xlarge": "15Gi",
			"4xlarge": "20Gi",
		},
	},
}

// GetResourcesForSize returns the resource requirements for a Thanos component
// based on the given instance size. Falls back to static defaults if the size
// is empty or not found in the sizing map.
func GetResourcesForSize(containerID, instanceSize string) *corev1.ResourceRequirements {
	if instanceSize == "" {
		instanceSize = "default"
	}

	comp, ok := componentResources[containerID]
	if !ok {
		return nil
	}

	cpuReq, okCPU := comp.cpuRequest[instanceSize]
	memReq, okMem := comp.memRequest[instanceSize]
	if !okCPU || !okMem {
		return nil
	}

	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuReq),
			corev1.ResourceMemory: resource.MustParse(memReq),
		},
	}
}
