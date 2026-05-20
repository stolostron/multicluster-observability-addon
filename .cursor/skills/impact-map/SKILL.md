---
name: mcoa-impact-map
description: >-
  Produce a repository impact map for MCOA (multicluster-observability-addon).
  Use when starting a new feature, analyzing a change request, or when the user
  says "plan", "analyze", "impact map" for MCOA.
---
# MCOA Repository Impact Map

Analyze the MCOA repository and produce a grounded impact map.

## MCOA-Specific Analysis Steps

1. **Read the requirement**
2. **Identify the signal type** — which observability signal does this touch?
   - Metrics: `internal/metrics/` + `internal/addon/manifests/charts/mcoa/charts/metrics/`
   - Logs: `internal/logging/` + `internal/addon/manifests/charts/mcoa/charts/logging/`
   - Traces: `internal/tracing/` + `internal/addon/manifests/charts/mcoa/charts/tracing/`
   - COO: `internal/coo/` + charts
   - Analytics: `internal/analytics/` (rightsizing, incident detection)
   - Perses: `internal/perses/`
3. **Identify affected layers**:
   - **Controller layer**: `internal/controllers/`
   - **Addon core**: `internal/addon/` (options, config, Helm values)
   - **Signal package**: the specific `internal/<signal>/` package
   - **Helm charts**: `internal/addon/manifests/charts/mcoa/`
   - **Deploy manifests**: `deploy/` (kustomize)
4. **Check cross-repo impact**:
   - Does this require MCO changes? (MultiClusterObservability capabilities)
   - Does this require new upstream CRDs? (update `make download-crds`)
5. **Scan for real paths and symbols**: search, grep, read
6. **Produce impact map** using `templates/impact-map-template.md`
7. **STOP for human review**

## Key Directories

| What | Where |
|------|-------|
| Addon core (values, options) | `internal/addon/` |
| Controllers | `internal/controllers/{addon,watcher,resourcecreator}/` |
| Metrics signal | `internal/metrics/` |
| Logging signal | `internal/logging/` |
| Tracing signal | `internal/tracing/` |
| COO integration | `internal/coo/` |
| Analytics / rightsizing | `internal/analytics/` |
| Helm umbrella chart | `internal/addon/manifests/charts/mcoa/` |
| Deploy kustomize | `deploy/` |
| Entry point + scheme | `main.go` |
| Tests | `*_test.go` colocated with source |

## Rules

- NEVER guess paths. Verify by searching.
- ALWAYS identify which signal(s) and layers are affected.
- ALWAYS check Helm chart impact when signal packages change.
- Check if new `AddOnDeploymentConfig` keys are needed.
- Save to `docs/impact-maps/<feature-slug>.md`.
