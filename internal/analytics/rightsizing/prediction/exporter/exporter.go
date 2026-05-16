// Package exporter exposes prediction forecast gauges on the controller-runtime /metrics endpoint.
package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction/provider"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	stateLabelKey   = "app.kubernetes.io/component"
	stateLabelValue = "rs-prediction-state"
	dataKeyStates   = "states"

	mForecastCPU            = "acm_rs:prediction_forecast_cpu"
	mForecastMemory         = "acm_rs:prediction_forecast_memory"
	mForecastWorkloadCPU    = "acm_rs:prediction_forecast_workload_cpu"
	mForecastWorkloadMemory = "acm_rs:prediction_forecast_workload_memory"
	mForecastGPUUtil        = "acm_rs:prediction_forecast_gpu_utilization"
	mForecastGPUMem         = "acm_rs:prediction_forecast_gpu_memory"
	mForecastVMCPU          = "acm_rs:prediction_forecast_vm_cpu"
	mForecastVMMemory       = "acm_rs:prediction_forecast_vm_memory"

	mAnomalyNS       = "acm_rs:prediction_anomaly_score"
	mAnomalyWorkload = "acm_rs:prediction_anomaly_score_workload"
	mAnomalyGPU      = "acm_rs:prediction_anomaly_score_gpu"
	mAnomalyVM       = "acm_rs:prediction_anomaly_score_vm"

	mAccuracy = "acm_rs:prediction_model_accuracy"
	mWeight   = "acm_rs:prediction_ensemble_weight"
)

// WorkloadKey identifies a series the same way as the training controller (cluster/namespace/workload/resource).
type WorkloadKey struct {
	Cluster   string
	Namespace string
	Workload  string
	Resource  string
}

// ModelState mirrors JSON written to rs-prediction-model-state-* ConfigMaps by the training controller.
type ModelState struct {
	Weights       map[string]float64     `json:"weights"`
	LastMAPE      float64                `json:"lastMAPE"`
	LastTrainedAt time.Time              `json:"lastTrainedAt"`
	Config        prediction.ModelConfig `json:"config"`
}

// DataPointSeries is one metric line from Thanos query_range.
type DataPointSeries struct {
	Key    WorkloadKey
	Points []prediction.DataPoint
}

// Querier matches training.Querier; use an adapter from the training package to avoid an import cycle.
type Querier interface {
	Query(ctx context.Context, promQL string, start, end time.Time, step time.Duration) ([]DataPointSeries, error)
}

// Options configures ForecastExporter.
type Options struct {
	Client        client.Client
	Querier       Querier
	Namespace     string
	IntervalHours int
	HistoryDays   int
	ProviderType  string
	Logger        logr.Logger
}

// ForecastExporter implements prometheus.Collector and re-computes short-horizon forecasts
// from recently queried usage series and per-key model config persisted in ConfigMaps.
type ForecastExporter struct {
	opts Options

	mu        sync.Mutex
	profile   string
	lookback  time.Duration
	descCache map[string]*prometheus.Desc
}

// NewForecastExporter builds a collector registered on controller-runtime metrics.Registry.
func NewForecastExporter(opts Options) *ForecastExporter {
	if opts.Logger.GetSink() == nil {
		opts.Logger = logr.Discard()
	}
	if opts.IntervalHours <= 0 {
		opts.IntervalHours = 1
	}
	if opts.HistoryDays <= 0 {
		opts.HistoryDays = 7
	}
	lbH := opts.HistoryDays * 24
	if lbH > 72 {
		lbH = 72
	}
	return &ForecastExporter{
		opts:      opts,
		profile:   rightsizing.RecommendationProfiles[0].Name,
		lookback:  time.Duration(lbH) * time.Hour,
		descCache: make(map[string]*prometheus.Desc),
	}
}

func (e *ForecastExporter) desc(fqName, help string, labelNames []string) *prometheus.Desc {
	k := fqName + "\x00" + strings.Join(labelNames, ",")
	if d, ok := e.descCache[k]; ok {
		return d
	}
	d := prometheus.NewDesc(fqName, help, labelNames, nil)
	e.descCache[k] = d
	return d
}

// Describe implements prometheus.Collector.
func (e *ForecastExporter) Describe(ch chan<- *prometheus.Desc) {
	ns2 := []string{"cluster", "namespace"}
	ns3 := []string{"cluster", "namespace", "workload"}
	ns2res := []string{"cluster", "namespace", "resource"}
	ns3res := []string{"cluster", "namespace", "workload", "resource"}
	ns4 := []string{"cluster", "namespace", "workload", "model", "resource"}
	for _, x := range []struct {
		name   string
		help   string
		labels []string
	}{
		{mForecastCPU, "Forecast namespace-level CPU usage.", ns2},
		{mForecastMemory, "Forecast namespace-level memory usage.", ns2},
		{mForecastWorkloadCPU, "Forecast workload CPU usage.", ns3},
		{mForecastWorkloadMemory, "Forecast workload memory usage.", ns3},
		{mForecastGPUUtil, "Forecast GPU utilization (namespace).", ns2},
		{mForecastGPUMem, "Forecast GPU memory used (namespace).", ns2},
		{mForecastVMCPU, "Forecast VM CPU usage.", ns3},
		{mForecastVMMemory, "Forecast VM memory usage.", ns3},
		{mAnomalyNS, "Namespace / pod-resource anomaly score from component disagreement.", ns2res},
		{mAnomalyWorkload, "Workload anomaly score from component disagreement.", ns3res},
		{mAnomalyGPU, "GPU namespace anomaly score from component disagreement.", ns2res},
		{mAnomalyVM, "Virtualization (VM) anomaly score from component disagreement.", ns3res},
		{mAccuracy, "Last training MAPE (ensemble).", ns4},
		{mWeight, "Ensemble member weight snapshot from training.", ns4},
	} {
		ch <- e.desc(x.name, x.help, x.labels)
	}
}

// Collect implements prometheus.Collector.
func (e *ForecastExporter) Collect(ch chan<- prometheus.Metric) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	states, err := e.loadStates(ctx)
	if err != nil {
		e.opts.Logger.Error(err, "forecast exporter: list model state configmaps")
		return
	}

	// Consider forecasts stale if training has not refreshed within this window. Use at least 6h
	// so short trainingIntervalHours (e.g. 1h) does not blank dashboards whenever a scrape misses a cycle.
	staleHours := 2 * e.opts.IntervalHours
	if staleHours < 6 {
		staleHours = 6
	}
	staleAfter := time.Duration(staleHours) * time.Hour
	now := time.Now().UTC()

	for key, st := range states {
		k, perr := parseWorkloadKey(key)
		if perr != nil {
			e.opts.Logger.V(1).Info("forecast exporter: skip invalid key", "key", key, "err", perr)
			continue
		}

		stale := now.Sub(st.LastTrainedAt) > staleAfter

		baseMetric, kind, ferr := e.resolveTrainingMetric(ctx, k)
		if ferr != nil {
			e.opts.Logger.V(1).Info("forecast exporter: cannot map key to metric", "key", key, "err", ferr)
			continue
		}

		promql := e.promQLWithKey(baseMetric, k, kind)
		points, qerr := e.queryPoints(ctx, promql)
		if qerr != nil {
			e.opts.Logger.V(1).Info("forecast exporter: query failed", "key", key, "err", qerr)
			continue
		}
		if len(points) < 5 {
			e.opts.Logger.V(1).Info("forecast exporter: insufficient points", "key", key, "n", len(points))
			continue
		}

		forecastVal, anomaly, cerr := e.computeForecastAndAnomaly(st, points)
		if cerr != nil {
			e.opts.Logger.V(1).Info("forecast exporter: forecast failed", "key", key, "err", cerr)
			continue
		}
		if stale {
			forecastVal = math.NaN()
		}

		e.emitForKind(ch, kind, k, forecastVal, anomaly)

		// Accuracy (MAPE) — ensemble row
		accDesc := e.desc(mAccuracy, "Last training MAPE (ensemble).", []string{"cluster", "namespace", "workload", "model", "resource"})
		ch <- prometheus.MustNewConstMetric(accDesc, prometheus.GaugeValue, st.LastMAPE,
			k.Cluster, k.Namespace, k.Workload, "ensemble", k.Resource)

		for modelName, w := range st.Weights {
			wDesc := e.desc(mWeight, "Ensemble member weight snapshot from training.", []string{"cluster", "namespace", "workload", "model", "resource"})
			ch <- prometheus.MustNewConstMetric(wDesc, prometheus.GaugeValue, w,
				k.Cluster, k.Namespace, k.Workload, modelName, k.Resource)
		}
	}
}

func (e *ForecastExporter) loadStates(ctx context.Context) (map[string]ModelState, error) {
	cmList := &corev1.ConfigMapList{}
	if err := e.opts.Client.List(ctx, cmList,
		client.InNamespace(e.opts.Namespace),
		client.MatchingLabels{stateLabelKey: stateLabelValue},
	); err != nil {
		return nil, err
	}
	merged := make(map[string]ModelState)
	for _, cm := range cmList.Items {
		if !strings.HasPrefix(cm.Name, "rs-prediction-model-state-") {
			continue
		}
		raw, ok := cm.Data[dataKeyStates]
		if !ok || raw == "" {
			continue
		}
		var partial map[string]ModelState
		if err := json.Unmarshal([]byte(raw), &partial); err != nil {
			return nil, fmt.Errorf("configmap %s/%s: %w", cm.Namespace, cm.Name, err)
		}
		for k, v := range partial {
			merged[k] = v
		}
	}
	return merged, nil
}

func parseWorkloadKey(key string) (WorkloadKey, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		return WorkloadKey{}, fmt.Errorf("expected cluster/namespace/workload/resource, got %d segments", len(parts))
	}
	return WorkloadKey{
		Cluster:   parts[0],
		Namespace: parts[1],
		Workload:  parts[2],
		Resource:  parts[3],
	}, nil
}

type seriesKind int

const (
	kindNamespace seriesKind = iota
	kindWorkload
	kindVMNamespace
	kindVMWorkload
	kindGPU
)

func (e *ForecastExporter) resolveTrainingMetric(ctx context.Context, k WorkloadKey) (baseMetric string, kind seriesKind, err error) {
	r := k.Resource
	switch r {
	case "gpu_utilization":
		return "acm_rs:namespace:gpu_usage", kindGPU, nil
	case "gpu_memory":
		return "acm_rs:namespace:gpu_memory_used", kindGPU, nil
	case "cpu", "memory":
		// continue
	default:
		return "", 0, fmt.Errorf("unknown resource %q", r)
	}

	metricSuffix := "cpu_usage"
	if r == "memory" {
		metricSuffix = "memory_usage"
	}

	if k.Workload == "" {
		nsQ := e.promQLForRecord("acm_rs:namespace:"+metricSuffix, k, "", "")
		if n, qerr := e.countSeries(ctx, nsQ); qerr == nil && n > 0 {
			return "acm_rs:namespace:" + metricSuffix, kindNamespace, nil
		}
		vmQ := e.promQLForRecord("acm_rs_vm:namespace:"+metricSuffix, k, "", "")
		if n, qerr := e.countSeries(ctx, vmQ); qerr == nil && n > 0 {
			return "acm_rs_vm:namespace:" + metricSuffix, kindVMNamespace, nil
		}
		return "", 0, fmt.Errorf("no namespace/vm namespace series for %s/%s", k.Cluster, k.Namespace)
	}

	wQ := e.promQLForRecord("acm_rs:workload:"+metricSuffix, k, "workload", k.Workload)
	if n, qerr := e.countSeries(ctx, wQ); qerr == nil && n > 0 {
		return "acm_rs:workload:" + metricSuffix, kindWorkload, nil
	}
	vmWQ := e.promQLForRecord("acm_rs_vm:namespace:"+metricSuffix, k, "name", k.Workload)
	if n, qerr := e.countSeries(ctx, vmWQ); qerr == nil && n > 0 {
		return "acm_rs_vm:namespace:" + metricSuffix, kindVMWorkload, nil
	}
	return "", 0, fmt.Errorf("no workload/vm series for %+v", k)
}

func (e *ForecastExporter) countSeries(ctx context.Context, selector string) (int, error) {
	end := time.Now().Truncate(time.Minute)
	start := end.Add(-2 * time.Hour)
	sel, err := e.opts.Querier.Query(ctx, selector, start, end, time.Hour)
	if err != nil {
		return 0, err
	}
	return len(sel), nil
}

func (e *ForecastExporter) promQLWithKey(record string, k WorkloadKey, kind seriesKind) string {
	switch kind {
	case kindNamespace, kindGPU:
		return e.promQLForRecord(record, k, "", "")
	case kindVMNamespace:
		return e.promQLForRecord(record, k, "", "")
	case kindWorkload:
		return e.promQLForRecord(record, k, "workload", k.Workload)
	case kindVMWorkload:
		return e.promQLForRecord(record, k, "name", k.Workload)
	default:
		return e.promQLForRecord(record, k, "", "")
	}
}

func (e *ForecastExporter) promQLForRecord(record string, k WorkloadKey, extraLabel, extraValue string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `%s{profile=%q,aggregation="1d",cluster=%q,namespace=%q`,
		record, e.profile, k.Cluster, k.Namespace)
	if extraLabel != "" && extraValue != "" {
		fmt.Fprintf(&b, `,%s=%q`, extraLabel, extraValue)
	}
	b.WriteString(`}`)
	return b.String()
}

func (e *ForecastExporter) queryPoints(ctx context.Context, promql string) ([]prediction.DataPoint, error) {
	end := time.Now().Truncate(time.Minute)
	start := end.Add(-e.lookback)
	series, err := e.opts.Querier.Query(ctx, promql, start, end, time.Hour)
	if err != nil {
		return nil, err
	}
	if len(series) == 0 {
		return nil, nil
	}
	if len(series) == 1 {
		return series[0].Points, nil
	}
	return mergePointsByTimestamp(series), nil
}

func mergePointsByTimestamp(series []DataPointSeries) []prediction.DataPoint {
	type bucket struct {
		ts time.Time
		sum float64
		n   int
	}
	m := make(map[int64]bucket)
	for _, s := range series {
		for _, p := range s.Points {
			sec := p.Timestamp.Unix()
			b := m[sec]
			b.ts = p.Timestamp
			b.sum += p.Value
			b.n++
			m[sec] = b
		}
	}
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]prediction.DataPoint, 0, len(keys))
	for _, k := range keys {
		b := m[k]
		if b.n == 0 {
			continue
		}
		out = append(out, prediction.DataPoint{Timestamp: b.ts, Value: b.sum})
	}
	return out
}

func (e *ForecastExporter) computeForecastAndAnomaly(st ModelState, points []prediction.DataPoint) (forecast float64, anomaly float64, err error) {
	if !isBuiltinProvider(e.opts.ProviderType) {
		return 0, 0, fmt.Errorf("forecast exporter supports builtin provider only, got %q", e.opts.ProviderType)
	}
	raw, jerr := json.Marshal(st.Config)
	if jerr != nil {
		return 0, 0, jerr
	}
	prov, perr := provider.Create(prediction.ProviderConfig{Type: string(provider.ProviderBuiltin), Config: raw})
	if perr != nil {
		return 0, 0, perr
	}
	if err := prov.Train(context.Background(), points); err != nil {
		return 0, 0, err
	}
	interval := time.Hour
	if len(points) >= 2 {
		interval = points[len(points)-1].Timestamp.Sub(points[len(points)-2].Timestamp)
		if interval <= 0 {
			interval = time.Hour
		}
	}
	fr, err := prov.Forecast(context.Background(), prediction.ForecastRequest{
		Points:   points,
		Horizon:  1,
		Interval: interval,
	})
	if err != nil || len(fr) < 1 {
		return 0, 0, fmt.Errorf("empty forecast: %v", err)
	}
	anomaly = normalizedComponentDisagreement(st.Config, points)
	return fr[0].PredictedValue, anomaly, nil
}

func isBuiltinProvider(pt string) bool {
	switch strings.ToLower(strings.TrimSpace(pt)) {
	case "", string(provider.ProviderBuiltin):
		return true
	default:
		return false
	}
}

func normalizedComponentDisagreement(cfg prediction.ModelConfig, points []prediction.DataPoint) float64 {
	if len(points) < 2 {
		return 0
	}
	hw := prediction.NewHoltWintersModel(cfg)
	_ = hw.Fit(points)
	hwOut := hw.Forecast(1)

	stl := prediction.NewSTLModel(cfg)
	_ = stl.Decompose(points)
	stlOut := stl.Forecast(points, 1)

	ar := prediction.NewARModel(cfg)
	_ = ar.Fit(points)
	arOut := ar.Forecast(1)

	if len(hwOut) == 0 || len(stlOut) == 0 || len(arOut) == 0 {
		return 0
	}
	a, b, c := hwOut[0].PredictedValue, stlOut[0].PredictedValue, arOut[0].PredictedValue
	m := (a + b + c) / 3
	denom := math.Abs(m)
	if denom < 1e-9 {
		denom = 1e-9
	}
	v := ((a-m)*(a-m) + (b-m)*(b-m) + (c-m)*(c-m)) / 3
	return math.Min(1, math.Sqrt(v)/denom)
}

func (e *ForecastExporter) emitForKind(ch chan<- prometheus.Metric, kind seriesKind, k WorkloadKey, forecast, anomaly float64) {
	nsLabels := []string{k.Cluster, k.Namespace}
	wlLabels := []string{k.Cluster, k.Namespace, k.Workload}

	switch kind {
	case kindNamespace:
		if k.Resource == "cpu" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastCPU, "Forecast CPU usage (namespace).", []string{"cluster", "namespace"}),
				prometheus.GaugeValue, forecast, nsLabels...)
		} else if k.Resource == "memory" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastMemory, "Forecast memory usage (namespace).", []string{"cluster", "namespace"}),
				prometheus.GaugeValue, forecast, nsLabels...)
		}
		ch <- prometheus.MustNewConstMetric(
			e.desc(mAnomalyNS, "Anomaly score (namespace CPU/memory path).", []string{"cluster", "namespace", "resource"}),
			prometheus.GaugeValue, anomaly, nsLabels[0], nsLabels[1], k.Resource)
	case kindWorkload:
		if k.Resource == "cpu" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastWorkloadCPU, "Forecast CPU (workload).", []string{"cluster", "namespace", "workload"}),
				prometheus.GaugeValue, forecast, wlLabels...)
		} else if k.Resource == "memory" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastWorkloadMemory, "Forecast memory (workload).", []string{"cluster", "namespace", "workload"}),
				prometheus.GaugeValue, forecast, wlLabels...)
		}
		ch <- prometheus.MustNewConstMetric(
			e.desc(mAnomalyWorkload, "Anomaly score (workload).", []string{"cluster", "namespace", "workload", "resource"}),
			prometheus.GaugeValue, anomaly, wlLabels[0], wlLabels[1], wlLabels[2], k.Resource)
	case kindVMNamespace, kindVMWorkload:
		vmLab := []string{k.Cluster, k.Namespace, k.Workload}
		if k.Resource == "cpu" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastVMCPU, "Forecast VM CPU usage.", []string{"cluster", "namespace", "workload"}),
				prometheus.GaugeValue, forecast, vmLab...)
		} else if k.Resource == "memory" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastVMMemory, "Forecast VM memory usage.", []string{"cluster", "namespace", "workload"}),
				prometheus.GaugeValue, forecast, vmLab...)
		}
		ch <- prometheus.MustNewConstMetric(
			e.desc(mAnomalyVM, "Virtualization (VM) anomaly score from component disagreement.", []string{"cluster", "namespace", "workload", "resource"}),
			prometheus.GaugeValue, anomaly, vmLab[0], vmLab[1], vmLab[2], k.Resource)
	case kindGPU:
		if k.Resource == "gpu_utilization" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastGPUUtil, "Forecast GPU utilization.", []string{"cluster", "namespace"}),
				prometheus.GaugeValue, forecast, nsLabels...)
		} else if k.Resource == "gpu_memory" {
			ch <- prometheus.MustNewConstMetric(
				e.desc(mForecastGPUMem, "Forecast GPU memory used.", []string{"cluster", "namespace"}),
				prometheus.GaugeValue, forecast, nsLabels...)
		}
		ch <- prometheus.MustNewConstMetric(
			e.desc(mAnomalyGPU, "Anomaly score (GPU namespace).", []string{"cluster", "namespace", "resource"}),
			prometheus.GaugeValue, anomaly, nsLabels[0], nsLabels[1], k.Resource)
	}
}
