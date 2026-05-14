# Prediction Engine Deployment Guide

> How to deploy and operate the right-sizing prediction engine on an OpenShift cluster using ACM Multicluster Observability. This guide covers the default builtin model with full data-privacy guarantees.

## Prerequisites

- OpenShift Container Platform 4.14+
- Red Hat Advanced Cluster Management 2.12+
- Multicluster Observability Operator (MCO) installed on the hub cluster
- At least one managed cluster with right-sizing recording rules active
- Thanos on the hub with at least 7 days of historical `acm_rs:*` metric data

## Quick Start

Apply this patch to your `MultiClusterObservability` CR on the hub:

```yaml
apiVersion: observability.open-cluster-management.io/v1beta2
kind: MultiClusterObservability
metadata:
  name: observability
spec:
  capabilities:
    platform:
      analytics:
        namespaceRightSizingRecommendation:
          enabled: true
        prediction:
          enabled: true
```

This enables namespace-level CPU/memory prediction with the builtin model. No data leaves the cluster.

## Full Configuration

### Enabling All Dimensions

To enable prediction across all four right-sizing dimensions:

```yaml
spec:
  capabilities:
    platform:
      analytics:
        namespaceRightSizingRecommendation:
          enabled: true
        workloadPodRightSizingRecommendation:
          enabled: true
        gpuRightSizingRecommendation:
          enabled: true
        virtualizationRightSizingRecommendation:
          enabled: true
        prediction:
          enabled: true
          provider:
            type: builtin
          config:
            trainingIntervalHours: 6
            historyDays: 30
            safetyMarginPercent: 115
```

The prediction engine automatically activates per dimension based on which RS components are enabled. If only `namespaceRightSizingRecommendation` and `prediction` are enabled, only namespace-level forecasting runs.

### CRD Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `prediction.enabled` | bool | `false` | Master toggle for the prediction engine |
| `prediction.provider.type` | enum | `builtin` | Provider selection: `builtin`, `onnx`, `external`, `custom` |
| `prediction.provider.dataExfiltrationConsent` | bool | `false` | Must be `true` for external/custom providers |
| `prediction.provider.onnxModelConfigMapRef` | ObjectRef | — | ConfigMap containing `.onnx` model (ONNX provider only) |
| `prediction.provider.externalAPIKeySecretRef` | ObjectRef | — | Secret with vendor API key (external provider only) |
| `prediction.provider.customEndpointURL` | string | — | URL of custom model server (custom provider only) |
| `prediction.config.trainingIntervalHours` | int | `6` | How often the model retrains (hours) |
| `prediction.config.historyDays` | int | `30` | Days of historical data used for training |
| `prediction.config.safetyMarginPercent` | int | `115` | Headroom on forecasts (115 = 15% buffer) |

### CRD Path

```
spec.capabilities.platform.analytics.prediction
```

## Provider Types

### Builtin (Default — Recommended)

The builtin provider runs a pure Go multi-model ensemble directly in the MCOA binary:

- **Models**: Holt-Winters (triple exponential smoothing), STL (seasonal-trend decomposition), AR(p) (autoregressive with BIC order selection)
- **Weighting**: Inverse-MAPE from 80/20 backtest validation; weights update only when improvement > 5%
- **Privacy**: Zero network calls. All computation in-process. NetworkPolicy denies all egress.
- **Dependencies**: None beyond stdlib `math` and `sort`

```yaml
prediction:
  enabled: true
  provider:
    type: builtin
```

### ONNX (Customer-Provided Model)

Run a customer-trained ONNX model on-cluster. The model file is loaded from a ConfigMap — no data leaves the cluster.

```yaml
prediction:
  enabled: true
  provider:
    type: onnx
    onnxModelConfigMapRef:
      name: my-prediction-model
```

> **Note**: ONNX provider requires CGO and `onnxruntime-go`. Currently a stub — full implementation requires a build-tag-gated binary.

### External API (Vendor Service)

Send data to a third-party prediction API using a customer-provided API key. **Requires explicit consent.**

```yaml
prediction:
  enabled: true
  provider:
    type: external
    externalAPIKeySecretRef:
      name: vendor-api-key
    dataExfiltrationConsent: true   # REQUIRED — blocked without this
```

### Custom Endpoint (Customer Model Server)

Point to a customer-operated model server. Consent is required unless the endpoint is cluster-local (`svc.cluster.local`).

```yaml
prediction:
  enabled: true
  provider:
    type: custom
    customEndpointURL: "http://my-model.ml-serving.svc.cluster.local:8080"
```

## Data Privacy Guarantees

### Builtin Provider (Zero Exfiltration)

| Layer | Protection |
|-------|-----------|
| **Provider code** | Pure Go math — no HTTP client, no network calls |
| **NetworkPolicy** | `egress: []` — deny-all egress on prediction pods |
| **Consent gate** | `ValidateConsent("builtin", false)` always passes; external/custom blocked |
| **RBAC** | `rs-prediction-sa` scoped to ConfigMaps and PrometheusRules only |
| **Audit metrics** | `consent_violations_total`, `prediction_api_calls_total` tracked |
| **Label redaction** | Labels SHA-256 hashed before any external call (defense in depth) |

### Verifying Privacy on a Running Cluster

```bash
# 1. Verify NetworkPolicy denies all egress
oc get networkpolicy rs-prediction-policy \
  -n open-cluster-management-addon-observability -o yaml
# Expect: spec.egress: []

# 2. Verify provider type is builtin
oc get configmap rs-prediction-config \
  -n open-cluster-management-addon-observability \
  -o jsonpath='{.data.config\.json}' | jq .
# Expect: {"provider": "builtin", ...}

# 3. Verify consent is NOT given
oc get multiclusterobservability observability \
  -o jsonpath='{.spec.capabilities.platform.analytics.prediction.provider.dataExfiltrationConsent}'
# Expect: false (or empty)

# 4. Verify RBAC is scoped
oc get clusterrole rs-prediction-role -o yaml
# Expect: only configmaps, secrets (get), prometheusrules
```

## Monitoring and Observability

### Forecast Metrics

The prediction engine produces these metrics, federated to the hub via ScrapeConfig:

| Metric | Dimensions | Description |
|--------|-----------|-------------|
| `acm_rs:prediction_forecast_cpu` | namespace | Namespace CPU forecast |
| `acm_rs:prediction_forecast_memory` | namespace | Namespace memory forecast |
| `acm_rs:prediction_forecast_workload_cpu` | namespace, workload | Workload CPU forecast |
| `acm_rs:prediction_forecast_workload_memory` | namespace, workload | Workload memory forecast |
| `acm_rs:prediction_forecast_gpu_utilization` | namespace | GPU utilization forecast |
| `acm_rs:prediction_forecast_gpu_memory` | namespace | GPU memory forecast |
| `acm_rs:prediction_forecast_vm_cpu` | namespace, name | VM CPU forecast |
| `acm_rs:prediction_forecast_vm_memory` | namespace, name | VM memory forecast |
| `acm_rs:prediction_anomaly_score` | namespace | Namespace anomaly score |
| `acm_rs:prediction_anomaly_score_workload` | namespace, workload | Workload anomaly score |
| `acm_rs:prediction_anomaly_score_gpu` | namespace | GPU anomaly score |
| `acm_rs:prediction_anomaly_score_vm` | namespace, name | VM anomaly score |
| `acm_rs:prediction_model_accuracy` | — | Overall model MAPE |
| `acm_rs:prediction_ensemble_weight` | model | Per-model ensemble weight |

### Checking Training Controller

```bash
# View training logs
oc logs -n open-cluster-management-addon-observability \
  deployment/multicluster-observability-addon \
  | grep "training:"

# Check model state ConfigMaps
oc get configmap -n open-cluster-management-addon-observability \
  -l app.kubernetes.io/component=rs-prediction-state

# View model weights and last MAPE
oc get configmap rs-prediction-model-state-0 \
  -n open-cluster-management-addon-observability \
  -o jsonpath='{.data.states}' | jq . | head -20
```

### Perses Dashboards

Each right-sizing dashboard has a collapsible **Forecasting** section:

- **Namespace RS Dashboard**: CPU/Memory forecast vs actual
- **Workload RS Dashboard**: Per-workload CPU/Memory forecast
- **GPU RS Dashboard**: GPU utilization and memory forecast
- **VM RS Dashboard**: VM CPU/Memory forecast

Panels show "No data" when prediction is disabled for that dimension — this is expected.

## Troubleshooting

### Prediction Not Producing Metrics

1. **Check prediction is enabled**:
   ```bash
   oc get multiclusterobservability observability \
     -o jsonpath='{.spec.capabilities.platform.analytics.prediction.enabled}'
   ```

2. **Check ADC has prediction keys**:
   ```bash
   oc get addondeploymentconfig -n open-cluster-management \
     -o yaml | grep -A2 platformRightSizingPrediction
   ```

3. **Check training has enough history** — the controller needs at least 5 data points (5 hours at 1h step). New clusters need time to accumulate data.

4. **Check Thanos connectivity**:
   ```bash
   oc logs deployment/multicluster-observability-addon \
     -n open-cluster-management-addon-observability \
     | grep "training: .* query"
   ```

### Model State Not Persisting

Check the ConfigMap shards:
```bash
oc get configmap -n open-cluster-management-addon-observability \
  -l app.kubernetes.io/component=rs-prediction-state -o name
```

If missing, the controller may not have completed its first training cycle (waits up to `trainingIntervalHours`).

### External Provider Blocked

If using `external` or `custom` provider and getting consent errors:
```bash
# Check audit metric
# In Thanos/Grafana:
consent_violations_total{provider_type="external"}
```

Ensure `dataExfiltrationConsent: true` is set in the CR.

## Architecture Overview

```
Hub Cluster
├── MCO CR (prediction spec)
│   └── Analytics Controller → ADC sync (prediction keys + resolved secrets)
│
├── MCOA Addon
│   ├── Training Controller (periodic, queries Thanos)
│   │   ├── Namespace: acm_rs:namespace:cpu_usage:5m, memory_usage:5m
│   │   ├── Workload: acm_rs:workload:cpu_usage:5m, memory_usage:5m
│   │   ├── GPU: acm_rs:namespace:gpu_usage:5m, gpu_memory_used:5m
│   │   └── VM: acm_rs_vm:namespace:cpu_usage:5m, memory_usage:5m
│   │
│   ├── Builtin Provider (in-process)
│   │   └── Ensemble: HW + STL + AR(p) → ForecastResult
│   │
│   ├── Model State (ConfigMap shards, auto-split at 1MB)
│   └── Privacy Layer (NetworkPolicy, RBAC, consent, audit)
│
├── Perses Dashboards (forecast panels per RS dimension)
└── ScrapeConfig (14 prediction metrics federated)

Managed Clusters
└── PrometheusRules (recording rules for acm_rs:* metrics)
    → Federated to hub via ScrapeConfig
```
