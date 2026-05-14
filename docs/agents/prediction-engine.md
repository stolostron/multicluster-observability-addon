# Prediction Engine

> Multi-model ensemble forecasting, anomaly detection, optimization recommendations, and pluggable prediction providers. All compiled into the MCOA binary — runs on the hub cluster with zero external dependencies by default. For the recording rules and ScrapeConfig integration, see [rightsizing-core.md](rightsizing-core.md). For the addon framework and ManifestWork delivery, see [addon-framework.md](addon-framework.md).

## Key Entry Points

- `internal/analytics/rightsizing/prediction/ensemble.go`: `EnsembleForecaster` — weighted model combiner that produces `ForecastResult` with confidence intervals. Calls `HoltWinters`, `STL`, and `AR` models, weights by inverse MAPE
- `internal/analytics/rightsizing/prediction/holt_winters.go`: `HoltWintersModel` — triple exponential smoothing (alpha/beta/gamma). `Forecast()` for batch prediction, `Update()`/`Predict()` for incremental. Seasonal period auto-detection (daily=288, weekly=2016 at 5m intervals)
- `internal/analytics/rightsizing/prediction/stl.go`: `STLModel` — Seasonal-Trend decomposition using LOESS. `Decompose()` separates Trend + Seasonal + Residual. `Forecast()` extrapolates trend + repeats seasonal
- `internal/analytics/rightsizing/prediction/autoregressive.go`: `ARModel` — AR(p) fitted via Yule-Walker equations with automatic order selection (AIC/BIC). `Fit()` + `Forecast()`
- `internal/analytics/rightsizing/prediction/backtest.go`: `Backtest()` — train/test split validation (80/20). Computes MAPE, RMSE per model. Updates ensemble weights only if improvement > 5%
- `internal/analytics/rightsizing/prediction/features/types.go`: `FeatureVector` struct and `ExtractFeatures()` — extracts temporal, statistical, trend, workload behavior, and correlation features from `acm_rs:*` time-series
- `internal/analytics/rightsizing/prediction/anomaly/detector.go`: `Detect()` — composite anomaly detector combining Z-score, rate-of-change, and adaptive threshold methods. Returns `[]AnomalyResult`
- `internal/analytics/rightsizing/prediction/optimizer/recommender.go`: `Recommend()` — combines forecast + anomaly + safety bounds into final `OptimizationResult` with target CPU/memory and estimated savings
- `internal/analytics/rightsizing/prediction/training/controller.go`: `TrainingController` — periodic reconciler (every 6h). Queries Thanos for 7-90 day history, trains per-workload models, validates via backtest, stores parameters in ConfigMaps
- `internal/analytics/rightsizing/prediction/provider/interface.go`: `PredictionProvider` interface — the pluggable contract. `Forecast()`, `Train()`, `DetectAnomalies()`, `Explain()`, `ProviderType()`, `PrivacyLevel()`
- `internal/analytics/rightsizing/prediction/provider/registry.go`: `ProviderRegistry` — factory that creates the correct provider from ADC configuration
- `internal/analytics/rightsizing/prediction/privacy/consent.go`: `ValidateConsent()` — blocks external API calls unless `dataExfiltrationConsent: true` is set. `RedactLabels()` hashes sensitive labels before external send

## Patterns & Conventions

### Model Ensemble

Three models run independently, then results are combined by the `EnsembleForecaster`:

1. **Holt-Winters** (triple exponential smoothing): best for workloads with regular daily/weekly patterns. Alpha=0.2 (level), Beta=0.1 (trend), Gamma=0.05 (seasonal). Seasonal period auto-detected from data frequency.
2. **STL** (Seasonal-Trend decomposition using LOESS): best for workloads with strong seasonality and gradual trends. Iterative LOESS fitting (3 iterations default). Forecast = extrapolated trend + repeated seasonal component.
3. **AR(p)** (Autoregressive): best for workloads with short-term autocorrelation but no clear seasonality. Order selected by minimizing BIC over p=1..10. Fitted via Yule-Walker equations (pure linear algebra).

**Ensemble weighting**: `weight_i = (1/MAPE_i) / sum(1/MAPE_j)`. Models that predict more accurately get higher weight. Weights are updated each training cycle via backtest validation.

**Forecast output**: `ForecastResult` contains `PredictedValue`, `ConfidenceLower`/`ConfidenceUpper` (90% CI from weighted variance), `DominantModel` (which model contributed most), and `FeatureImportance`.

### Feature Engineering Pipeline

`prediction/features/` extracts structured features from raw `acm_rs:*` time-series before feeding to models:

- **Temporal**: `HourOfDay` (0-23), `DayOfWeek` (0-6), `IsBusinessHours`, `IsWeekend`, `WeekOfMonth` — captures daily/weekly seasonality
- **Statistical**: `RollingMean`, `RollingStdDev`, `RollingMedian`, `P95`, `P99`, `Skewness`, `Kurtosis`, `CoefficientOfVariation` — distribution shape over configurable windows
- **Trend**: `LinearSlope`, `Acceleration` (second derivative), `ChangePointScore` — growth direction and regime shifts
- **Workload behavior**: `BurstFrequency` (spikes > 2σ/window), `BurstMagnitude`, `IdleRatio` (fraction < 10% utilization), `UtilizationEfficiency` (usage/request)
- **Correlation**: `CPUMemoryCorrelation` (Pearson), `GPUComputeCorrelation`

### Pluggable Providers

The `PredictionProvider` interface abstracts the prediction backend. Four implementations:

| Provider | Privacy | How it works |
|----------|---------|-------------|
| `builtin` (default) | NoExfiltration | Wraps the Go ensemble. Zero deps, microsecond inference |
| `onnx` | NoExfiltration | Customer uploads `.onnx` model as ConfigMap/PVC. MCOA runs inference via `onnxruntime-go` |
| `external` | ConsentRequired | Customer API key (OpenAI, Vertex, etc.). Label redaction + audit logging |
| `custom` | ConsentRequired | Customer model server (REST/gRPC). Standardized request/response schema |

**Provider selection** flows from MCO CR → ADC → `ProviderRegistry.Create()` → concrete provider instance.

### Privacy Guardrails

7 enforcement layers for the built-in/ONNX providers:
1. **Compilation boundary**: Go code, no HTTP client — physically no network calls during inference
2. **NetworkPolicy**: K8s-level egress deny (DNS + Thanos + K8s API only)
3. **Data minimization**: only aggregated `acm_rs:*`, never raw `container_*` / `kube_pod_*`
4. **RBAC**: dedicated `rs-prediction-sa` ServiceAccount
5. **Consent gate**: `dataExfiltrationConsent: true` required for external/custom
6. **Label redaction**: SHA256 hash of namespace/cluster labels before external send
7. **Audit trail**: every training/inference/API call logged as Prometheus metric

### Training Controller

`TrainingController` runs as a reconciler inside the MCOA process (not a separate CronJob):
- **Trigger**: every 6 hours (configurable via `rs-prediction-config` ConfigMap)
- **Scope**: one model ensemble per (cluster, namespace, workload, resource) tuple
- **Data source**: Thanos range queries for `acm_rs:*` metrics, 7-90 day windows
- **Storage**: model coefficients in ConfigMap `rs-prediction-model-state` (~200 bytes per workload). Sharded if fleet is large.
- **Validation**: trains on 80%, validates on 20%. Only updates weights if MAPE improves by > 5%.

### Optimization Recommender

Combines forecast + anomaly + safety into a final target:
```
target = max(
    forecast.PredictedValue * safetyMargin,     // 1.15 default
    forecast.ConfidenceUpper,                    // never below upper 90% CI
    currentUsage * minimumHeadroom,              // never below 5% over current
    historicalP99 * burstProtection              // protect against known bursts
)
```

Respects existing safety mechanisms: bounds (50m-8 CPU, 64Mi-16Gi memory), rate limits (down 30%, up 100%), rollback history (extra headroom for previously-OOMed workloads).

## Gotchas

- **Training controller shares the MCOA process.** It's not a separate pod or CronJob. If MCOA restarts, training state is preserved in ConfigMaps but the in-memory model objects are rebuilt from stored coefficients.
- **ConfigMap size limits.** Model state is ~200 bytes per workload, but a fleet with 10,000 workloads × 4 resources approaches the 1MB ConfigMap limit. The controller auto-shards across multiple ConfigMaps (`rs-prediction-model-state-0`, `-1`, etc.).
- **Holt-Winters seasonal period must match data frequency.** At 5m recording rule intervals: daily = 288 samples, weekly = 2016. If recording rules change frequency, seasonal periods must be updated.
- **STL LOESS is O(n*k) per iteration.** For 90 days of 5m data (25,920 points), each LOESS iteration processes ~26K points × window size. 3 iterations is the default; increasing iterations improves accuracy but linearly increases training time.
- **AR order selection tests p=1..maxOrder.** With `maxOrder=10`, Yule-Walker runs 10 times per training. Each is O(p²) from the Toeplitz solve, so this is fast, but if `maxOrder` is set very high it adds up across thousands of workloads.
- **External provider label redaction is irreversible.** SHA256 hashes of namespace/cluster labels cannot be reversed. This is by design (privacy), but means external API responses reference hashed identifiers that must be mapped back locally.
- **ONNX provider requires CGO.** The `onnxruntime-go` bindings use CGO. Since MCOA's Konflux build already uses `CGO_ENABLED=1` (for FIPS), this is compatible. The community Dockerfile uses `CGO_ENABLED=0` — ONNX provider would not work there without changing the build.
- **Consent validation happens at call time, not config time.** A misconfigured MCO CR with `provider: external` but missing `dataExfiltrationConsent` will start the external provider but block every call. The `consent_violations_total` metric will increment.
- **Backtest validation threshold is 5%.** The ensemble only updates weights if a new training run improves MAPE by more than 5%. This prevents oscillation but means small accuracy improvements are ignored.

## Dependencies & Context

- **Thanos Store**: the training controller queries Thanos for historical `acm_rs:*` metrics. Requires the existing ScrapeConfig federation to be working.
- **MCO CR**: prediction configuration flows from `spec.capabilities.platform.analytics.prediction` through MCO → ADC → MCOA. Three ADC keys: `platformRightSizingPrediction`, `platformRightSizingPredictionProvider`, `platformRightSizingPredictionConfig`.
- **RS Agent** (spoke-side): consumes forecast-informed target values from policy ConfigMaps delivered via ManifestWork. The agent doesn't run any models — it just reads the targets.
- **PersAI**: extends existing RS tools with `explain_forecast` (which model dominated, feature importance) and `prediction_health` (model accuracy, training status, ensemble weights).
- **Percentile profiles** (from `rs-percentile-profiles-all` branch): the optimizer can output per-percentile forecasts (P50/P90/P95/P99/max) instead of a single target.
- **In-memory predicate evaluation** (from `rs-perses-mcoa` branch): replaces Placement API with in-memory cluster matching. The training controller uses this to determine which clusters to train models for.

## Links

- [rightsizing-core.md](rightsizing-core.md) — recording rules, ScrapeConfig, handler pipeline
- [addon-framework.md](addon-framework.md) — ADC options, Helm values, ManifestWork delivery
- [CONTRIBUTING.md](../../CONTRIBUTING.md) — development setup and test commands
- [ARCHITECTURE.md](../../ARCHITECTURE.md) — documentation index
