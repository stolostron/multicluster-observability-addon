# MCOA Tasks: Pluggable Prediction Engine

> Phase 2 output — 17 tasks across 3 sequential batches + integration layer
> Approved decisions: flat sibling API, acm_rs:prediction_* naming, independent delegation

---

## Batch 1 — Core Models (Tasks 1-7)

### Task 1: Prediction types + provider interface

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: none

**Files to Create:**
- `internal/analytics/rightsizing/prediction/types.go` — ForecastRequest, ForecastResult, DataPoint, ModelConfig, confidence interval types
- `internal/analytics/rightsizing/prediction/provider/interface.go` — PredictionProvider interface (Forecast, Train, DetectAnomalies, Explain, ProviderType, PrivacyLevel), ProviderType enum (Builtin/ONNX/External/Custom), PrivacyLevel enum (NoExfiltration/ConsentRequired)
- `internal/analytics/rightsizing/prediction/provider/registry.go` — ProviderRegistry factory: Create(config) -> PredictionProvider; reads ADC-derived config

**Implementation Notes:**
- Follow existing type patterns in `internal/analytics/rightsizing/types.go` (RSPrometheusRuleConfig, RSConfigMapData)
- Provider interface is the contract all 4 providers implement — keep it minimal
- Registry reads provider type string from ADC config JSON

**Acceptance Criteria:**
- [ ] types.go compiles with zero external deps beyond stdlib
- [ ] interface.go defines PredictionProvider with 6 methods
- [ ] registry.go resolves "builtin" to a stub (real builtin in Task 7)
- [ ] `go test ./internal/analytics/rightsizing/prediction/...` passes

**Test Requirements:**
- [ ] Unit test: registry.go returns correct provider type for each config string
- [ ] Unit test: unknown provider type returns error

---

### Task 2: Feature engineering pipeline

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: L
- **Depends on**: Task 1 (types.go)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/features/types.go` — FeatureVector struct, FeatureConfig, ExtractFeatures() top-level function
- `internal/analytics/rightsizing/prediction/features/temporal.go` — HourOfDay, DayOfWeek, IsBusinessHours, IsWeekend, WeekOfMonth
- `internal/analytics/rightsizing/prediction/features/statistical.go` — RollingMean, StdDev, Median, P95, P99, Skewness, Kurtosis, CoV
- `internal/analytics/rightsizing/prediction/features/trend.go` — LinearSlope, Acceleration, ChangePointScore
- `internal/analytics/rightsizing/prediction/features/workload.go` — BurstFrequency, BurstMagnitude, IdleRatio, UtilizationEfficiency
- `internal/analytics/rightsizing/prediction/features/correlation.go` — CPUMemoryCorrelation (Pearson)

**Implementation Notes:**
- Pure Go math — no external ML libraries
- All extractors take []DataPoint and return named float64 fields on FeatureVector
- Window sizes configurable via FeatureConfig

**Acceptance Criteria:**
- [ ] ExtractFeatures on a synthetic 24h series returns all temporal fields correctly
- [ ] Statistical features match expected values for known distributions
- [ ] Zero-length input returns zero-valued FeatureVector without panic

**Test Requirements:**
- [ ] Table-driven tests per extractor with known input/output pairs
- [ ] Edge cases: empty series, single point, constant series

---

### Task 3: Holt-Winters triple smoothing

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: S
- **Depends on**: Task 1 (types.go)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/holt_winters.go` — HoltWintersModel: Forecast(points, horizon) -> []ForecastResult, Update/Predict for incremental, seasonal period auto-detection (daily=288, weekly=2016 at 5m intervals)

**Implementation Notes:**
- Alpha=0.2 (level), Beta=0.1 (trend), Gamma=0.05 (seasonal) as defaults
- Seasonal period auto-detected from data frequency
- Pure Go — no external deps

**Acceptance Criteria:**
- [ ] Forecast on a sinusoidal series produces values within 10% MAPE
- [ ] Seasonal period correctly detected for 5m-interval data
- [ ] Short series (< 2 periods) falls back to simple exponential smoothing

**Test Requirements:**
- [ ] Synthetic daily-pattern series -> forecast accuracy check
- [ ] Edge: series shorter than one period

---

### Task 4: STL decomposition model

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Task 1 (types.go)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/stl.go` — STLModel: Decompose(points) -> (Trend, Seasonal, Residual), Forecast(points, horizon) -> []ForecastResult. LOESS fitting, 3 iterations default.

**Implementation Notes:**
- LOESS = locally weighted scatterplot smoothing, pure Go implementation
- Forecast = extrapolated trend + repeated seasonal component
- 3 iterations balances accuracy vs CPU

**Acceptance Criteria:**
- [ ] Decompose recovers known trend + seasonal from synthetic data
- [ ] Residual has near-zero mean for clean periodic input
- [ ] Forecast extends trend correctly beyond training window

**Test Requirements:**
- [ ] Known synthetic series decomposition validation
- [ ] LOESS accuracy on linear + quadratic test cases

---

### Task 5: Autoregressive AR(p) model

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Task 1 (types.go)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/autoregressive.go` — ARModel: Fit(points) selects order p via BIC over p=1..10, Forecast(horizon) -> []ForecastResult. Yule-Walker equations for coefficient estimation.

**Implementation Notes:**
- Yule-Walker: solve Toeplitz system from autocorrelation estimates
- Order selection: minimize BIC = n*ln(sigma^2) + p*ln(n)
- Pure linear algebra — no external deps

**Acceptance Criteria:**
- [ ] AR(1) with known coefficient recovers it within 5%
- [ ] BIC correctly selects p=1 for AR(1) generated data
- [ ] Forecast on stationary series stays within confidence bounds

**Test Requirements:**
- [ ] AR(1) coefficient recovery test
- [ ] Order selection correctness on AR(2) data

---

### Task 6: Ensemble combiner + backtest

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Tasks 3, 4, 5 (all three models)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/backtest.go` — Backtest(): 80/20 train/test split, computes MAPE/RMSE per model, returns weight updates only if improvement > 5%
- `internal/analytics/rightsizing/prediction/ensemble.go` — EnsembleForecaster: Forecast() runs all 3 models, combines by inverse-MAPE weights, produces ForecastResult with ConfidenceLower/Upper (90% CI), DominantModel, FeatureImportance

**Implementation Notes:**
- weight_i = (1/MAPE_i) / sum(1/MAPE_j)
- Confidence interval from weighted variance of model predictions
- DominantModel = model with highest weight

**Acceptance Criteria:**
- [ ] Ensemble on daily-pattern data outperforms worst individual model
- [ ] Backtest with < 5% improvement does NOT update weights
- [ ] ConfidenceLower < PredictedValue < ConfidenceUpper

**Test Requirements:**
- [ ] Ensemble weight calculation with known MAPE values
- [ ] Backtest threshold gate (5%) validation
- [ ] CI sanity check

---

### Task 7: Built-in provider (wraps ensemble)

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: S
- **Depends on**: Tasks 1 (interface), 6 (ensemble)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/provider/builtin/provider.go` — BuiltinProvider implements PredictionProvider; wraps EnsembleForecaster; PrivacyLevel = NoExfiltration

**Implementation Notes:**
- Simplest provider — delegates all calls to ensemble
- No HTTP client, no external deps
- ProviderType() returns "builtin"

**Acceptance Criteria:**
- [ ] Implements all 6 PredictionProvider methods
- [ ] PrivacyLevel returns NoExfiltration
- [ ] Forecast returns same result as direct ensemble call

**Test Requirements:**
- [ ] Interface compliance test
- [ ] Forecast passthrough test

---

## Batch 2 — Intelligence + Safety (Tasks 8-11)

### Task 8: Anomaly detection suite

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Task 1 (types.go)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/anomaly/types.go` — AnomalyResult (Timestamp, Score, Type, Severity), DetectorConfig
- `internal/analytics/rightsizing/prediction/anomaly/zscore.go` — seasonal residual Z-score, threshold |Z|>3 default
- `internal/analytics/rightsizing/prediction/anomaly/rateofchange.go` — first derivative exceeds historical tail
- `internal/analytics/rightsizing/prediction/anomaly/adaptive.go` — rolling percentile thresholds that adjust to data distribution
- `internal/analytics/rightsizing/prediction/anomaly/detector.go` — composite Detect() running all three, deduplicating overlapping alerts

**Acceptance Criteria:**
- [ ] Z-score detects spike injected at 5x normal variance
- [ ] Rate-of-change detects step function
- [ ] Adaptive threshold detects slow drift over 7 days
- [ ] Composite deduplicates overlapping detections

**Test Requirements:**
- [ ] Synthetic spike/step/drift series per detector
- [ ] Composite dedup logic test

---

### Task 9: Optimization recommender

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Tasks 6 (ensemble), 8 (anomaly)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/optimizer/types.go` — OptimizationResult (TargetCPU, TargetMemory, EstimatedSavings, Confidence), bounds config
- `internal/analytics/rightsizing/prediction/optimizer/recommender.go` — Recommend(): target = max(forecast*safety, upper_CI, current*headroom, p99*burstProtection). Respects existing RS bounds (50m-8 CPU, 64Mi-16Gi memory).
- `internal/analytics/rightsizing/prediction/optimizer/savings.go` — EstimateSavings from current vs recommended

**Implementation Notes:**
- Safety margin default 1.15 (15% headroom)
- Rate limits: down 30%, up 100% per recommendation cycle
- Extra headroom for previously-OOMed workloads (from anomaly history)

**Acceptance Criteria:**
- [ ] Recommender never suggests below lower bounds
- [ ] Rate limit caps downscale at 30%
- [ ] OOM history adds extra headroom

**Test Requirements:**
- [ ] Bounds enforcement table tests
- [ ] Rate limiting correctness
- [ ] Savings calculation accuracy

---

### Task 10: Training controller

- **Signal Type**: analytics
- **Layer**: controllers
- **Size**: L
- **Depends on**: Tasks 6 (ensemble), 7 (builtin provider), 1 (registry)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/training/types.go` — TrainingConfig, WorkloadKey (cluster, namespace, workload, resource), ShardMetadata
- `internal/analytics/rightsizing/prediction/training/querier.go` — ThanosQuerier: queries Thanos for acm_rs:* historical data, 7-90 day windows
- `internal/analytics/rightsizing/prediction/training/validator.go` — validates training output via backtest; only updates if MAPE improves > 5%
- `internal/analytics/rightsizing/prediction/training/controller.go` — TrainingController: periodic reconciler (6h default), per-workload train, model coefficients stored in ConfigMap rs-prediction-model-state, auto-shards at 1MB

**Implementation Notes:**
- Follow pattern of `OptionsBuilder.Build` in handlers/handler.go for reconcile structure
- ConfigMap storage: ~200 bytes per workload, shard at 1MB limit
- Uses controller-runtime client for ConfigMap CRUD

**Acceptance Criteria:**
- [ ] Controller runs training cycle on interval
- [ ] Model state persisted to ConfigMap and recoverable after restart
- [ ] Sharding kicks in at >4000 workloads (approaching 1MB)
- [ ] Thanos query failures don't crash the controller (backoff)

**Test Requirements:**
- [ ] Fake Thanos querier returning known series -> training completes
- [ ] ConfigMap CRUD with controller-runtime fake client
- [ ] Shard boundary test

---

### Task 11: Privacy + consent management

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Task 1 (types.go)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/privacy/consent.go` — ValidateConsent(config): blocks external calls without dataExfiltrationConsent=true; RedactLabels(labels): SHA256 hash of namespace/cluster labels
- `internal/analytics/rightsizing/prediction/privacy/audit.go` — audit metrics: consent_violations_total, prediction_api_calls_total, training_runs_total, labels_redacted_total
- `internal/analytics/rightsizing/prediction/privacy/rbac.go` — RBAC helpers: ServiceAccount, ClusterRole for rs-prediction-sa
- `internal/analytics/rightsizing/prediction/privacy/policy.go` — NetworkPolicy generation by provider type: deny-all for builtin/onnx, targeted allow for external/custom

**Acceptance Criteria:**
- [ ] ValidateConsent returns error for external provider without consent flag
- [ ] RedactLabels produces consistent SHA256 hashes
- [ ] NetworkPolicy for builtin has zero egress rules
- [ ] NetworkPolicy for external allows only the configured endpoint

**Test Requirements:**
- [ ] Consent validation: all 4 provider types x consent true/false
- [ ] Redaction consistency and irreversibility
- [ ] NetworkPolicy diff by provider type

---

## Batch 3 — Providers + Integration (Tasks 12-17)

### Task 12: ONNX provider

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Tasks 1 (interface), 11 (privacy)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/provider/onnx/provider.go` — ONNXProvider: loads .onnx from ConfigMap/PVC, runs inference via onnxruntime-go, PrivacyLevel=NoExfiltration

**Files to Modify:**
- `go.mod` — add onnxruntime-go dependency (CGO required, compatible with Konflux build)

**Acceptance Criteria:**
- [ ] Loads a test .onnx model and runs inference
- [ ] PrivacyLevel returns NoExfiltration
- [ ] Missing model ConfigMap returns clear error

**Test Requirements:**
- [ ] Mock ONNX runtime for unit tests (build-tag gated)
- [ ] Error paths: missing model, corrupt model

---

### Task 13: External API provider + redaction

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Tasks 1 (interface), 11 (privacy)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/provider/external/provider.go` — ExternalProvider: HTTP client to vendor APIs, consent check before every call, label redaction, audit logging, PrivacyLevel=ConsentRequired

**Acceptance Criteria:**
- [ ] Consent check blocks call without flag (increments consent_violations_total)
- [ ] Labels are redacted (SHA256) before HTTP send
- [ ] Every API call logged to audit metric
- [ ] HTTP timeout and retry with backoff

**Test Requirements:**
- [ ] httptest server for mock API responses
- [ ] Consent block test
- [ ] Redaction applied to request body

---

### Task 14: Custom endpoint provider

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: M
- **Depends on**: Tasks 1 (interface), 11 (privacy)

**Files to Create:**
- `internal/analytics/rightsizing/prediction/provider/custom/provider.go` — CustomProvider: REST client to customer model server, standardized request/response schema, consent check if endpoint is external, audit

**Acceptance Criteria:**
- [ ] Calls customer endpoint with standardized JSON schema
- [ ] Consent required only if endpoint is not cluster-local (svc.cluster.local)
- [ ] Audit logging on every call

**Test Requirements:**
- [ ] httptest for mock custom endpoint
- [ ] Cluster-local vs external consent gating

---

### Task 15: Helm templates (NetworkPolicy, config, RBAC)

- **Signal Type**: analytics
- **Layer**: helm-charts
- **Size**: S
- **Depends on**: Task 11 (privacy generates the specs)

**Files to Create:**
- `internal/addon/manifests/charts/mcoa/templates/rs-prediction-configmap.yaml` — prediction config ConfigMap, gated on rightSizing.prediction.enabled
- `internal/addon/manifests/charts/mcoa/templates/rs-prediction-rbac.yaml` — ServiceAccount + ClusterRole for prediction, gated
- `internal/addon/manifests/charts/mcoa/templates/rs-agent-networkpolicy.yaml` — provider-aware egress NetworkPolicy, gated

**Implementation Notes:**
- Follow pattern of existing rs-namespace-rules.yaml (gate, labels, fromJson)
- All three gated on `.Values.rightSizing.prediction.enabled`

**Acceptance Criteria:**
- [ ] Templates render when prediction enabled
- [ ] Templates produce empty output when prediction disabled
- [ ] NetworkPolicy varies by provider type

**Test Requirements:**
- [ ] Helm render test with prediction on/off
- [ ] Helm render test with each provider type

---

### Task 16: Integration (ADC, handlers, scrapeconfig, controllers, main.go)

- **Signal Type**: analytics
- **Layer**: addon-core + controllers
- **Size**: M
- **Depends on**: Tasks 1-15 (all core packages)

**Files to Modify:**
- `internal/addon/options.go` — add 3 ADC keys, prediction fields in Options struct, extend BuildOptions
- `internal/addon/helm/values.go` — pass prediction options to getRightSizingValues
- `internal/analytics/rightsizing/handlers/handler.go` — gate prediction on ADC; ensure rs-prediction-config ConfigMap
- `internal/analytics/rightsizing/handlers/options.go` — add prediction fields to Options
- `internal/analytics/rightsizing/handlers/values.go` — extend RightSizingValues with prediction subtree; fix early-exit to not drop prediction-only
- `internal/analytics/rightsizing/handlers/rs_resources.go` — extend cleanup for prediction ConfigMaps
- `internal/analytics/rightsizing/scrapeconfig.go` — append acm_rs:prediction_* to federation match[]
- `internal/analytics/rightsizing/types.go` — prediction ConfigMap name constants
- `internal/controllers/resourcecreator/controller.go` — invoke prediction ConfigMap reconcile
- `internal/controllers/watcher/controller.go` — watch prediction ConfigMap names
- `main.go` — start TrainingController in-process alongside addon manager

**Acceptance Criteria:**
- [ ] ADC with prediction=enabled reaches handler pipeline
- [ ] ScrapeConfig includes acm_rs:prediction_* metric names
- [ ] TrainingController starts and runs on interval
- [ ] Prediction-only mode (RS disabled, prediction enabled) still renders manifests

**Test Requirements:**
- [ ] options_test.go: prediction ADC keys parsed correctly
- [ ] values_test.go: prediction subtree in Helm values
- [ ] scrapeconfig test: new metric names in match[]

---

### Task 17: Perses forecast dashboards + PersAI tools

- **Signal Type**: analytics
- **Layer**: signal-package
- **Size**: S
- **Depends on**: Task 16 (metrics available in scrapeconfig)

**Files to Modify:**
- `internal/coo/manifests/values.go` — append forecast dashboards when prediction enabled
- `internal/perses/dashboards/rightsizing/namespace-rightsizing.go` — add forecast band panels (predicted vs actual)
- `internal/perses/panels/rightsizing/` — new panel constructors for forecast time series

**Acceptance Criteria:**
- [ ] Forecast panels appear in namespace RS dashboard when prediction enabled
- [ ] Dashboard builds without error and JSON round-trips
- [ ] COO values include analytics dashboards for prediction

**Test Requirements:**
- [ ] rightsizing_test.go: forecast panels in dashboard spec
- [ ] COO values test: prediction dashboard count

---

## Summary

| Batch | Tasks | New Files | Modified Files | Complexity |
|-------|-------|-----------|---------------|-----------|
| 1 — Core Models | 1-7 | ~20 | 0 | S-L |
| 2 — Intelligence + Safety | 8-11 | ~16 | 0 | M-L |
| 3 — Providers + Integration | 12-17 | ~6 | ~15 | S-M |
| **Total** | **17** | **~42** | **~15** | |

**MCO runs in parallel with MCOA Batch 1** (independent; 4 tasks, see MCO task file).
