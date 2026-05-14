package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction/privacy"
)

// CustomProvider calls a user-defined HTTP prediction service inside or outside the cluster.
type CustomProvider struct {
	endpointURL  string
	client       *http.Client
	consentGiven bool
}

// NewCustomProvider builds a provider with a 30s HTTP client timeout.
func NewCustomProvider(endpointURL string, consentGiven bool) *CustomProvider {
	return &CustomProvider{
		endpointURL:  endpointURL,
		client:       &http.Client{Timeout: 30 * time.Second},
		consentGiven: consentGiven,
	}
}

// isClusterLocal reports whether the endpoint is addressed as an in-cluster Kubernetes DNS name.
func (p *CustomProvider) isClusterLocal() bool {
	return strings.Contains(p.endpointURL, "svc.cluster.local")
}

func (p *CustomProvider) ensureCustomConsent() error {
	if p.isClusterLocal() {
		return nil
	}
	if err := privacy.ValidateConsent("custom", p.consentGiven); err != nil {
		privacy.ConsentViolationsTotal.WithLabelValues("custom").Inc()
		return err
	}
	return nil
}

func (p *CustomProvider) joinPath(path string) string {
	base := strings.TrimSuffix(strings.TrimSpace(p.endpointURL), "/")
	return base + path
}

type customStdRequest struct {
	Points          []prediction.DataPoint `json:"points"`
	Horizon         int                    `json:"horizon"`
	IntervalSeconds int64                  `json:"interval_seconds"`
}

func customStdFromForecast(req prediction.ForecastRequest) customStdRequest {
	return customStdRequest{
		Points:          req.Points,
		Horizon:         req.Horizon,
		IntervalSeconds: int64(req.Interval / time.Second),
	}
}

type customForecastResponse struct {
	Predictions []struct {
		Value     float64 `json:"value"`
		Lower     float64 `json:"lower"`
		Upper     float64 `json:"upper"`
		Timestamp string  `json:"timestamp"`
	} `json:"predictions"`
}

type customTrainBody struct {
	Points []prediction.DataPoint `json:"points"`
}

type customDetectResponse struct {
	Anomalies []struct {
		Timestamp string  `json:"timestamp"`
		Score     float64 `json:"score"`
		Type      string  `json:"type"`
		Severity  string  `json:"severity"`
	} `json:"anomalies"`
}

func parseCustomTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func (p *CustomProvider) Forecast(ctx context.Context, req prediction.ForecastRequest) ([]prediction.ForecastResult, error) {
	if err := p.ensureCustomConsent(); err != nil {
		return nil, err
	}
	privacy.PredictionAPICallsTotal.WithLabelValues("custom", "forecast").Inc()

	raw, err := json.Marshal(customStdFromForecast(req))
	if err != nil {
		return nil, fmt.Errorf("custom provider: marshal forecast request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.joinPath("/v1/forecast"), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("custom provider forecast: HTTP %d", resp.StatusCode)
	}

	var payload customForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("custom provider: decode forecast response: %w", err)
	}

	out := make([]prediction.ForecastResult, 0, len(payload.Predictions))
	for _, pr := range payload.Predictions {
		ts, err := parseCustomTimestamp(pr.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("custom provider: parse prediction timestamp %q: %w", pr.Timestamp, err)
		}
		out = append(out, prediction.ForecastResult{
			PredictedValue:  pr.Value,
			ConfidenceLower: pr.Lower,
			ConfidenceUpper: pr.Upper,
			Timestamp:       ts,
		})
	}
	return out, nil
}

// Train POSTs historical points to the remote /v1/train endpoint.
func (p *CustomProvider) Train(ctx context.Context, points []prediction.DataPoint) error {
	if err := p.ensureCustomConsent(); err != nil {
		return err
	}

	raw, err := json.Marshal(customTrainBody{Points: points})
	if err != nil {
		return fmt.Errorf("custom provider: marshal train request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.joinPath("/v1/train"), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("custom provider train: HTTP %d", resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// DetectAnomalies POSTs points to /v1/detect and parses anomaly results.
func (p *CustomProvider) DetectAnomalies(ctx context.Context, points []prediction.DataPoint) ([]prediction.AnomalyResult, error) {
	if err := p.ensureCustomConsent(); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(customTrainBody{Points: points})
	if err != nil {
		return nil, fmt.Errorf("custom provider: marshal detect request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.joinPath("/v1/detect"), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("custom provider detect: HTTP %d", resp.StatusCode)
	}

	var payload customDetectResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("custom provider: decode detect response: %w", err)
	}

	out := make([]prediction.AnomalyResult, 0, len(payload.Anomalies))
	for _, a := range payload.Anomalies {
		ts, err := parseCustomTimestamp(a.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("custom provider: parse anomaly timestamp %q: %w", a.Timestamp, err)
		}
		out = append(out, prediction.AnomalyResult{
			Timestamp: ts,
			Score:     a.Score,
			Type:      a.Type,
			Severity:  a.Severity,
		})
	}
	return out, nil
}

// Explain POSTs to /v1/explain and returns the JSON object as a map.
func (p *CustomProvider) Explain(ctx context.Context, req prediction.ForecastRequest) (map[string]interface{}, error) {
	if err := p.ensureCustomConsent(); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(customStdFromForecast(req))
	if err != nil {
		return nil, fmt.Errorf("custom provider: marshal explain request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.joinPath("/v1/explain"), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("custom provider explain: HTTP %d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	var out map[string]interface{}
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("custom provider: decode explain response: %w", err)
	}
	return out, nil
}

// ProviderType implements PredictionProvider.
func (p *CustomProvider) ProviderType() ProviderType {
	return ProviderCustom
}

// PrivacyLevel requires consent when the endpoint is not cluster-local.
func (p *CustomProvider) PrivacyLevel() PrivacyLevel {
	if p.isClusterLocal() {
		return NoExfiltration
	}
	return ConsentRequired
}
