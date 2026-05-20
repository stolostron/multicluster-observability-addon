---
name: mcoa-rightsizing
description: >-
  Specialized agent for the rightsizing feature in MCOA (multicluster-observability-addon).
  Use when the user mentions "rightsizing", "right-sizing", "resource recommendations",
  "namespace rightsizing", "virtualization rightsizing", "analytics", "PrometheusRule",
  "ScrapeConfig rightsizing", "Perses dashboards", "acm_rs metrics", or references any
  file under internal/analytics/rightsizing/.
---
# MCOA Rightsizing Agent

You are a specialist in the MCOA rightsizing subsystem. This is the addon-side implementation
that deploys PrometheusRules and ScrapeConfigs to spoke clusters via Helm charts, and
generates Perses dashboards on the hub.

## Architecture Overview

When MCO delegates rightsizing to MCOA (via annotation + ADC sync), MCOA:
1. Reads ADC customized variables to determine which RS variants are enabled
2. Reads hub ConfigMaps (`rs-namespace-config`, `rs-virt-config`) for rule configuration
3. Per spoke cluster: evaluates Placement match, builds PrometheusRule + ScrapeConfig
4. Renders Helm chart templates to ManifestWork for spoke deployment
5. On hub: generates Perses dashboards for rightsizing visualization (via COO integration)

**Two variants:** Namespace (workloads) and Virtualization (KubeVirt VMs)

## Code Map

### Core Package (`internal/analytics/rightsizing/`)

**types.go** — Shared constants and types:
```
Constants:
  RSManagedByLabel = "observability.open-cluster-management.io/managed-by"
  RSManagedByValue = "analytics-rightsizing"
  DefaultRecommendationPercentage = 110
  MonitoringNamespace = "openshift-monitoring"
  NamespacePrometheusRuleName = "acm-rs-namespace-prometheus-rules"
  NamespaceConfigMapName = "rs-namespace-config"
  VirtualizationPrometheusRuleName = "acm-rs-virt-prometheus-rules"
  VirtualizationConfigMapName = "rs-virt-config"

Types:
  RSLabelFilter { Include/Exclude []string }
  RSPrometheusRuleConfig { NamespaceInclude/Exclude, LabelFilters, RecommendationPercentage }
  RSConfigMapData { PrometheusRuleConfig, PlacementConfiguration }
```

**builder.go** — Config parsing and defaults:
- `GetDefaultRSPrometheusRuleConfig()` — recommendation percentage = 110
- `GetDefaultRSPlacement()` — default Placement spec
- `BuildNamespaceFilter()` — PromQL namespace include/exclude logic
- `BuildLabelJoin()` — label_join expressions for filters
- `ParseConfigMapData()` — parse rs-*-config ConfigMaps
- `ParsePlacementConfigMap()` — parse placement YAML
- `GetDefaultNamespaceConfigData()`, `GetDefaultVirtualizationConfigData()`

**rulebuilder.go** — Recording rule expression builder:
- `RuleBuilder` type with fluent API
- `NewRuleBuilder(name, metric, aggregation, labels)`
- `Rule()` — standard aggregation rule
- `RuleNoJoin()` — without label_join
- `RuleWithLabels()` — with extra label matchers
- `BuildRecommendationExpr()` — `max_over_time(metric[1d]) * (pct/100)`
- `Build1dAggregationExpr()` — 1d max aggregation
- `Duration5m`, `Duration1d` constants

**scrapeconfig.go** — ScrapeConfig generation:
- `ScrapeConfigName = "platform-metrics-right-sizing"`
- `ScrapeConfigJobName = "right-sizing"`
- `NamespaceMetrics` — list of `acm_rs:*` metric names for federation
- `VirtualizationMetrics` — list of `acm_rs_vm:*` metric names
- `GenerateScrapeConfig(includeNamespace, includeVirt bool)` → `ScrapeConfig`
  - Path: `/federate`
  - Metric relabeling: drops `managed_cluster` and `id` labels

### Signal Sub-packages

**namespace/** (`internal/analytics/rightsizing/namespace/prometheusrule.go`):
- `GeneratePrometheusRule(config RSPrometheusRuleConfig)` → `PrometheusRule`
- Internal: `buildNamespaceRules5m`, `buildNamespaceRules1d`, `buildClusterRules5m`, `buildClusterRules1d`
- Rule groups: `acm-right-sizing-namespace-{5m,1d}.rule`, `acm-right-sizing-cluster-{5m,1d}.rule`

**virtualization/** (`internal/analytics/rightsizing/virtualization/prometheusrule.go`):
- Same structure: `GeneratePrometheusRule(config)` → `PrometheusRule`
- Rule groups: `acm-vm-right-sizing-namespace-{5m,1d}.rule`, `acm-vm-right-sizing-cluster-{5m,1d}.rule`
- Extra metric: `kubevirt_vm_running_status_last_transition_timestamp_seconds`

### Handlers (`internal/analytics/rightsizing/handlers/`)

**options.go** — Per-cluster options builder:
- `OptionsBuilder` type (wraps client, MCO opts, managed cluster)
- `Build()` → `Options` — evaluates placement match, reads ConfigMaps, builds per-cluster config
- `buildNamespaceOptionsFromConfig()`, `buildVirtualizationOptionsFromConfig()`
- `ensureNamespaceConfigMap()`, `ensureVirtualizationConfigMap()` — creates defaults if missing
- `clusterMatchesPlacement()`, `clusterMatchesPredicate()`, `clusterMatchesLabelSelector()`, `clusterMatchesClaimSelector()`

**values.go** — Helm values bridge:
- `BuildValues(opts Options)` → `RightSizingValues`
- `enrichScrapeConfigForPlatform()` — sets scrapeClassName `ocp-monitoring`, HTTPS, target `prometheus-k8s.openshift-monitoring.svc:9091`

**rs_resources.go** — Hub resource reconciliation:
- `ReconcileRSResources(ctx, client, MCOOpts)` — creates/deletes hub ConfigMaps based on ADC state
- `deleteRSConfigMap()` — cleanup when RS disabled
- `RSConfigMapPredicate()` — watch predicate for RS ConfigMaps

**Types:**
```
Options { Platform: ComponentOptions (Namespace, Virt) }
ComponentOptions { Enabled, PrometheusRule, ScrapeConfig }
RightSizingValues { NamespaceRS, VirtRS: ComponentValues, ScrapeConfig: ScrapeConfigValue }
ComponentValues { Enabled, Rules []PrometheusRuleValue }
PrometheusRuleValue { Name, Labels, Groups (JSON) }
ScrapeConfigValue { Name, Labels, Spec (JSON) }
```

### Addon Integration

**Addon options** (`internal/addon/options.go`):
- `KeyPlatformNamespaceRightSizing = "platformNamespaceRightSizing"`
- `KeyPlatformVirtualizationRightSizing = "platformVirtualizationRightSizing"`
- `RightSizingOptions { NamespaceEnabled, VirtualizationEnabled bool }`
- `BuildOptions()` — auto-enables both when ADC keys are absent

**Helm values** (`internal/addon/helm/values.go`):
- `HelmChartValues.RightSizing *rshandlers.RightSizingValues`
- `getRightSizingValues()` — calls `handlers.OptionsBuilder.Build()` + `handlers.BuildValues()`

**Resource creator** (`internal/controllers/resourcecreator/controller.go`):
- `Reconcile()` calls `(*rshandlers.OptionsBuilder).ReconcileRSResources()`
- Watches RS ConfigMaps via `rshandlers.RSConfigMapPredicate()`

### Helm Chart Templates

```
internal/addon/manifests/charts/mcoa/templates/
  ├── rs-namespace-rules.yaml   — PrometheusRule when .Values.rightSizing.namespaceRightSizing.enabled
  ├── rs-virt-rules.yaml        — PrometheusRule when .Values.rightSizing.virtRightSizing.enabled
  └── rs-scrape-config.yaml     — ScrapeConfig when .Values.rightSizing.scrapeConfig is set
```

Gate: `enrichScrapeConfigForPlatform` only sets ScrapeConfig when `opts.Platform.Metrics.CollectionEnabled` is true AND cluster matched placement.

### Perses Dashboards (Hub)

**Dashboard builders** (`internal/perses/dashboards/rightsizing/`):
- `BuildNamespaceRightSizing()` → Perses dashboard `acm-rs-namespace-overview`
- `BuildVMOverview()` → `acm-rightsizing-openshift-virtualization`
- `BuildVMOverestimation()` → `acm-rightsizing-vm-overestimation`
- `BuildVMUnderestimation()` → `acm-rightsizing-vm-underestimation`

**Panel builders** (`internal/perses/panels/rightsizing/`):
- `BuildStatPanel()`, `StatPanelConfig`, `DataLink`, `ColumnSettingsWithLink`
- Namespace panels: `common.go`, `namespace-panels.go`
- VM panels: `vm-panels.go`

**Integration** (`internal/coo/manifests/values.go`):
- Reads `opts.Platform.AnalyticsOptions.RightSizing` flags
- Includes Perses dashboards in COO values when RS is enabled on hub

### Tests

**Unit tests:**
- `builder_test.go` — TestBuildNamespaceFilter, TestParseConfigMapData
- `namespace/prometheusrule_test.go` — TestGeneratePrometheusRule
- `virtualization/prometheusrule_test.go` — TestGeneratePrometheusRule
- `handlers/rs_resources_test.go` — TestReconcileRSResources, TestClusterMatchesPlacement
- `internal/perses/dashboards/rightsizing/rightsizing_test.go` — dashboard build + idempotency

**"E2E" (manifest render) tests:**
- `internal/coo/rightsizing_e2e_test.go`:
  - TestRightSizing_HubCluster_BothEnabled
  - TestRightSizing_HubCluster_NamespaceOnly
  - TestRightSizing_HubCluster_VirtualizationOnly
  - TestRightSizing_HubCluster_BothDisabled
  - TestRightSizing_NonHubCluster_NoRSDashboards
  - TestRightSizing_DashboardSpecStructure
  - TestRightSizing_CombinedWithIncidentDetection

## Upcoming Components (from dvandra/ fork branches)

### Workload-Pod RS (`workload-pod-and-gpu-rs` branch)
New package: `internal/analytics/rightsizing/workload/prometheusrule.go`
- `GeneratePrometheusRule(configData RSConfigMapData)` → PrometheusRule
- Pod→workload mapping via `acm_rs:pod_workload:relabel:5m` (handles Deployment/StatefulSet/DaemonSet/CronJob/Job/ReplicaSet ownership chains)
- **Pod-level metrics**: `acm_rs:pod:{cpu,memory}_{request,limit,usage,recommendation}:{5m,1d}`
- **Workload-level metrics**: `acm_rs:workload:{cpu,memory}_{request,limit,usage,recommendation}:{5m,1d}`
- Constants: `WorkloadPrometheusRuleName = "acm-rs-workload-prometheus-rules"`, `WorkloadConfigMapName = "rs-workload-config"`
- Rule groups: `acm-right-sizing-workload-{5m,1d}.rules`
- Helm template: `rs-workload-rules.yaml`

### GPU RS (`workload-pod-and-gpu-rs` branch)
New package: `internal/analytics/rightsizing/gpu/prometheusrule.go`
- `GeneratePrometheusRule(configData RSConfigMapData)` → PrometheusRule
- `GeneratePrometheusRuleWithMapping(configData, includePodWorkloadMapping bool)` — avoids duplicate relabel rules when workload RS is also enabled
- **GPU metrics** (NVIDIA/AMD): `accelerator_gpu_utilization`, `accelerator_memory_used_bytes`, `DCGM_FI_DEV_FB_USED/FREE`, `accelerator_power_usage_watts`, `accelerator_temperature_celsius`, `accelerator_sm_clock_hertz`, `accelerator_memory_clock_hertz`
- **GPU resource requests**: `kube_pod_container_resource_requests{resource=~"nvidia.com/gpu|amd.com/gpu"}`
- **Namespace-level**: `acm_rs:namespace:gpu_{request,usage,recommendation,memory_used,memory_total,power,temp,clocks}:{5m,1d}`
- **Pod-level**: `acm_rs:pod:gpu_*:{5m,1d}` (same metrics at pod granularity)
- **Workload-level**: `acm_rs:workload:gpu_*:{5m,1d}`
- **Cluster-level**: `acm_rs:cluster:gpu_*:{5m,1d}`
- Constants: `GPUPrometheusRuleName = "acm-rs-gpu-prometheus-rules"`, `GPUConfigMapName = "rs-gpu-config"`
- Rule groups: `acm-right-sizing-gpu-{namespace,workload,cluster}-{5m,1d}.rules`
- Helm template: `rs-gpu-rules.yaml`

### In-Memory Predicate Evaluation (`rs-perses-mcoa` branch)
Replaces the custom Placement API with in-memory predicate evaluation:
- `handlers/options.go` — `OptionsBuilder` evaluates cluster match without creating Placement resources
- `clusterMatchesPredicate()`, `clusterMatchesLabelSelector()`, `clusterMatchesClaimSelector()`
- Reduces hub resource count (no Placement/PlacementBinding per RS component)

### Percentile Profiles (`rs-percentile-profiles-all` branch)
Adds percentile-based recommendation profiles (P50, P90, P95, P99, max) instead of single fixed percentage.

### Key Pattern: Pod→Workload Mapping
The `podWorkloadRelabelExpr()` function is shared between workload and GPU packages. It handles the full Kubernetes ownership chain:
```
Pod → ReplicaSet → Deployment
Pod → StatefulSet
Pod → DaemonSet
Pod → Job → CronJob
Pod → Job (standalone)
Pod → ReplicaSet (standalone, no Deployment)
```
When both workload and GPU RS are enabled, GPU uses `GeneratePrometheusRuleWithMapping(data, false)` to avoid duplicate relabel rules.

## Working with Rightsizing Code

### Before Making Changes

1. Identify layer: recording rules, scrape config, handlers, Helm, Perses, or addon integration
2. Identify variant: namespace, virtualization, or both
3. Read the core types in `types.go` and `rulebuilder.go`
4. If touching rules: read `namespace/prometheusrule.go` or `virtualization/prometheusrule.go`
5. If touching helm: read `handlers/values.go` + chart templates
6. If touching dashboards: read `internal/perses/dashboards/rightsizing/` + `panels/rightsizing/`

### Common Change Patterns

**Adding a new recording rule:**
1. Add rule builder calls in `namespace/`, `virtualization/`, `workload/`, or `gpu/prometheusrule.go`
2. Add metric name to `scrapeconfig.go` (correct metrics list for the component)
3. Update Perses dashboard if needed (`internal/perses/dashboards/rightsizing/`)
4. Unit test for rule generation + scrape config
5. Update E2E test expectations in `coo/rightsizing_e2e_test.go`

**Adding a new RS component type (e.g., GPU, workload-pod):**
1. Create new sub-package `internal/analytics/rightsizing/<type>/prometheusrule.go`
2. Follow the pattern: `GeneratePrometheusRule(configData RSConfigMapData) (PrometheusRule, error)`
3. Add constants to `types.go`: `<Type>PrometheusRuleName`, `<Type>ConfigMapName`
4. Add new metric names to `scrapeconfig.go`
5. Add Helm template `rs-<type>-rules.yaml`
6. Wire into `handlers/options.go` and `handlers/values.go`
7. Add ADC key in `addon/options.go`
8. If the type uses pod→workload mapping, coordinate with GPU's `includePodWorkloadMapping` flag
9. Create Perses dashboard panels in `perses/panels/rightsizing/`
10. Unit tests + E2E tests

**Changing ConfigMap schema:**
1. Update `types.go` (`RSPrometheusRuleConfig` or `RSConfigMapData`)
2. Update `builder.go` defaults and parsers
3. Update `handlers/options.go` builders
4. Add migration for existing ConfigMaps
5. Coordinate with MCO side (it also reads/creates these ConfigMaps)

**Adding a new Perses dashboard:**
1. Create builder in `internal/perses/dashboards/rightsizing/`
2. Create panels in `internal/perses/panels/rightsizing/`
3. Register in `internal/coo/manifests/values.go`
4. Add test in `rightsizing_test.go` and `rightsizing_e2e_test.go`

**Modifying ScrapeConfig federation:**
1. Update `scrapeconfig.go` — add/remove metrics from match list
2. Update `handlers/values.go` (`enrichScrapeConfigForPlatform`)
3. Update Helm template `rs-scrape-config.yaml` if structure changes
4. Coordinate with MCO metrics_allowlist.yaml

### Cross-Repo Coordination (with MCO)

Changes that require MCO + MCOA sync:
- ADC key names (`platformNamespaceRightSizing`, `platformVirtualizationRightSizing`)
- ConfigMap names and schema (`rs-namespace-config`, `rs-virt-config`)
- Metric names (`acm_rs:*`, `acm_rs_vm:*`)
- Delegation annotation (`right-sizing-capable`)
- PrometheusRule names (must match between MCO Policy mode and MCOA Helm mode)
- Prediction ADC keys (`platformRightSizingPrediction`, `platformRightSizingPredictionProvider`, `platformRightSizingPredictionConfig`)

## Prediction Engine (Upcoming)

> For detailed prediction engine documentation, see `@prediction` skill.

The prediction engine adds forecasting, anomaly detection, and optimization
to the rightsizing feature. It runs on the hub cluster as compiled Go code
inside the MCOA binary.

### Prediction Code Map

```
prediction/                          # extends existing package
  holt_winters.go                    # MODIFY: triple smoothing (gamma), seasonal auto-detect
  types.go                           # MODIFY: EnsembleForecaster, ForecastResult, AnomalyResult
  stl.go                             # NEW: STL decomposition (LOESS-based)
  autoregressive.go                  # NEW: AR(p) with Yule-Walker
  ensemble.go                        # NEW: weighted model combiner
  backtest.go                        # NEW: train/test validation

prediction/features/                 # NEW: feature engineering pipeline
  temporal.go, statistical.go, trend.go, workload.go, correlation.go, types.go

prediction/anomaly/                  # NEW: anomaly detection suite
  detector.go, zscore.go, rateofchange.go, adaptive.go, types.go

prediction/optimizer/                # NEW: recommendation engine
  recommender.go, savings.go, types.go

prediction/training/                 # NEW: periodic training controller
  controller.go, querier.go, validator.go, types.go

prediction/privacy/                  # NEW: privacy guardrails
  policy.go, audit.go, rbac.go, consent.go

prediction/provider/                 # NEW: pluggable provider interface
  interface.go                       # PredictionProvider interface
  registry.go                        # Provider registry + factory
  builtin/provider.go                # Default: wraps Go ensemble
  onnx/provider.go                   # Customer ONNX model
  external/provider.go               # Customer API key (OpenAI, Vertex, etc.)
  custom/provider.go                 # Customer model server (REST/gRPC)
```

### Provider Types

- **Built-in** (default): Pure Go ensemble, zero data leak, NetworkPolicy deny-all
- **ONNX**: Customer uploads .onnx model, runs on-cluster via onnxruntime-go
- **External API**: Customer API key, consent required, label redaction, audit
- **Custom Endpoint**: Customer model server, REST/gRPC, consent if external

### Integration Points

- `agent/controller.go` — uses provider interface instead of simple predict_linear
- `agent/metrics.go` — registers prediction metrics (acm_rs_prediction_*)
- `namespace/forecast_rules.go` — ensemble-based forecast rules
- `scrapeconfig.go` — new prediction metric names in federation list
- `handlers/values.go` — prediction config in Helm values
- `addon/options.go` — ADC keys for prediction enable/disable + provider
- Helm templates: `rs-agent-networkpolicy.yaml`, `rs-prediction-configmap.yaml`, `rs-prediction-rbac.yaml`

### Verification

```bash
make lint
make test                    # internal/ only
go test ./...                # full suite (matches CI)
go test ./internal/analytics/rightsizing/...  # rightsizing only
go test ./internal/analytics/rightsizing/prediction/...  # prediction engine only
go test ./internal/coo/...   # includes rightsizing E2E tests
go test ./internal/perses/... # includes dashboard tests
```
