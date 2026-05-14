package provider

import (
	"context"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// ProviderType identifies how predictions are produced.
type ProviderType string

const (
	ProviderBuiltin  ProviderType = "builtin"
	ProviderONNX     ProviderType = "onnx"
	ProviderExternal ProviderType = "external"
	ProviderCustom   ProviderType = "custom"
)

// PrivacyLevel expresses data-handling constraints for the provider.
type PrivacyLevel int

const (
	NoExfiltration PrivacyLevel = iota
	ConsentRequired
)

// PredictionProvider is implemented by concrete forecast engines (builtin, ONNX, etc.).
type PredictionProvider interface {
	Forecast(ctx context.Context, req prediction.ForecastRequest) ([]prediction.ForecastResult, error)
	Train(ctx context.Context, points []prediction.DataPoint) error
	DetectAnomalies(ctx context.Context, points []prediction.DataPoint) ([]prediction.AnomalyResult, error)
	Explain(ctx context.Context, req prediction.ForecastRequest) (map[string]interface{}, error)
	ProviderType() ProviderType
	PrivacyLevel() PrivacyLevel
}
