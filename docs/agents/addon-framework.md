# Addon Framework

> Hub controller lifecycle, AddOnDeploymentConfig options, Helm values rendering, and ManifestWork delivery. For the right-sizing subsystem, see [rightsizing-core.md](rightsizing-core.md). For the prediction engine, see [prediction-engine.md](prediction-engine.md).

## Key Entry Points

### Hub process and managers

| Path | Primary symbol | Role |
|------|----------------|------|
| `main.go` | `runControllers` | Starts OCM addon `AddonManager`, optionally watcher, and resourcecreator manager. |
| `internal/controllers/addon/controller.go` | `NewAddonManager` | Wires addon-framework: `NewAgentAddonFactory` + `BuildHelmAgentAddon`, config GVRs, health prober, updaters, CSR registration. |
| `internal/controllers/watcher/controller.go` | `WatcherReconciler`, `NewWatcherManager` | Watches ManifestWork, hub Secret/ConfigMap metadata, RS ConfigMaps, Hypershift HostedCluster; triggers addon reconciliation. |
| `internal/controllers/watcher/cache.go` | `ReferenceCache` | Maps configuration resources to ManifestWork namespaces for hub-side change propagation. |
| `internal/controllers/resourcecreator/controller.go` | `ResourceCreatorReconciler` | Hub-only reconciler for ADC: builds options, reconciles default metrics stack, RS ConfigMaps, patches ClusterManagementAddOn placement configs. |

### `internal/addon/` - options, registration, health, Helm values

| Path | Primary symbol | Role |
|------|----------------|------|
| `internal/addon/var.go` | `FS` (embed.FS) | Embeds `internal/addon/manifests/` and the mcoa chart tree. |
| `internal/addon/config/config.go` | Constants | Addon name, hub namespace, OCM labels, subchart directory paths. |
| `internal/addon/options.go` | `Options`, `BuildOptions`, `validate` | Parses ADC into typed Options: CustomizedVariables keys for all signals; auto-enables RS when keys absent. |
| `internal/addon/addon.go` | `NewRegistrationOption`, `HealthProber`, `Updaters` | CSR registration, work health prober with feedback rules, SSA updaters. |
| `internal/addon/helm/values.go` | `HelmChartValues`, `GetValuesFunc` | Per-cluster Helm values: loads ADC, builds Options, populates metrics/logging/tracing/coo/rightSizing/obs-api. |
| `internal/addon/common/managedclusteraddon.go` | `GetAddOnDeploymentConfig` | Resolves exactly one ADC from ManagedClusterAddOn.Status.ConfigReferences. |
| `internal/addon/common/clustermanagementaddon.go` | `EnsureAddonConfig` | SSA applies ClusterManagementAddOn with default per-placement AddOnConfig entries. |
| `internal/addon/common/managedcluster.go` | `IsHubCluster`, `IsOpenShiftVendor` | Gates Helm branches (hub vs spoke, OCP vs non-OCP). |

### Helm chart layout

- Umbrella chart: `internal/addon/manifests/charts/mcoa/`
- Subcharts: `metrics`, `logging`, `tracing`, `coo`, `obs-api`
- RS templates in parent chart: `rs-namespace-rules.yaml`, `rs-scrape-config.yaml`, `rs-virt-rules.yaml`

## Patterns & Conventions

### ADC as contract from MCO

MCOA does not read `MultiClusterObservability` directly. Toggles and endpoints arrive through `AddOnDeploymentConfig` referenced in `ManagedClusterAddOn.Status.ConfigReferences`. MCO creates ADC, analytics controller syncs RS keys.

### Per-cluster Helm values flow

1. OCM addon factory registers two value sources: framework ADC translation, then MCOA domain values
2. `GetValuesFunc` fetches ADC, calls `addon.BuildOptions`
3. Short-circuits to empty values when both Platform and UserWorkloads disabled
4. Fills signal-specific values via handler+manifest packages
5. `addonfactory.JsonStructToValues` merges into chart render

### ManifestWork lifecycle

- Addon-framework turns rendered objects into ManifestWork
- `AgentAddonWithSortedManifests` stable-sorts by GVK and name to avoid diff flapping
- Watcher maintains a cache mapping hub config resources to cluster namespaces for targeted fan-out

## Gotchas

- **RS defaults when ADC keys missing**: `BuildOptions` auto-enables both namespace and virtualization RS and sets `Platform.Enabled` true. Intentional bootstrap until analytics controller syncs.
- **Explicit "disabled" keeps pipeline alive**: Prevents stale ManifestWork by not short-circuiting render.
- **Empty ADC**: `BuildOptions(nil)` returns zero options but combined with auto-RS can still yield non-empty manifests on OCP clusters.
- **Metrics validation**: Non-empty `Platform.Metrics.HubEndpoint.Host` required when metrics collection enabled.
- **Tracing never on hub**: `getTracingValues` returns nil for hub clusters.
- **Logging/tracing require OpenShift**: Returns nil for non-OCP vendors.
- **ADC reference required**: Missing `addondeploymentconfigs` in ConfigReferences fails with `ErrMissingAODCRef`.
- **Obs API toggle is annotation**: `mcoa-obs-api: "true"` on ADC; may require pod restart.
- **Watcher can be disabled**: `DISABLE_WATCHER_CONTROLLER` env var skips WatcherManager.
- **Reference cache limitation**: Assumes at most one ManifestWork per namespace per config key.

## Dependencies & Context

- **`open-cluster-management.io/addon-framework`** (v1.1.2): addonmanager, addonfactory, Helm agent addon
- **`open-cluster-management.io/api`** (v1.1.0): ManagedCluster, MCA, CMA, ADC, ManifestWork
- **`sigs.k8s.io/controller-runtime`** (v0.22.3): watcher and resourcecreator managers
- **Helm/templating**: Chart rendering performed inside addon-framework (not helm CLI)
- **Domain APIs**: OpenShift logging/metrics/tracing/OLM/HyperShift/Perses/Route/UI plugin types

## Links

- [rightsizing-core.md](rightsizing-core.md) -- recording rules, handlers, Helm templates
- [prediction-engine.md](prediction-engine.md) -- forecasting, providers, privacy
- [ARCHITECTURE.md](../../ARCHITECTURE.md) -- documentation index
