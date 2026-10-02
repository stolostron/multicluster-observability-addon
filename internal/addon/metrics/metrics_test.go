// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRegistrationAndRecording(t *testing.T) {
	// Verify ManifestRenderDuration
	ManifestRenderDuration.WithLabelValues(ResultSuccess).Observe(0.123)
	ManifestRenderDuration.WithLabelValues(ResultError).Observe(0.456)

	// Verify ManifestRenderErrors
	ManifestRenderErrors.WithLabelValues(StageValidation).Inc()
	ManifestRenderErrors.WithLabelValues(StageRender).Inc()

	assert.InDelta(t, 1.0, testutil.ToFloat64(ManifestRenderErrors.WithLabelValues(StageValidation)), 0.001)
	assert.InDelta(t, 1.0, testutil.ToFloat64(ManifestRenderErrors.WithLabelValues(StageRender)), 0.001)

	// Verify HealthCheckStatus
	HealthCheckStatus.WithLabelValues("cluster-1", SubsystemOverall).Set(1)
	HealthCheckStatus.WithLabelValues("cluster-1", SubsystemMetrics).Set(1)
	HealthCheckStatus.WithLabelValues("cluster-2", SubsystemOverall).Set(0)
	HealthCheckStatus.WithLabelValues("cluster-2", SubsystemLogs).Set(0)

	assert.InDelta(t, 1.0, testutil.ToFloat64(HealthCheckStatus.WithLabelValues("cluster-1", SubsystemOverall)), 0.001)
	assert.InDelta(t, 1.0, testutil.ToFloat64(HealthCheckStatus.WithLabelValues("cluster-1", SubsystemMetrics)), 0.001)
	assert.InDelta(t, 0.0, testutil.ToFloat64(HealthCheckStatus.WithLabelValues("cluster-2", SubsystemOverall)), 0.001)
	assert.InDelta(t, 0.0, testutil.ToFloat64(HealthCheckStatus.WithLabelValues("cluster-2", SubsystemLogs)), 0.001)

	// Verify HealthCheckFailures
	HealthCheckFailures.WithLabelValues(SubsystemMetrics).Inc()
	HealthCheckFailures.WithLabelValues(SubsystemLogs).Add(2)

	assert.InDelta(t, 1.0, testutil.ToFloat64(HealthCheckFailures.WithLabelValues(SubsystemMetrics)), 0.001)
	assert.InDelta(t, 2.0, testutil.ToFloat64(HealthCheckFailures.WithLabelValues(SubsystemLogs)), 0.001)
}
