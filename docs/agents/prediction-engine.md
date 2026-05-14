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
- `internal/analytics/rightsizing/prediction/optimizer/recommender.go`: `Recommend()` — combines forecast + anomaly + safety bounds into final `OptimizationResult` with target CPU/memory and estimated savings; `RecommendWithGPU()` adds `TargetGPU` and `TargetGPUMemory` from GPU utilization and GPU memory forecasts
- `internal/analytics/rightsizing/prediction/training/controller.go`: `TrainingController` — periodic reconciler (every 6h). Queries Thanos for 7-90 day history across all **enabled** RS dimensions (namespace, workload/pod, GPU, VM), trains per-series models, validates via backtest, stores parameters in ConfigMaps
- `internal/analytics/rightsizing/prediction/provider/interface.go`: `PredictionProvider` interface — the pluggable contract. `Forecast()`, `Train()`, `DetectAnomalies()`, `Explain()`, `ProviderType()`, `PrivacyLevel()`
- `internal/analytics/rightsizing/prediction/provider/registry.go`: `ProviderRegistry` — factory that creates the correct provider from ADC configuration
- `internal/analytics/rightsizing/prediction/privacy/consent.go`: `ValidateConsent()` — blocks external API calls unless `dataExfiltrationConsent: true` is set. `RedactLabels()` hashes sensitive labels before external send
- `internal/analytics/rightsizing/scrapeconfig.go`: `GenerateScrapeConfig` — builds federation `match[]` lists for namespace, virtualization (VM), workload/pod, GPU, and prediction metrics when each feature flag is set

## Multi-Dimension Architecture

Training, forecasting, and federation are aligned with four **right-sizing dimensions**. Each dimension uses distinct recording-rule inputs (5m rollups where applicable):

| Dimension | Resource signals (examples) |
|-----------|------------------------------|
| **Namespace** (CPU/memory) | `acm_rs:namespace:cpu_usage:5m`, `acm_rs:namespace:memory_usage:5m` |
| **Workload/pod** (CPU/memory) | `acm_rs:workload:cpu_usage:5m`, `acm_rs:workload:memory_usage:5m` |
| **GPU** (utilization / memory) | `acm_rs:namespace:gpu_usage:5m`, `acm_rs:namespace:gpu_memory_used:5m` |
| **VM** (CPU/memory) | `acm_rs_vm:namespace:cpu_usage:5m`, `acm_rs_vm:namespace:memory_usage:5m` |

The **training controller** appends Thanos queries only for dimensions whose flags are set in `TrainingConfig`: `NamespaceEnabled`, `WorkloadEnabled`, `GPUEnabled`, `VMEnabled`. Those flags are populated from the active RS capability flags on the hub (`NamespaceRightSizing`, `WorkloadPodRightSizing`, `GPURightSizing`, `VirtualizationRightSizing` via `internal/analytics/rightsizing/handlers/values.go`), so turning off a RS component stops training and history pull for that dimension.

**Forecast exposition** uses fourteen Prometheus metrics (federated when prediction is enabled; see `PredictionMetrics` in `scrapeconfig.go`):

- Namespace: `acm_rs:prediction_forecast_cpu`, `acm_rs:prediction_forecast_memory`
- Workload: `acm_rs:prediction_forecast_workload_cpu`, `acm_rs:prediction_forecast_workload_memory`
- GPU: `acm_rs:prediction_forecast_gpu_utilization`, `acm_rs:prediction_forecast_gpu_memory`
- VM: `acm_rs:prediction_forecast_vm_cpu`, `acm_rs:prediction_forecast_vm_memory`
- Anomaly (cross-cutting): `acm_rs:prediction_anomaly_score`, `acm_rs:prediction_anomaly_score_workload`, `acm_rs:prediction_anomaly_score_gpu`, `acm_rs:prediction_anomaly_score_vm`
- Quality: `acm_rs:prediction_model_accuracy`, `acm_rs:prediction_ensemble_weight`

**Perses** rightsizing dashboards (`internal/perses/dashboards/rightsizing/`) add a collapsible **Forecasting** section on namespace, workload, GPU, and VM dashboards, with **ten** panels overall (forecast vs. actual time series per dimension, wired through `internal/perses/panels/rightsizing/`).

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
- **Scope**: one model ensemble per time series key (cluster, namespace, workload identity, resource label) — covering each enabled dimension’s CPU/memory/utilization streams
- **Data source**: Thanos range queries for `acm_rs:*` and `acm_rs_vm:*` metrics (see Multi-Dimension Architecture), 7-90 day windows
- **Gating**: `NamespaceEnabled`, `WorkloadEnabled`, `GPUEnabled`, `VMEnabled` in `TrainingConfig` — only queries and trains for dimensions whose RS components are active
- **Storage**: model coefficients in ConfigMap `rs-prediction-model-state` (~200 bytes per workload). Sharded if fleet is large.
- **Validation**: trains on 80%, validates on 20%. Only updates weights if MAPE improves by > 5%.

### Optimization Recommender

`Recommend()` combines forecast + anomaly + safety into CPU/memory targets. **`RecommendWithGPU()`** extends that path with GPU utilization and GPU memory: it fills `TargetGPU` and `TargetGPUMemory` on `OptimizationResult`, using the same margin and rate-limit machinery as CPU/memory.

Default **GPU bounds** in `BoundsConfig`: utilization **0–100%** (`MinGPU` / `MaxGPU`); GPU memory **0–80 GiB** (`MinGPUMemoryMiB` / `MaxGPUMemoryMiB`, default max 81920 MiB).

CPU/memory targeting still follows:
```
target = max(
    forecast.PredictedValue * safetyMargin,     // 1.15 default
    forecast.ConfidenceUpper,                    // never below upper 90% CI
    currentUsage * minimumHeadroom,              // never below 5% over current
    historicalP99 * burstProtection              // protect against known bursts
)
```

Respects existing safety mechanisms: bounds (50m-8 CPU, 64Mi-16Gi memory), rate limits (down 30%, up 100%), rollback history (extra headroom for previously-OOMed workloads).

### ScrapeConfig and federation

`GenerateScrapeConfig(includeNamespace, includeVirtualization, includeWorkloadPod, includeGPU, includePrediction bool)` unions dedicated metric name lists:

- **Namespace** → `NamespaceMetrics`
- **Virtualization (VM)** → `VirtualizationMetrics`
- **Workload/pod** → `WorkloadPodMetrics`
- **GPU** → `GPUMetrics`
- **Prediction** → `PredictionMetrics` (the fourteen forecast/anomaly/quality series above)

The handler passes placement-derived RS matches plus whether prediction is enabled so spokes only federate the slices they need.

## Gotchas

- **Training controller shares the MCOA process.** It's not a separate pod or CronJob. If MCOA restarts, training state is preserved in ConfigMaps but the in-memory model objects are rebuilt from stored coefficients.
- **ConfigMap size limits.** Model state is ~200 bytes per trained series, but many workloads × several resource types per enabled dimension approaches the 1MB ConfigMap limit. The controller auto-shards across multiple ConfigMaps (`rs-prediction-model-state-0`, `-1`, etc.).
- **Holt-Winters seasonal period must match data frequency.** At 5m recording rule intervals: daily = 288 samples, weekly = 2016. If recording rules change frequency, seasonal periods must be updated.
- **STL LOESS is O(n*k) per iteration.** For 90 days of 5m data (25,920 points), each LOESS iteration processes ~26K points × window size. 3 iterations is the default; increasing iterations improves accuracy but linearly increases training time.
- **AR order selection tests p=1..maxOrder.** With `maxOrder=10`, Yule-Walker runs 10 times per training. Each is O(p²) from the Toeplitz solve, so this is fast, but if `maxOrder` is set very high it adds up across thousands of workloads.
- **External provider label redaction is irreversible.** SHA256 hashes of namespace/cluster labels cannot be reversed. This is by design (privacy), but means external API responses reference hashed identifiers that must be mapped back locally.
- **ONNX provider requires CGO.** The `onnxruntime-go` bindings use CGO. Since MCOA's Konflux build already uses `CGO_ENABLED=1` (for FIPS), this is compatible. The community Dockerfile uses `CGO_ENABLED=0` — ONNX provider would not work there without changing the build.
- **Consent validation happens at call time, not config time.** A misconfigured MCO CR with `provider: external` but missing `dataExfiltrationConsent` will start the external provider but block every call. The `consent_violations_total` metric will increment.
- **Backtest validation threshold is 5%.** The ensemble only updates weights if a new training run improves MAPE by more than 5%. This prevents oscillation but means small accuracy improvements are ignored.

## File inventory

Rough layout on `feature/rs-prediction-engine`:

- **`internal/analytics/rightsizing/prediction/`** — **34** production Go files (excluding `*_test.go`) across packages: root (`ensemble`, models, `backtest`, `types`), `features/`, `anomaly/`, `optimizer/`, `training/`, `privacy/`, `provider/`.
- **`internal/perses/panels/rightsizing/`** — **10** panel modules for forecasting and quality visualization: `forecast_cpu.go`, `forecast_memory.go`, `forecast_workload_cpu.go`, `forecast_workload_memory.go`, `forecast_gpu_util.go`, `forecast_gpu_memory.go`, `forecast_vm_cpu.go`, `forecast_vm_memory.go`, `anomaly_score.go`, `model_accuracy.go` (dashboards also use `common.go`, `namespace-panels.go`, `workload-panels.go`, `gpu-panels.go`, `vm-panels.go` for layout).
- **`internal/addon/manifests/charts/mcoa/templates/`** — **3** Helm templates for prediction: `rs-prediction-configmap.yaml`, `rs-prediction-rbac.yaml`, `rs-prediction-networkpolicy.yaml`.

## Dependencies & Context

- **Thanos Store**: the training controller queries Thanos for historical `acm_rs:*` and `acm_rs_vm:*` metrics (per enabled dimension). Requires the existing ScrapeConfig federation to be working.
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
