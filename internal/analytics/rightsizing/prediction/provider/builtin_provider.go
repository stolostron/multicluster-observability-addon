package provider

import (
	"context"
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// BuiltinProvider runs the in-process ensemble (no external services).
type BuiltinProvider struct {
	ensemble *prediction.EnsembleForecaster
}

// NewBuiltinProvider constructs the default on-cluster forecaster implementation.
func NewBuiltinProvider(cfg prediction.ModelConfig) *BuiltinProvider {
	return &BuiltinProvider{
		ensemble: prediction.NewEnsembleForecaster(cfg),
	}
}

// Forecast delegates to the ensemble.
func (p *BuiltinProvider) Forecast(ctx context.Context, req prediction.ForecastRequest) ([]prediction.ForecastResult, error) {
	_ = ctx
	return p.ensemble.Forecast(req), nil
}

// Train fits ensemble members and refreshes weights when backtest improves sufficiently.
func (p *BuiltinProvider) Train(ctx context.Context, points []prediction.DataPoint) error {
	_ = ctx
	return p.ensemble.Train(points)
}

// DetectAnomalies is not implemented yet (reserved for Task 8).
func (p *BuiltinProvider) DetectAnomalies(ctx context.Context, points []prediction.DataPoint) ([]prediction.AnomalyResult, error) {
	_ = ctx
	_ = points
	return nil, nil
}

// Explain returns ensemble weights and the currently dominant model name.
func (p *BuiltinProvider) Explain(ctx context.Context, req prediction.ForecastRequest) (map[string]any, error) {
	_ = ctx
	_ = req
	w := p.ensemble.Weights()
	dom := ""
	best := math.Inf(-1)
	for k, v := range w {
		if v > best {
			best = v
			dom = k
		}
	}
	return map[string]any{
		"weights":       w,
		"dominantModel": dom,
	}, nil
}

// ProviderType implements PredictionProvider.
func (p *BuiltinProvider) ProviderType() ProviderType {
	return ProviderBuiltin
}

// PrivacyLevel reports that data never leaves the cluster via this provider.
func (p *BuiltinProvider) PrivacyLevel() PrivacyLevel {
	return NoExfiltration
}
