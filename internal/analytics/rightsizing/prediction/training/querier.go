package training

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

var (
	errThanosHTTPStatus = errors.New("thanos query_range: non-OK HTTP response")
	errThanosAPIError   = errors.New("thanos query_range: API returned error")
	errPromPairFormat   = errors.New("expected [timestamp, value] sample pair")
	errJSONFloatType    = errors.New("unsupported JSON value type for float conversion")
)

// Querier loads Prometheus range vectors (e.g. from Thanos Querier).
type Querier interface {
	Query(ctx context.Context, promQL string, start, end time.Time, step time.Duration) ([]DataPointSeries, error)
}

// DataPointSeries is one metric line with a workload identity and samples.
type DataPointSeries struct {
	Key    WorkloadKey
	Points []prediction.DataPoint
}

// ThanosQuerier calls Prometheus-compatible /api/v1/query_range against Thanos.
type ThanosQuerier struct {
	thanosURL string
	client    *http.Client
}

// NewThanosQuerier builds a querier. thanosURL should be the base URL of the query frontend.
func NewThanosQuerier(thanosURL string) *ThanosQuerier {
	return &ThanosQuerier{
		thanosURL: strings.TrimRight(thanosURL, "/"),
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

// Query implements Querier.
func (t *ThanosQuerier) Query(ctx context.Context, promQL string, start, end time.Time, step time.Duration) ([]DataPointSeries, error) {
	u, err := url.Parse(t.thanosURL + "/api/v1/query_range")
	if err != nil {
		return nil, fmt.Errorf("thanos query_range: bad base url: %w", err)
	}
	q := u.Query()
	q.Set("query", promQL)
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("step", formatPrometheusStep(step))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("thanos query_range: build request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("thanos query_range: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("thanos query_range: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d: %s", errThanosHTTPStatus, resp.StatusCode, truncateForErr(body, 512))
	}

	var payload thanosQueryRangeResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("thanos query_range: json: %w", err)
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("%w: %s: %s", errThanosAPIError, payload.ErrorType, payload.Error)
	}

	out := make([]DataPointSeries, 0, len(payload.Data.Result))
	hint := resourceHintFromQuery(promQL)
	for _, r := range payload.Data.Result {
		key := workloadKeyFromMetric(r.Metric, hint)
		points, perr := valuesToDataPoints(r.Values)
		if perr != nil {
			return nil, fmt.Errorf("thanos query_range: series %v: %w", r.Metric, perr)
		}
		out = append(out, DataPointSeries{Key: key, Points: points})
	}
	return out, nil
}

type thanosQueryRangeResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func formatPrometheusStep(d time.Duration) string {
	if d <= 0 {
		d = time.Minute
	}
	sec := int64(d / time.Second)
	if sec < 1 {
		sec = 1
	}
	return fmt.Sprintf("%ds", sec)
}

func resourceHintFromQuery(promQL string) string {
	s := strings.ToLower(promQL)
	if strings.Contains(s, "gpu_memory") {
		return "gpu_memory"
	}
	if strings.Contains(s, "gpu_usage") {
		return "gpu_utilization"
	}
	if strings.Contains(s, "namespace_memory") || strings.Contains(s, "memory_usage") {
		return "memory"
	}
	if strings.Contains(s, "namespace_cpu") || strings.Contains(s, "cpu_usage") {
		return "cpu"
	}
	return ""
}

func workloadKeyFromMetric(m map[string]string, resourceHint string) WorkloadKey {
	r := m["resource"]
	if r == "" {
		r = resourceHint
	}
	wk := m["workload"]
	if wk == "" {
		wk = m["name"]
	}
	return WorkloadKey{
		Cluster:   m["cluster"],
		Namespace: m["namespace"],
		Workload:  wk,
		Resource:  r,
	}
}

func valuesToDataPoints(values [][]interface{}) ([]prediction.DataPoint, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]prediction.DataPoint, 0, len(values))
	for i, pair := range values {
		if len(pair) < 2 {
			return nil, fmt.Errorf("pair %d: %w", i, errPromPairFormat)
		}
		tsF, err := ifaceToFloat64(pair[0])
		if err != nil {
			return nil, fmt.Errorf("pair %d ts: %w", i, err)
		}
		vF, err := ifaceToFloat64(pair[1])
		if err != nil {
			return nil, fmt.Errorf("pair %d value: %w", i, err)
		}
		sec, frac := math.Modf(tsF)
		t := time.Unix(int64(sec), int64(frac*1e9))
		out = append(out, prediction.DataPoint{Timestamp: t, Value: vF})
	}
	return out, nil
}

func ifaceToFloat64(v interface{}) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case json.Number:
		return x.Float64()
	case string:
		return strconv.ParseFloat(x, 64)
	default:
		return 0, fmt.Errorf("%T: %w", v, errJSONFloatType)
	}
}

func truncateForErr(b []byte, max int) string {
	s := string(b)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
