// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	Subsystem = "mcoa"

	ResultSuccess = "success"
	ResultError   = "error"

	StageValidation = "validation"
	StageRender     = "render"

	SubsystemOverall     = "overall"
	SubsystemMetrics     = "metrics"
	SubsystemLogs        = "logs"
	SubsystemTraces      = "traces"
	SubsystemThanos      = "thanos"
	SubsystemRightSizing = "rightsizing"
)

var (
	// ManifestRenderDuration tracks duration of manifest rendering and sorting.
	ManifestRenderDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: Subsystem,
			Name:      "manifest_render_duration_seconds",
			Help:      "Duration in seconds to render and sort manifests for a managed cluster.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"result"},
	)

	// ManifestRenderErrors counts errors during manifest generation by stage.
	ManifestRenderErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "manifest_render_errors_total",
			Help:      "Total count of manifest rendering failures categorized by stage.",
		},
		[]string{"stage"},
	)

	// HealthCheckStatus represents the pass/fail state of specific health checks for a managed cluster.
	HealthCheckStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: Subsystem,
			Name:      "health_check_status",
			Help:      "Health check status per subsystem for a managed cluster (1=healthy, 0=unhealthy).",
		},
		[]string{"cluster", "subsystem"},
	)

	// HealthCheckFailures counts health check failures by subsystem.
	HealthCheckFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "health_check_failures_total",
			Help:      "Total count of health check failures categorized by subsystem.",
		},
		[]string{"subsystem"},
	)

	// ClusterReconcileTotal counts manifest reconciliations per managed cluster and result.
	ClusterReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: Subsystem,
			Name:      "cluster_reconcile_total",
			Help:      "Total number of manifest reconciliations per managed cluster.",
		},
		[]string{"cluster", "result"},
	)
)

func init() {
	crmetrics.Registry.MustRegister(
		ManifestRenderDuration,
		ManifestRenderErrors,
		HealthCheckStatus,
		HealthCheckFailures,
		ClusterReconcileTotal,
	)
}
