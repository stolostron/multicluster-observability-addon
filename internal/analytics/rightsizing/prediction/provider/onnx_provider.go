package provider

import (
	"context"
	"errors"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// ONNXProvider holds an ONNX model payload for future onnxruntime-go integration.
// Real inference requires CGO and a build tag; see Forecast.
type ONNXProvider struct {
	modelData []byte
	loaded    bool
}

// NewONNXProvider constructs an ONNX-backed provider. modelData may be nil until loaded from a ConfigMap.
func NewONNXProvider(modelData []byte) *ONNXProvider {
	return &ONNXProvider{
		modelData: modelData,
		loaded:    len(modelData) > 0,
	}
}

// Forecast is not implemented until onnxruntime-go is linked under a CGO + build-tag gated build.
func (p *ONNXProvider) Forecast(ctx context.Context, req prediction.ForecastRequest) ([]prediction.ForecastResult, error) {
	_ = ctx
	_ = req
	// Stub: wire onnxruntime-go with //go:build onnx (or similar) for real inference.
	return nil, errors.New("ONNX inference not available: onnxruntime-go not linked")
}

// Train reports that ONNX models are supplied pre-trained.
func (p *ONNXProvider) Train(ctx context.Context, points []prediction.DataPoint) error {
	_ = ctx
	_ = points
	return errors.New("ONNX provider uses pre-trained models; call Forecast directly")
}

// DetectAnomalies is not supported for external ONNX model graphs.
func (p *ONNXProvider) DetectAnomalies(ctx context.Context, points []prediction.DataPoint) ([]prediction.AnomalyResult, error) {
	_ = ctx
	_ = points
	return nil, nil
}

// Explain returns minimal ONNX metadata.
func (p *ONNXProvider) Explain(ctx context.Context, req prediction.ForecastRequest) (map[string]interface{}, error) {
	_ = ctx
	_ = req
	return map[string]interface{}{
		"provider":     "onnx",
		"model_loaded": p.loaded,
	}, nil
}

// ProviderType implements PredictionProvider.
func (p *ONNXProvider) ProviderType() ProviderType {
	return ProviderONNX
}

// PrivacyLevel reports on-cluster execution once inference is wired.
func (p *ONNXProvider) PrivacyLevel() PrivacyLevel {
	return NoExfiltration
}
