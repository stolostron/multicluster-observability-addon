# multicluster-observability-addon

OCM addon automating observability signal collection (metrics, logs, traces) from managed spoke clusters to central storage. Go 1.24 + controller-runtime + addon-framework.

## Architecture Overview

### Core Components

**Addon Manager** (`main.go`) - Hub-side controller managing addon lifecycle across spoke clusters

**Key Packages**:
- `internal/addon/` - Addon framework integration
- `internal/logging/` - ClusterLogForwarder manifest generation (cluster-logging-operator)
- `internal/tracing/` - OpenTelemetryCollector and Instrumentation manifests (RHBOTO)
- `internal/metrics/` - PrometheusAgent and ScrapeConfig manifests (prometheus-operator)
- `internal/controllers/` - Kubernetes reconciliation controllers
- `internal/perses/` - Dashboard definitions
- `internal/coo/` - cluster-logging-operator utilities
- `internal/analytics/` - Analytics reporting

### How It Works

1. User configures observability capabilities via `AddOnDeploymentConfig` on hub cluster
2. Addon manager builds manifests based on configuration stanzas
3. Manifests deployed to spoke clusters via `ManifestWork` resources
4. Operators install on spoke clusters and forward signals to configured destinations

### Three Observability Signals

- **Metrics**: PrometheusAgent scrapes and forwards metrics
- **Logs**: ClusterLogForwarder collects and forwards logs
- **Traces**: OpenTelemetryCollector receives and forwards traces

## Development Workflow

### Quick Start

```bash
make addon          # Build binary
make test           # Run unit tests
make fmt            # Format code
make lint           # Run linters

make oci            # Build+push image (set REGISTRY_BASE env first)
make addon-deploy   # Deploy to hub
```

### Iterative Development

After making code changes:

```bash
make oci
oc -n open-cluster-management-observability delete pod -l app=multicluster-observability-addon-manager
```

Additional useful targets: `make install-crds`, `make download-crds`, `make update-metrics-crds`

See [CONTRIBUTING.md](CONTRIBUTING.md) for multi-cluster development setup and [README.md](README.md) for detailed installation.

## Common Mistakes to Avoid

TBD - to be populated with project-specific gotchas and anti-patterns

## Prerequisites and Dependencies

See [README.md](README.md)

## Maintaining This Document

Always suggest AGENTS.md edits when architectural, structural, or conventional changes are made.
