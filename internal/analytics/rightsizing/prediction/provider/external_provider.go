package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction/privacy"
)

// ExternalProvider calls a vendor-managed prediction HTTP API (not yet configured).
type ExternalProvider struct {
	apiKey       string
	client       *http.Client
	consentGiven bool
}

// NewExternalProvider builds a provider with a 30s HTTP timeout.
func NewExternalProvider(apiKey string, consentGiven bool) *ExternalProvider {
	return &ExternalProvider{
		apiKey:       apiKey,
		client:       &http.Client{Timeout: 30 * time.Second},
		consentGiven: consentGiven,
	}
}

type externalForecastBody struct {
	Points          []prediction.DataPoint `json:"points"`
	Horizon         int                    `json:"horizon"`
	IntervalSeconds int64                  `json:"interval_seconds"`
}

// Forecast validates consent, records metrics, marshals the request, and stubs outbound HTTP.
func (p *ExternalProvider) Forecast(ctx context.Context, req prediction.ForecastRequest) ([]prediction.ForecastResult, error) {
	if err := privacy.ValidateConsent("external", p.consentGiven); err != nil {
		privacy.ConsentViolationsTotal.WithLabelValues("external").Inc()
		return nil, err
	}
	privacy.PredictionAPICallsTotal.WithLabelValues("external", "forecast").Inc()

	body := externalForecastBody{
		Points:          req.Points,
		Horizon:         req.Horizon,
		IntervalSeconds: int64(req.Interval / time.Second),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("external provider: marshal forecast request: %w", err)
	}
	_ = raw
	_ = ctx
	_ = p.client
	_ = p.apiKey
	// Real implementation would POST to a configured vendor URL and parse the response.
	return nil, errors.New("external API endpoint not configured")
}

// Train is unsupported for hosted external models.
func (p *ExternalProvider) Train(ctx context.Context, points []prediction.DataPoint) error {
	_ = ctx
	_ = points
	return errors.New("external provider does not support on-cluster training")
}

// DetectAnomalies is not exposed by the external stub.
func (p *ExternalProvider) DetectAnomalies(ctx context.Context, points []prediction.DataPoint) ([]prediction.AnomalyResult, error) {
	_ = ctx
	_ = points
	return nil, nil
}

// Explain returns consent and provider metadata only.
func (p *ExternalProvider) Explain(ctx context.Context, req prediction.ForecastRequest) (map[string]interface{}, error) {
	_ = ctx
	_ = req
	return map[string]interface{}{
		"provider":       "external",
		"consent_given": p.consentGiven,
	}, nil
}

// ProviderType implements PredictionProvider.
func (p *ExternalProvider) ProviderType() ProviderType {
	return ProviderExternal
}

// PrivacyLevel reports that user consent is required before exfiltration.
func (p *ExternalProvider) PrivacyLevel() PrivacyLevel {
	return ConsentRequired
}
