// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
)

// ModelAccuracyPanel shows a one-number summary of model fit for the selected namespace.
func ModelAccuracyPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Prediction Model Accuracy",
		Description: "Model accuracy score (e.g. validation error or fit quality) for workload forecasting in the selected namespace",
		Query:       `max(acm_rs:prediction_model_accuracy{cluster="$cluster", namespace="$namespace"})`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    3,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}
