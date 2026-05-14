# Multi-Cluster Observability Addon (MCOA)

An ACM addon that deploys observability components (metrics, logs, traces, analytics) to managed OCP clusters. Runs on the hub, renders Helm charts into ManifestWork via the OCM addon-framework.

## Quick Reference

### Building and Testing
- `make addon` — compile the binary
- `make lint` — golangci-lint + Dockerfile label verify
- `make test` — unit tests (`internal/` only)
- `go test ./...` — full test suite (matches CI)
- `make download-crds` — fetch upstream CRDs (required before deploy)
- `make oci-build` — build container image

### Deployment
- `make addon-deploy` — deploy to hub cluster via kustomize
- `make oci-push` — push container image

## Architecture at a Glance

> For the full architecture index, see [ARCHITECTURE.md](ARCHITECTURE.md).

Single addon manager on hub using `open-cluster-management.io/addon-framework`. Renders Helm charts → ManifestWork → spoke clusters.

All production code lives under `internal/`. Key packages:

- `internal/addon/` — core addon logic, options, config, Helm values, embedded charts
- `internal/controllers/` — addon, watcher, resourcecreator controllers
- `internal/metrics/`, `internal/logging/`, `internal/tracing/` — signal packages
- `internal/analytics/rightsizing/` — right-sizing recording rules, handlers, prediction engine
- `internal/analytics/rightsizing/prediction/` — multi-model forecasting, anomaly detection, pluggable providers
- `internal/coo/`, `internal/perses/` — COO and Perses dashboard integration

### Relationship to MCO
- MCO installs MCOA via `MultiClusterObservability.spec.capabilities` (ACM 2.12+)
- MCO is the metrics prerequisite; they share some API types but are separate repos
- Configuration flows: MCO CR → AddOnDeploymentConfig → MCOA

## Coding Standards

- **Go 1.25+**, favor standard library, minimize new deps
- **Lint**: `.golangci.yml` v2 — `err113`, `errorlint`, `revive`, `testifylint`, `modernize`
- **Imports**: `gci` / `gofumpt` / `goimports` ordering
- **Errors**: wrap with `%w`, handle once
- **Tests**: `testify` (assert/require), colocated `*_test.go`, `controller-runtime` fake client
- **Containers**: `Dockerfile` (distroless, CGO_ENABLED=0) + `Dockerfile.Konflux` (UBI, CGO_ENABLED=1 for FIPS)

## Documentation

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full documentation index.

## Context Exclusions

Do NOT read or index: `vendor/`, `.git/`, `bin/`, `*.log`, `node_modules/`.
