# AGENTS.md

This file provides guidance for working with code in this repository,
for use by coding assistants.

## What This Project Does

The **multicluster-observability-addon** is an Open Cluster Management (OCM) addon that automates
the collection and forwarding of observability signals (metrics, logs, traces) from managed
clusters to central storage. It's built on the
[addon-framework](https://github.com/open-cluster-management-io/addon-framework) and runs on the
hub cluster to manage observability components on spoke clusters.

**Tech Stack**: Go 1.24, Kubernetes controller-runtime, OCM addon-framework

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

## Project Structure

```
.
├── main.go                    # Addon manager entry point
├── internal/
│   ├── addon/                 # Addon framework integration
│   ├── controllers/           # Kubernetes controllers
│   ├── logging/               # ClusterLogForwarder logic
│   ├── tracing/               # OpenTelemetry logic
│   ├── metrics/               # Prometheus metrics logic
│   ├── perses/                # Dashboard definitions
│   ├── analytics/             # Analytics and reporting
│   └── coo/                   # Cluster-logging-operator utils
├── deploy/                    # Kustomize deployment manifests
├── hack/                      # Development tools and test resources
└── .bingo/                    # Tool dependency management
```

## Development Workflow

### Quick Start

```bash
# Build and test
make addon          # Build binary
make test           # Run unit tests
make fmt            # Format code
make lint           # Run linters

# Deploy to development cluster
export REGISTRY_BASE=quay.io/YOUR_QUAY_ID
make oci            # Build and push image
make addon-deploy   # Deploy to hub cluster
```

### Iterative Development

After making code changes:

```bash
make oci
oc -n open-cluster-management-observability delete pod -l app=multicluster-observability-addon-manager
```

Additional useful targets: `make install-crds`, `make download-crds`, `make update-metrics-crds`

See [CONTRIBUTING.md](CONTRIBUTING.md) for multi-cluster development setup and [README.md](README.md) for detailed installation.

## Prerequisites and Dependencies

**Hub Cluster Requirements**:
- cert-manager operator (required)
- multicluster-observability-operator (for metrics)

**Spoke Cluster Operators** (installed automatically by addon):
- cluster-logging-operator (for logs)
- opentelemetry-operator (for traces)
- prometheus-operator (for metrics)

## Important Notes

- Addon runs on hub cluster, manages resources on spoke clusters
- Manifests deployed to spokes are copies of hub stanzas (except serviceAccountName)
- Secrets can be in spoke namespace or same namespace as stanza
- Service account for logs: `openshift-logging/mcoa-logcollector`
- Default installation: all managed clusters (modify `ClusterManagementAddOn.spec.installStrategy`)
- Configuration via `AddOnDeploymentConfig.spec.customizedVariables` (see README for details)
- Deployment modes: via MCO (recommended) or Kustomize

## Maintaining This Document

Always suggest AGENTS.md edits when architectural, structural, or conventional changes are made:
- New packages under `internal/`
- Changes to Makefile targets or build system
- New CRDs or API resources
- Changes to deployment procedures
- Updates to testing approaches
- Platform or dependency changes

Keep this file concise and universally applicable. For task-specific details, point to other documentation.
