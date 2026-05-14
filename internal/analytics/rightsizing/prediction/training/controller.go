package training

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction/provider"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	stateConfigMapLabelKey   = "app.kubernetes.io/component"
	stateConfigMapLabelValue = "rs-prediction-state"
	stateConfigMapPrefix     = "rs-prediction-model-state-"
	configMapDataKeyStates   = "states"
	maxShardBytes            = 1024 * 1024
)

var errWorkloadStateTooLarge = errors.New("single workload state exceeds maximum serialized size")

// ModelState is persisted to ConfigMaps for reuse across controller restarts.
type ModelState struct {
	Weights       map[string]float64     `json:"weights"`
	LastMAPE      float64                `json:"lastMAPE"`
	LastTrainedAt time.Time              `json:"lastTrainedAt"`
	Config        prediction.ModelConfig `json:"config"`
}

// Controller periodically retrains prediction providers and persists workload model state.
type Controller struct {
	config    TrainingConfig
	querier   Querier
	client    client.Client
	namespace string

	states map[string]*ModelState
	mu     sync.RWMutex
}

// NewController constructs a training controller.
func NewController(cfg TrainingConfig, querier Querier, cl client.Client, namespace string) *Controller {
	return &Controller{
		config:    cfg,
		querier:   querier,
		client:    cl,
		namespace: namespace,
		states:    make(map[string]*ModelState),
	}
}

// Start restores state, runs a training loop until ctx is canceled, then returns ctx.Err().
func (c *Controller) Start(ctx context.Context) error {
	if err := c.restoreState(ctx); err != nil {
		log.Printf("training: restore state: %v", err)
	}

	runSafe := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("training: cycle panic: %v", r)
			}
		}()
		if err := c.runTrainingCycle(ctx); err != nil {
			log.Printf("training: cycle error: %v", err)
		}
	}

	runSafe()

	ticker := time.NewTicker(time.Duration(c.config.IntervalHours) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			runSafe()
		}
	}
}

func (c *Controller) runTrainingCycle(ctx context.Context) error {
	end := time.Now().Truncate(time.Minute)
	start := end.Add(-time.Duration(c.config.HistoryDays) * 24 * time.Hour)
	step := time.Hour

	type metricQuery struct {
		query string
		label string
	}

	var queries []metricQuery

	if c.config.NamespaceEnabled {
		queries = append(queries,
			metricQuery{"acm_rs:namespace:cpu_usage:5m", "namespace_cpu"},
			metricQuery{"acm_rs:namespace:memory_usage:5m", "namespace_memory"},
		)
	}
	if c.config.WorkloadEnabled {
		queries = append(queries,
			metricQuery{"acm_rs:workload:cpu_usage:5m", "workload_cpu"},
			metricQuery{"acm_rs:workload:memory_usage:5m", "workload_memory"},
		)
	}
	if c.config.GPUEnabled {
		queries = append(queries,
			metricQuery{"acm_rs:namespace:gpu_usage:5m", "gpu_utilization"},
			metricQuery{"acm_rs:namespace:gpu_memory_used:5m", "gpu_memory"},
		)
	}
	if c.config.VMEnabled {
		queries = append(queries,
			metricQuery{"acm_rs_vm:namespace:cpu_usage:5m", "vm_cpu"},
			metricQuery{"acm_rs_vm:namespace:memory_usage:5m", "vm_memory"},
		)
	}

	for _, q := range queries {
		series, err := c.querier.Query(ctx, q.query, start, end, step)
		if err != nil {
			log.Printf("training: %s query: %v", q.label, err)
			continue
		}
		for _, s := range series {
			c.trainOneSeries(ctx, s)
		}
	}

	if err := c.persistState(ctx); err != nil {
		return fmt.Errorf("persist state: %w", err)
	}
	return nil
}

func (c *Controller) trainOneSeries(ctx context.Context, series DataPointSeries) {
	if len(series.Points) < 5 {
		return
	}

	key := series.Key.String()
	var oldMAPE float64
	c.mu.RLock()
	if st := c.states[key]; st != nil {
		oldMAPE = st.LastMAPE
	}
	c.mu.RUnlock()

	prov, err := provider.Create(providerConfigFromTraining(c.config))
	if err != nil {
		log.Printf("training: provider %s: %v", key, err)
		return
	}

	pts := series.Points
	n := len(pts)
	split := (n * 8) / 10
	if split < 2 || split >= n {
		return
	}
	trainPts := pts[:split]
	testPts := pts[split:]

	if trainErr := prov.Train(ctx, trainPts); trainErr != nil {
		log.Printf("training: Train %s: %v", key, trainErr)
		return
	}

	interval := time.Hour
	if len(trainPts) >= 2 {
		interval = trainPts[len(trainPts)-1].Timestamp.Sub(trainPts[len(trainPts)-2].Timestamp)
		if interval <= 0 {
			interval = time.Hour
		}
	}

	fr, err := prov.Forecast(ctx, prediction.ForecastRequest{
		Points:   trainPts,
		Horizon:  len(testPts),
		Interval: interval,
	})
	if err != nil || len(fr) < len(testPts) {
		log.Printf("training: Forecast %s: err=%v len=%d want=%d", key, err, len(fr), len(testPts))
		return
	}

	predPts := make([]prediction.DataPoint, len(testPts))
	for i := range testPts {
		predPts[i] = prediction.DataPoint{
			Timestamp: testPts[i].Timestamp,
			Value:     fr[i].PredictedValue,
		}
	}
	newMAPE := ComputeMAPE(testPts, predPts)

	if !ValidateTrainingResult(oldMAPE, newMAPE) {
		return
	}

	weights := map[string]float64{}
	if ex, err := prov.Explain(ctx, prediction.ForecastRequest{Points: trainPts, Horizon: 1, Interval: interval}); err == nil {
		if w, ok := ex["weights"].(map[string]float64); ok {
			weights = w
		}
	}

	ns := &ModelState{
		Weights:       weights,
		LastMAPE:      newMAPE,
		LastTrainedAt: time.Now().UTC(),
		Config:        c.config.ModelConfig,
	}

	c.mu.Lock()
	c.states[key] = ns
	c.mu.Unlock()
}

// GetState returns current state for a workload key.
func (c *Controller) GetState(key WorkloadKey) (*ModelState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st, ok := c.states[key.String()]
	return st, ok
}

func (c *Controller) restoreState(ctx context.Context) error {
	cmList := &corev1.ConfigMapList{}
	if err := c.client.List(ctx, cmList,
		client.InNamespace(c.namespace),
		client.MatchingLabels{stateConfigMapLabelKey: stateConfigMapLabelValue},
	); err != nil {
		return fmt.Errorf("list state configmaps: %w", err)
	}

	merged := make(map[string]ModelState)
	for _, cm := range cmList.Items {
		raw, ok := cm.Data[configMapDataKeyStates]
		if !ok || raw == "" {
			continue
		}
		var partial map[string]ModelState
		if err := json.Unmarshal([]byte(raw), &partial); err != nil {
			return fmt.Errorf("configmap %s/%s: unmarshal states: %w", cm.Namespace, cm.Name, err)
		}
		maps.Copy(merged, partial)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range merged {
		c.states[k] = &v
	}
	return nil
}

func (c *Controller) persistState(ctx context.Context) error {
	c.mu.RLock()
	snapshot := make(map[string]ModelState, len(c.states))
	for k, st := range c.states {
		if st == nil {
			continue
		}
		snapshot[k] = *st
	}
	c.mu.RUnlock()

	shards, err := splitStateShards(snapshot, maxShardBytes)
	if err != nil {
		return fmt.Errorf("shard states: %w", err)
	}

	for i, shard := range shards {
		name := stateConfigMapPrefix + strconv.Itoa(i)
		payload, err := json.Marshal(shard)
		if err != nil {
			return fmt.Errorf("marshal shard %d: %w", i, err)
		}

		cmKey := types.NamespacedName{Namespace: c.namespace, Name: name}
		existing := &corev1.ConfigMap{}
		err = c.client.Get(ctx, cmKey, existing)
		if apierrors.IsNotFound(err) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: c.namespace,
					Labels: map[string]string{
						stateConfigMapLabelKey: stateConfigMapLabelValue,
					},
				},
				Data: map[string]string{
					configMapDataKeyStates: string(payload),
				},
			}
			if createErr := c.client.Create(ctx, cm); createErr != nil {
				return fmt.Errorf("create %s: %w", name, createErr)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("get %s: %w", name, err)
		}

		if existing.Data == nil {
			existing.Data = make(map[string]string)
		}
		existing.Data[configMapDataKeyStates] = string(payload)
		existing.Labels = ensureComponentLabel(existing.Labels)

		if err := c.client.Update(ctx, existing); err != nil {
			return fmt.Errorf("update %s: %w", name, err)
		}
	}

	if err := c.deleteStaleShards(ctx, len(shards)); err != nil {
		return err
	}
	return nil
}

func ensureComponentLabel(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	if labels[stateConfigMapLabelKey] == "" {
		labels[stateConfigMapLabelKey] = stateConfigMapLabelValue
	}
	return labels
}

func splitStateShards(states map[string]ModelState, maxBytes int) ([]map[string]ModelState, error) {
	if len(states) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var shards []map[string]ModelState
	current := make(map[string]ModelState)

	flush := func() {
		if len(current) == 0 {
			return
		}
		shardCopy := make(map[string]ModelState, len(current))
		maps.Copy(shardCopy, current)
		shards = append(shards, shardCopy)
		for k := range current {
			delete(current, k)
		}
	}

	for _, k := range keys {
		v := states[k]
		try := make(map[string]ModelState, len(current)+1)
		maps.Copy(try, current)
		try[k] = v

		b, err := json.Marshal(try)
		if err != nil {
			return nil, err
		}
		if len(b) > maxBytes && len(current) > 0 {
			flush()
			try = map[string]ModelState{k: v}
			b, err = json.Marshal(try)
			if err != nil {
				return nil, err
			}
		}
		if len(b) > maxBytes && len(try) == 1 {
			return nil, fmt.Errorf("workload state for key %q exceeds %d bytes: %w", k, maxBytes, errWorkloadStateTooLarge)
		}
		maps.Copy(current, try)
	}
	flush()
	return shards, nil
}

func (c *Controller) deleteStaleShards(ctx context.Context, keep int) error {
	cmList := &corev1.ConfigMapList{}
	if err := c.client.List(ctx, cmList,
		client.InNamespace(c.namespace),
		client.MatchingLabels{stateConfigMapLabelKey: stateConfigMapLabelValue},
	); err != nil {
		return fmt.Errorf("list for shard cleanup: %w", err)
	}

	for _, cm := range cmList.Items {
		if !strings.HasPrefix(cm.Name, stateConfigMapPrefix) {
			continue
		}
		suffix := strings.TrimPrefix(cm.Name, stateConfigMapPrefix)
		idx, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if idx >= keep {
			if err := c.client.Delete(ctx, &cm); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("delete stale configmap %s: %w", cm.Name, err)
			}
		}
	}
	return nil
}

func providerConfigFromTraining(c TrainingConfig) prediction.ProviderConfig {
	pc := prediction.ProviderConfig{Type: c.ProviderType}
	switch strings.ToLower(strings.TrimSpace(c.ProviderType)) {
	case "", string(provider.ProviderBuiltin):
		raw, _ := json.Marshal(c.ModelConfig)
		pc.Config = raw
	case string(provider.ProviderONNX):
		pc.Config = nil
	case string(provider.ProviderExternal):
		raw, _ := json.Marshal(struct {
			APIKey       string `json:"apiKey"`
			ConsentGiven bool   `json:"consentGiven"`
		}{c.ExternalAPIKey, c.ConsentGiven})
		pc.Config = raw
	case string(provider.ProviderCustom):
		raw, _ := json.Marshal(struct {
			EndpointURL  string `json:"endpointURL"`
			ConsentGiven bool   `json:"consentGiven"`
		}{c.CustomEndpointURL, c.ConsentGiven})
		pc.Config = raw
	default:
		pc.Config = nil
	}
	return pc
}
