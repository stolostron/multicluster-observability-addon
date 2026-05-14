# Observability Signals

> Metrics, logging, tracing signal packages, COO integration, and Perses dashboard rendering. For right-sizing dashboards and rules, see [rightsizing-core.md](rightsizing-core.md). For addon framework delivery, see [addon-framework.md](addon-framework.md).

## Key Entry Points

### Metrics (`internal/metrics/`)

| Path | Symbol | Description |
|------|--------|-------------|
| `internal/metrics/handlers/handler.go` | `OptionsBuilder.Build` | Builds spoke Helm inputs from ManagedClusterAddOn config references: loads hub PrometheusAgent/ScrapeConfig/PrometheusRule, enforces CMAO ownership, applies relabeling for cluster identity. |
| `internal/metrics/handlers/options.go` | `Options`, `Collector` | Typed Helm value input: platform vs user-workload collectors (agents, scrape configs, rules), hub/spoke IDs, image overrides, COO subscription flag. |
| `internal/metrics/handlers/hypershift.go` | `Hypershift.GenerateResources` | Clones etcd/apiserver scrape configs per hosted cluster; adds metric relabel filters for cluster identity. |
| `internal/metrics/config/config.go` | `ImageOverrides`, constants | Hub images-list ConfigMap parsing, registry mirror rules, collector app names, scrape classes. |
| `internal/metrics/resource/resource.go` | `DefaultStackResources.Reconcile` | Hub-side: creates default PrometheusAgent per placement, SSA matching ScrapeConfig/PrometheusRule. |
| `internal/metrics/manifests/values.go` | `BuildValues`, `MetricsValues` | Turns handlers.Options into JSON Helm structs: serializes agent/scrape/rule specs, OCP vs non-OCP configuration. |

### Logging (`internal/logging/`)

| Path | Symbol | Description |
|------|--------|-------------|
| `internal/logging/handlers/handler.go` | `BuildOptions` | Resolves one ClusterLogForwarder from config references; pulls referenced Secrets/ConfigMaps; validates output specs. |
| `internal/logging/manifests/values.go` | `BuildValues`, `LoggingValues` | Serializes CLF spec + secrets for Helm; decides whether to install CLO from subscription presence. |

### Tracing (`internal/tracing/`)

| Path | Symbol | Description |
|------|--------|-------------|
| `internal/tracing/handlers/handler.go` | `BuildOptions` | Loads template OpenTelemetryCollector from config reference; discovers TLS secrets. |
| `internal/tracing/manifests/values.go` | `BuildValues`, `TracingValues` | JSON Helm payload for OTel collector spec, optional instrumentation spec. |

### COO integration (`internal/coo/`)

| Path | Symbol | Description |
|------|--------|-------------|
| `internal/coo/handlers/coo.go` | `InstallOfCOOOnTheHubIsNeeded` | Hub-only: reads COO Subscription; returns whether MCOA should render install manifests. |
| `internal/coo/manifests/values.go` | `BuildValues`, `COOValues` | Computes Perses/UI plugin Helm values; hub analytics dashboards for incident detection + RS. |

### Perses (`internal/perses/`)

| Path | Description |
|------|-------------|
| `internal/perses/dashboards/acm/` | ACM-wide operational dashboards (clusters overview, resource use, alert analysis). |
| `internal/perses/dashboards/acm/k8s/` | Kubernetes signal dashboards (apiserver, etcd, compute, networking, SLO). |
| `internal/perses/dashboards/incident-management/` | Incident detection UI dashboards. |
| `internal/perses/dashboards/rightsizing/` | RS analytics dashboards (namespace, VM overview/over/underestimation). |
| `internal/perses/dashboards/virtualization/` | OpenShift Virtualization dashboards. |
| `internal/perses/panels/` | Panel constructors per domain (queries, tables, time series). |

## Patterns & Conventions

### Signal package layout

- **Handlers** build an options struct from live API (ManagedClusterAddOn.Status.ConfigReferences, hub client Get/List)
- **Manifests** turn options into JSON-serializable Helm values (`BuildValues` pattern)
- **Metrics-only**: separates hub reconciliation (`internal/metrics/resource`) from spoke rendering (`handlers` + `manifests`)

### Perses dashboards

- Builders follow `func Build...(project, datasource, clusterLabelName string) (dashboard.Builder, error)`
- `internal/coo/manifests/values.go` runs each builder, marshals `Spec` only to JSON for Helm map
- Two namespaces: regular ACM dashboards in `open-cluster-management-observability`; analytics in `observability-analytics`

### Cross-signal gating

- Logging and tracing: only if `common.IsOpenShiftVendor(cluster)`
- Tracing: only spokes, not hub
- Metrics: if either platform or user metrics enabled
- COO: OpenShift only

## Gotchas

- **Metrics vs COO on spoke**: `DeployCOOResources` is true when metrics enabled and COO not subscribed via OLM
- **Hypershift**: UWL agent path guarded by isOpenShiftVendor; HCP monitoring ignored without UWL enabled
- **Non-OCP metrics**: `configureAgentForNonOCP` strips kube-rbac-proxy and TLS volume
- **Logging**: Exactly one CLF reference required; pipeline validation enforces matching inputs
- **Tracing JSON typo**: Helm field `InstrumenationSpec` (misspelling) must match charts
- **Perses build failures**: `buildDashboard` swallows builder errors (log only); dashboard list may be incomplete
- **COO values JSON**: `dashboards` omitempty removed so Helm sees `[]` not chart defaults

## Dependencies & Context

- **Perses**: `github.com/perses/perses` (go-sdk, community-mixins, plugins)
- **Prometheus**: prometheus-operator APIs, rhobs obo-prometheus-operator (PrometheusAgent, ScrapeConfig)
- **COO**: `rhobs/observability-operator` API (UIPlugin types)
- **Logging**: `openshift/cluster-logging-operator` API, OLM Subscriptions
- **Tracing**: `open-telemetry/opentelemetry-operator` APIs
- **Multi-cluster**: OCM addon-framework, controller-runtime
- **OpenShift**: openshift/api, Hypershift API

## Links

- [rightsizing-core.md](rightsizing-core.md) -- RS recording rules and dashboards
- [prediction-engine.md](prediction-engine.md) -- prediction engine
- [addon-framework.md](addon-framework.md) -- ADC options, Helm charts, ManifestWork delivery
- [ARCHITECTURE.md](../../ARCHITECTURE.md) -- documentation index
