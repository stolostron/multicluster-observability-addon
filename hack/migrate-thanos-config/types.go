// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License, Version 2.0

package main

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

// MigrationConfig is the intermediate representation between the MCO
// MultiClusterObservability CR and the MCOA Thanos CRs. It is produced by
// "capture" and consumed by "apply", so the two steps can run at different
// points of an upgrade (capture before the MCO stack is removed, apply after
// the MCOA Thanos CRs exist).
//
// Only user-declared values from the MCO CR are captured (spec.advanced and
// spec.storageConfig). MCO defaults and instanceSize-derived sizing are
// deliberately NOT carried over: MCOA has its own defaults and its own
// instanceSize handling.
type MigrationConfig struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`

	Source SourceInfo `json:"source"`

	Compact CompactConfig `json:"compact,omitzero"`
	Receive ReceiveConfig `json:"receive,omitzero"`
	Ruler   RulerConfig   `json:"ruler,omitzero"`
	Query   QueryConfig   `json:"query,omitzero"`
	Store   StoreConfig   `json:"store,omitzero"`
	// Memcached settings are applied to the memcached-store and
	// memcached-query-frontend Deployments. They persist only on MCOA versions
	// that manage those Deployments with SSA + OnSpokePresent (see README).
	StoreMemcached         MemcachedConfig `json:"storeMemcached,omitzero"`
	QueryFrontendMemcached MemcachedConfig `json:"queryFrontendMemcached,omitzero"`

	// SuggestedAddOnDeploymentConfig holds node placement and per-container
	// resources extracted from the MCO CR, expressed as a patch for the
	// AddOnDeploymentConfig that deploys MCOA. These fields are managed by
	// the ADC on the MCOA side and must NOT be written into the Thanos CRs,
	// so "apply" ignores them; the snippet is advisory only.
	SuggestedAddOnDeploymentConfig *ADDCSuggestion `json:"suggestedAddOnDeploymentConfig,omitempty"`

	// Warnings list MCO settings that have no Thanos CR equivalent (or are
	// not durably patchable) and were intentionally not migrated.
	Warnings []string `json:"warnings,omitempty"`
}

type SourceInfo struct {
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
	// CapturedAt is the time the capture ran, for audit purposes.
	CapturedAt string `json:"capturedAt"`
}

type CompactConfig struct {
	RetentionRaw              string `json:"retentionRaw,omitempty"`
	Retention5m               string `json:"retention5m,omitempty"`
	Retention1h               string `json:"retention1h,omitempty"`
	StorageSize               string `json:"storageSize,omitempty"`
	StorageClass              string `json:"storageClass,omitempty"`
	DownsampleConcurrency     *int32 `json:"downsampleConcurrency,omitempty"`
	BlockMetaFetchConcurrency *int32 `json:"blockMetaFetchConcurrency,omitempty"`
	// CleanupInterval is mapped from MCO compact.debug.waitInterval. MCO used
	// that value to synchronize --compact.cleanup-interval, so only the
	// cleanup cadence is preserved here (see warnings on capture).
	CleanupInterval string `json:"cleanupInterval,omitempty"`
	// LogLevel maps MCO compact.debug.logLevel to the CommonFields log level.
	LogLevel   string             `json:"logLevel,omitempty"`
	Containers []corev1.Container `json:"containers,omitempty"`
}

type ReceiveConfig struct {
	// Replicas maps MCO advanced.receive.replicas (the receive ingester
	// StatefulSet replica count) to the "default" hashring.
	Replicas     *int32 `json:"replicas,omitempty"`
	Retention    string `json:"retention,omitempty"`
	StorageSize  string `json:"storageSize,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
	// LogLevel maps MCO receive.debug.logLevel onto both the router and the
	// ingester hashring: MCO ran a single receive component, MCOA runs both.
	LogLevel   string             `json:"logLevel,omitempty"`
	Containers []corev1.Container `json:"containers,omitempty"`
}

type RulerConfig struct {
	Replicas     *int32             `json:"replicas,omitempty"`
	EvalInterval string             `json:"evalInterval,omitempty"`
	Retention    string             `json:"retention,omitempty"`
	StorageSize  string             `json:"storageSize,omitempty"`
	StorageClass string             `json:"storageClass,omitempty"`
	Containers   []corev1.Container `json:"containers,omitempty"`
}

type QueryConfig struct {
	Replicas              *int32             `json:"replicas,omitempty"`
	QueryFrontendReplicas *int32             `json:"queryFrontendReplicas,omitempty"`
	Containers            []corev1.Container `json:"containers,omitempty"`
	// QueryFrontendContainers maps MCO advanced.queryFrontend.containers.
	QueryFrontendContainers []corev1.Container `json:"queryFrontendContainers,omitempty"`
}

type StoreConfig struct {
	Replicas     *int32             `json:"replicas,omitempty"`
	StorageSize  string             `json:"storageSize,omitempty"`
	StorageClass string             `json:"storageClass,omitempty"`
	Containers   []corev1.Container `json:"containers,omitempty"`
}

// MemcachedConfig mirrors the MCO CacheConfig knobs that map onto the
// memcached container args and the Deployment replica count.
type MemcachedConfig struct {
	Replicas        *int32 `json:"replicas,omitempty"`
	MemoryLimitMB   *int32 `json:"memoryLimitMb,omitempty"`
	MaxItemSize     string `json:"maxItemSize,omitempty"`
	ConnectionLimit *int32 `json:"connectionLimit,omitempty"`
}

func (m MemcachedConfig) isEmpty() bool {
	return m.Replicas == nil && m.MemoryLimitMB == nil && m.MaxItemSize == "" && m.ConnectionLimit == nil
}

// ADDCSuggestion mirrors the relevant AddOnDeploymentConfig spec fields.
type ADDCSuggestion struct {
	NodeSelector         map[string]string       `json:"nodeSelector,omitempty"`
	Tolerations          []corev1.Toleration     `json:"tolerations,omitempty"`
	ResourceRequirements []ADDCContainerResource `json:"resourceRequirements,omitempty"`
}

type ADDCContainerResource struct {
	ContainerID string                      `json:"containerID"`
	Resources   corev1.ResourceRequirements `json:"resources"`
}

func (c *MigrationConfig) isEmpty() bool {
	return reflect.DeepEqual(c.Compact, CompactConfig{}) &&
		reflect.DeepEqual(c.Receive, ReceiveConfig{}) &&
		reflect.DeepEqual(c.Ruler, RulerConfig{}) &&
		reflect.DeepEqual(c.Query, QueryConfig{}) &&
		reflect.DeepEqual(c.Store, StoreConfig{}) &&
		c.StoreMemcached.isEmpty() &&
		c.QueryFrontendMemcached.isEmpty() &&
		c.SuggestedAddOnDeploymentConfig == nil
}
