# MCOA Rightsizing — Quick Reference

## Package Layout

```
internal/analytics/rightsizing/
  ├── types.go              — shared constants, RSLabelFilter, RSPrometheusRuleConfig
  ├── builder.go            — config parsing, defaults, filters
  ├── rulebuilder.go        — RuleBuilder fluent API for recording rules
  ├── scrapeconfig.go       — ScrapeConfig generation + metric name lists
  ├── namespace/
  │   └── prometheusrule.go — namespace recording rule groups
  ├── virtualization/
  │   └── prometheusrule.go — VM recording rule groups
  └── handlers/
      ├── handler.go        — types aggregation
      ├── options.go        — OptionsBuilder (per-cluster config + placement matching)
      ├── values.go         — BuildValues (Helm values bridge)
      └── rs_resources.go   — hub ConfigMap lifecycle

internal/perses/dashboards/rightsizing/
  ├── namespace-rightsizing.go  — acm-rs-namespace-overview
  ├── vm-overview.go            — acm-rightsizing-openshift-virtualization
  ├── vm-overestimation.go      — acm-rightsizing-vm-overestimation
  └── vm-underestimation.go     — acm-rightsizing-vm-underestimation

internal/perses/panels/rightsizing/
  ├── common.go            — shared panel config
  ├── namespace-panels.go  — namespace stat/table panels
  └── vm-panels.go         — VM stat/table panels

internal/addon/manifests/charts/mcoa/templates/
  ├── rs-namespace-rules.yaml
  ├── rs-virt-rules.yaml
  └── rs-scrape-config.yaml
```

## ADC Keys

| Key | Values | Default (when absent) |
|-----|--------|----------------------|
| `platformNamespaceRightSizing` | `enabled` / `disabled` | auto-enabled |
| `platformVirtualizationRightSizing` | `enabled` / `disabled` | auto-enabled |

## Helm Values Path

```
.Values.rightSizing.namespaceRightSizing.enabled
.Values.rightSizing.namespaceRightSizing.rules[].name/labels/groups
.Values.rightSizing.virtRightSizing.enabled
.Values.rightSizing.virtRightSizing.rules[].name/labels/groups
.Values.rightSizing.scrapeConfig.name/labels/spec
```

## Upcoming Components (from dvandra/ fork)

### Workload-Pod RS
```
WorkloadPrometheusRuleName = "acm-rs-workload-prometheus-rules"
WorkloadConfigMapName      = "rs-workload-config"
WorkloadPlacementCMName    = "rs-workload-placement"
```

Metrics: `acm_rs:pod:cpu_*`, `acm_rs:pod:memory_*`, `acm_rs:workload:cpu_*`, `acm_rs:workload:memory_*`
Pod→workload mapping: `acm_rs:pod_workload:relabel:5m`

### GPU RS
```
GPUPrometheusRuleName = "acm-rs-gpu-prometheus-rules"
GPUConfigMapName      = "rs-gpu-config"
GPUPlacementCMName    = "rs-gpu-placement"
```

Source metrics: `accelerator_gpu_utilization`, `accelerator_memory_used_bytes`, `DCGM_FI_DEV_FB_USED/FREE`, `accelerator_power_usage_watts`, `accelerator_temperature_celsius`, `accelerator_sm_clock_hertz`, `accelerator_memory_clock_hertz`, `kube_pod_container_resource_requests{resource=~"nvidia.com/gpu|amd.com/gpu"}`

Recorded metrics: `acm_rs:{namespace,pod,workload,cluster}:gpu_{request,usage,recommendation,memory_*,power,temp,clocks}:{5m,1d}`

### Branch Map (dvandra/ forks)
| MCOA Branch | MCO Branch | Feature |
|-------------|-----------|---------|
| `workload-pod-and-gpu-rs` | `workload-pod-gpu-rs` | Workload + GPU RS |
| `rs-percentile-profiles-all` | `workload-pod-gpu-rs-profiles` | Percentile profiles |
| `rs-perses-mcoa` | — | In-memory predicate eval |
| `rs-option-1-placement` | `namespace-rs-refactor` | Remove custom Placement |
| `decouple-right-sizing-from-metrics-collection` | `right-sizing-delegation` | Decouple from metrics |

## Prediction Engine

### ADC Keys
| Key | Values | Default |
|-----|--------|---------|
| `platformRightSizingPrediction` | `enabled` / `disabled` | disabled |
| `platformRightSizingPredictionProvider` | `builtin` / `onnx` / `external` / `custom` | `builtin` |
| `platformRightSizingPredictionConfig` | JSON blob | `{}` |

### Provider Config
```yaml
# Built-in (default)
builtin:
  trainingInterval: 6h
  historyWindow: 90d
  forecastHorizons: [4h, 24h, 7d]
  models: { holtWinters: {}, stl: {}, autoregressive: {} }

# ONNX (customer model, on-cluster)
onnx:
  modelSource: configmap | pvc
  modelConfigMap: rs-custom-model
  inputSchema: timeseries-v1

# External API (customer key, data leaves cluster)
external:
  endpoint: https://api.openai.com/v1/...
  apiKeySecret: rs-prediction-api-key   # Secret ref
  dataExfiltrationConsent: true         # REQUIRED
  allowedMetrics: [acm_rs:namespace:cpu_*, acm_rs:namespace:memory_*]
  redactLabels: [namespace, cluster_name]

# Custom endpoint (customer server)
custom:
  endpoint: https://ml-platform.corp:8443/predict
  protocol: rest | grpc
  tlsSecret: rs-custom-tls
  dataExfiltrationConsent: true         # required if external
```

### Prediction Metrics
```
acm_rs_prediction_forecast_value{ns, workload, resource, horizon, model}
acm_rs_prediction_confidence_lower{ns, workload, resource, horizon}
acm_rs_prediction_confidence_upper{ns, workload, resource, horizon}
acm_rs_prediction_anomaly_score{ns, workload, resource}
acm_rs_prediction_mape{cluster, model}
acm_rs_prediction_training_total{cluster, status}
acm_rs_prediction_training_duration_seconds{cluster}
acm_rs_prediction_dominant_model{cluster, ns, workload}
acm_rs_prediction_external_calls_total{endpoint, status}
acm_rs_prediction_external_bytes_sent{endpoint}
acm_rs_prediction_consent_violations_total{}
```

### Prediction Recording Rules
```
acm_rs:namespace:cpu_forecast_4h
acm_rs:namespace:cpu_forecast_24h
acm_rs:namespace:cpu_forecast_7d
acm_rs:namespace:memory_forecast_4h
acm_rs:namespace:memory_forecast_24h
acm_rs:namespace:memory_forecast_7d
acm_rs:namespace:cpu_anomaly_score
acm_rs:namespace:memory_anomaly_score
acm_rs:namespace:cpu_forecast_upper_4h
acm_rs:namespace:cpu_forecast_lower_4h
```

### Prediction Package Layout
```
prediction/
  holt_winters.go, stl.go, autoregressive.go, ensemble.go, backtest.go, types.go
  features/  — temporal, statistical, trend, workload, correlation
  anomaly/   — detector, zscore, rateofchange, adaptive
  optimizer/ — recommender, savings
  training/  — controller, querier, validator
  privacy/   — policy, audit, rbac, consent
  provider/  — interface, registry
    builtin/ — default pure Go provider
    onnx/    — customer ONNX model provider
    external/ — customer API key provider
    custom/  — customer model server provider
```

### Shipping Model
```
ACM → MCO → MCO CR (prediction config) → ADC → MCOA binary (prediction engine compiled in)
  Hub: Training controller + ensemble + anomaly + optimizer
  Spoke: RS Agent + forecast recording rules + policy ConfigMaps (via ManifestWork)
```

## Test Commands (Rightsizing Only)

```bash
go test ./internal/analytics/rightsizing/...                      # core + namespace + virt + workload + gpu
go test ./internal/analytics/rightsizing/handlers/...             # handlers + placement
go test ./internal/analytics/rightsizing/prediction/...           # prediction engine
go test ./internal/analytics/rightsizing/prediction/features/...  # feature engineering
go test ./internal/analytics/rightsizing/prediction/anomaly/...   # anomaly detection
go test ./internal/analytics/rightsizing/prediction/provider/...  # all providers
go test ./internal/coo/...                                        # E2E render tests
go test ./internal/perses/...                                     # dashboard tests
```
