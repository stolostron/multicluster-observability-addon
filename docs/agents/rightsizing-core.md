# Right-Sizing Core

> Recording rules, ScrapeConfig federation, handler pipeline, Helm chart integration, and Perses dashboards. For the prediction engine that extends this, see [prediction-engine.md](prediction-engine.md). For the addon framework and ManifestWork delivery, see [addon-framework.md](addon-framework.md).

## Key Entry Points

### Core package (`internal/analytics/rightsizing/`)

| Path | Key symbol | One-line role |
|------|------------|---------------|
| `internal/analytics/rightsizing/types.go` | `RSPrometheusRuleConfig`, `RSConfigMapData`, `RSLabels()`, constants (`NamespacePrometheusRuleName`, `VirtualizationPrometheusRuleName`, ConfigMap names, MCO-aligned `managed-by` label) | Domain types for hub ConfigMap JSON/YAML, placement embedding, and hub resource labels that must stay compatible with MCO's `rsutility` expectations. |
| `internal/analytics/rightsizing/builder.go` | `FormatJSON`, `GetDefaultRSPlacement`, `GetDefaultRSPrometheusRuleConfig`, `BuildNamespaceFilter`, `BuildLabelJoin`, `ParseConfigMapData`, `ParsePlacementConfigMap`, `GetDefaultNamespaceConfigData`, `GetDefaultVirtualizationConfigData` | Serialize/parse hub ConfigMaps; build Prometheus label selectors for namespaces and optional `label_env` joins; defaults and validation (mutually exclusive include/exclude). |
| `internal/analytics/rightsizing/rulebuilder.go` | `RuleBuilder`, `NewRuleBuilder`, `Rule`, `RuleNoJoin`, `RuleWithLabels`, `BuildRecommendationExpr`, `Build1dAggregationExpr`, `Duration5m`, `Duration1d` | Shared recording-rule construction: optional post-expr label join, cluster-level rules without join, 1d rollups with `profile`/`aggregation` labels, recommendation math. |
| `internal/analytics/rightsizing/scrapeconfig.go` | `NamespaceMetrics`, `VirtualizationMetrics`, `GenerateScrapeConfig` | Builds `monitoring.rhobs/v1alpha1` `ScrapeConfig` for Prometheus federation (`/federate` + `match[]` list); returns `nil` when no RS features selected. |

### PrometheusRule construction

| Path | Key symbol | One-line role |
|------|------------|---------------|
| `internal/analytics/rightsizing/namespace/prometheusrule.go` | `GeneratePrometheusRule`, `buildNamespaceRules5m`, `buildNamespaceRules1d`, `buildClusterRules5m`, `buildClusterRules1d` | Emits namespace + cluster recording rules for classic workloads (`acm_rs:*`), four rule groups (namespace 5m/1d, cluster 5m/1d) in `openshift-monitoring`. |
| `internal/analytics/rightsizing/virtualization/prometheusrule.go` | `GeneratePrometheusRule`, `buildNamespaceRules5m`, ... | Same structure as namespace RS but KubeVirt metrics (`acm_rs_vm:*`), namespace rules keyed by `(name, namespace)` for VM identity. |

### Handlers (per-cluster options and Helm values)

| Path | Key symbol | One-line role |
|------|------------|---------------|
| `internal/analytics/rightsizing/handlers/options.go` | `Options`, `ComponentOptions` | In-memory result: enabled flags, generated `PrometheusRule` pointers, optional `ScrapeConfig`. |
| `internal/analytics/rightsizing/handlers/handler.go` | `OptionsBuilder`, `(*OptionsBuilder).Build`, `buildNamespaceOptionsFromConfig`, `buildVirtualizationOptionsFromConfig`, `getConfigData`, `ensureNamespaceConfigMap`, `ensureVirtualizationConfigMap`, `createDefaultConfigMap`, `clusterMatchesPlacement` | Per-managed-cluster pipeline: gate on platform/OpenShift/analytics flags; ensure/read hub ConfigMaps; evaluate `Placement` predicates in-process; build PrometheusRules; attach federation `ScrapeConfig` when platform metrics collection is on AND cluster matched placement. |
| `internal/analytics/rightsizing/handlers/values.go` | `RightSizingValues`, `BuildValues`, `enrichScrapeConfigForPlatform` | Converts `Options` into Helm JSON: rule specs as JSON strings; scrape spec as JSON; OCP scrape-class + HTTPS + static target enrichment. |
| `internal/analytics/rightsizing/handlers/rs_resources.go` | `RSConfigMapPredicate`, `(*OptionsBuilder).ReconcileRSResources`, `deleteRSConfigMap` | Hub-only predicate (RS ConfigMaps, create/update only); deletes RS hub ConfigMaps when features disabled. |

### Wiring: addon Helm values, controllers, Perses

| Path | Key symbol | One-line role |
|------|------------|---------------|
| `internal/addon/helm/values.go` | `HelmChartValues.RightSizing`, `getRightSizingValues` | Addon-framework `GetValuesFunc`: instantiates `OptionsBuilder.Build` -> `BuildValues` -> `rightSizing` key on the MCOA chart. |
| `internal/controllers/resourcecreator/controller.go` | `(*ResourceCreatorReconciler).Reconcile`, `rsBuilder.ReconcileRSResources` | Hub-wide ConfigMap lifecycle (deletion when disabled) to avoid races from concurrent per-cluster `Build()` calls. |
| `internal/controllers/watcher/controller.go` | `SetupWithManager` + `rshandlers.RSConfigMapPredicate()` | Hub watcher triggers `AddonManager` when RS ConfigMaps change so spokes refresh ManifestWork. |
| `internal/coo/manifests/values.go` | `buildNamespaceRSDashboards`, `buildVMRSDashboards`, `BuildValues` | When hub + analytics toggles allow, appends Perses dashboard JSON into `COOValues.AnalyticsDashboards`. |
| `internal/addon/options.go` | `BuildOptions`, right-sizing `CustomizedVariables` keys, auto-enable block | ADC -> `Options`; explicit enable/disable keys; auto-enables both RS features when ADC keys absent. |

### Helm templates (spoke rendering)

| Path | Purpose |
|------|---------|
| `internal/addon/manifests/charts/mcoa/templates/rs-namespace-rules.yaml` | Renders `PrometheusRule` objects in `openshift-monitoring` from `rightSizing.namespaceRightSizing.rules` (JSON spec -> YAML). |
| `internal/addon/manifests/charts/mcoa/templates/rs-virt-rules.yaml` | Same for `virtRightSizing`. |
| `internal/addon/manifests/charts/mcoa/templates/rs-scrape-config.yaml` | Renders `ScrapeConfig` in release namespace when `rightSizing.scrapeConfig` set. |

### Perses dashboard builders (`internal/perses/dashboards/rightsizing/`)

| Path | Key function | Dashboard name | Role |
|------|--------------|----------------|------|
| `namespace-rightsizing.go` | `BuildNamespaceRightSizing` | `acm-rs-namespace-overview` | Variables: `cluster`, `profile`, `days`; CPU/memory panel groups. |
| `vm-overview.go` | `BuildVMOverview` | `acm-rightsizing-openshift-virtualization` | VM aggregate stats + drill-down tables. |
| `vm-overestimation.go` | `BuildVMOverestimation` | `acm-rightsizing-vm-overestimation` | VM-level detail dashboard for overestimation. |
| `vm-underestimation.go` | `BuildVMUnderestimation` | `acm-rightsizing-vm-underestimation` | Same pattern for underestimation. |

## Patterns & Conventions

### RuleBuilder and recording rules

- `NewRuleBuilder(labelJoin)` carries an optional PromQL suffix (from `BuildLabelJoin`) appended only via `Rule()`, not `RuleNoJoin()`. Namespace-scoped rules use `Rule` for the `group_left()` join; cluster rules aggregate with `by (cluster)` and must use `RuleNoJoin`.
- 5m groups use `:5m` recording names (e.g. `acm_rs:namespace:cpu_usage:5m`). 1d groups record without `:5m` and use `RuleWithLabels` with `profile: "Max OverAll"` and `aggregation: "1d"`.
- `Duration1d` is actually a `15m` PrometheusRule group interval while expressions use `max_over_time(...[1d])` -- fresher evaluation without waiting 24h between rule runs.
- Recommendations: `max_over_time(<usage-5m-series>[1d]) * (recommendationPercentage/100)`; percentage defaults to 110 when zero.

### Handler pipeline (per managed cluster)

1. `OptionsBuilder.Build` returns empty `Options` unless `opts.Platform.Enabled` and `common.IsOpenShiftVendor(cluster)` (non-OCP clusters skip RS entirely).
2. For each feature flag (`NamespaceEnabled` / `VirtualizationEnabled`): `ensure*ConfigMap` creates hub ConfigMaps with defaults if missing; `getConfigData` loads from hub namespace and `ParseConfigMapData`.
3. `clusterMatchesPlacement` ORs `PlacementSpec.Predicates` against `ManagedCluster` labels and ClusterClaims (custom in-memory placement; no PlacementDecision objects).
4. If matched, `GeneratePrometheusRule` fills `ComponentOptions`.
5. If `opts.Platform.Metrics.CollectionEnabled`, `GenerateScrapeConfig(nsMatched, virtMatched)` unions federation matchers for whichever features matched placement.
6. `BuildValues` skips the entire `rightSizing` Helm subtree if both components have `Enabled: false`.

### ScrapeConfig federation

- CR: `monitoring.rhobs/v1alpha1` `ScrapeConfig` named `platform-metrics-right-sizing`, job `right-sizing`, path `/federate`, `match[]` entries one per federated metric name.
- Relabeling: `labeldrop` regex `managed_cluster|id` on federated series.
- Enrichment: `enrichScrapeConfigForPlatform` sets `scrapeClassName: ocp-monitoring`, `scheme: HTTPS`, static target `prometheus-k8s.openshift-monitoring.svc:9091`.

## Gotchas

1. **MCO label contract**: `RSLabels()` / `RSManagedByValue` must match MCO's `rsutility.RSLabels()` so hub resources remain discoverable across mode switches and cleanup.
2. **ADC auto-enable**: If ADC omits right-sizing keys, `BuildOptions` sets both namespace and virtualization RS enabled and `Platform.Enabled` true. Intentional bootstrap behavior.
3. **Placement inside ConfigMap**: `ParseConfigMapData` reads `placementConfiguration` from the same ConfigMap as `prometheusRuleConfig`. `ParsePlacementConfigMap` is currently unused elsewhere.
4. **Placement parse fragility**: MCO may serialize `IntOrString` fields as YAML objects, breaking unmarshal; code falls back to default placement (empty predicates = all clusters) on error.
5. **`label_env` only**: `BuildLabelJoin` silently ignores `RSLabelFilter` entries whose `LabelName` is not `label_env`, and only the first `label_env` filter participates.
6. **ScrapeConfig gating**: Federation manifest is emitted only when metrics collection is enabled AND the cluster matched placement for that feature.
7. **Metric name alignment**: Federation `match[]` names must match recording rule output names. Virt list includes raw `kubevirt_vm_running_status_last_transition_timestamp_seconds`.
8. **Helm `BuildValues` early exit**: If both components disabled, `rightSizing` is omitted entirely (`nil`).
9. **Reconcile races**: Hub ConfigMap deletes run in `ResourceCreatorReconciler`, not in per-cluster `Build`, to avoid concurrent create/delete races.
10. **Watchers ignore RS ConfigMap delete**: `RSConfigMapPredicate` drops Delete events so MCO finalizer cleanup does not fan out a full addon reconcile.

## Dependencies & Context

| Dependency | Usage |
|------------|-------|
| `prometheus-operator/prometheus-operator/.../monitoring/v1` | `PrometheusRule` / `Rule` types for spoke recording rules |
| `rhobs/obo-prometheus-operator/.../monitoring/v1alpha1` | `ScrapeConfig` CR for federation |
| `open-cluster-management.io/api/cluster/v1` | `ManagedCluster` (vendor gate, labels, claims) |
| `open-cluster-management.io/api/cluster/v1beta1` | `Placement` embedded in ConfigMap |
| `open-cluster-management.io/addon-framework` | `GetValuesFunc` / `JsonStructToValues` for Helm |
| `sigs.k8s.io/controller-runtime` | Hub controllers' client, predicates, reconcilers |
| Perses ecosystem | Hub analytics dashboards |
| MCO / hub observability (contractual) | Label keys, ConfigMap formats, mode switching |
| OpenShift cluster monitoring | Rules namespace, controller-id, federation target, scrape class |

## Links

- [prediction-engine.md](prediction-engine.md) — forecasting, anomaly detection, providers
- [addon-framework.md](addon-framework.md) — ManifestWork delivery, ADC options
- [ARCHITECTURE.md](../../ARCHITECTURE.md) — documentation index
