# MCOA Repository Impact Map

## Feature
<!-- One-line description -->

## Source
<!-- Ticket reference or requirement -->

## Signal Type
- [ ] Metrics (`internal/metrics/`)
- [ ] Logging (`internal/logging/`)
- [ ] Tracing (`internal/tracing/`)
- [ ] COO (`internal/coo/`)
- [ ] Analytics / Rightsizing (`internal/analytics/`)
- [ ] Perses (`internal/perses/`)
- [ ] Core addon (`internal/addon/`)
- [ ] Controllers (`internal/controllers/`)

## Codebase Analysis
<!-- How you analyzed: file search, grep, symbol tracing -->

## Layer Impact Map

### Controllers (`internal/controllers/`)
**Affected**: Yes / No
Changes:
- `<verified-path>` — <what and why>

### Addon Core (`internal/addon/`)
**Affected**: Yes / No
Changes:
- `<verified-path>` — <what and why>

### Signal Package (`internal/<signal>/`)
**Affected**: Yes / No
Changes:
- `<verified-path>` — <what and why>

Existing patterns to follow:
- `<SymbolName>` in `<file-path>`

### Helm Charts (`internal/addon/manifests/charts/mcoa/`)
**Affected**: Yes / No
Changes:
- `charts/<subchart>/templates/<file>` — <what and why>
- `charts/<subchart>/values.yaml` — <what and why>

### Deploy Manifests (`deploy/`)
**Affected**: Yes / No

### Entry Point (`main.go`)
**Affected**: Yes / No (new scheme registrations, controller wiring)

## Cross-Repo Impact
- [ ] Requires MCO changes (capabilities, CRD, config)
- [ ] Requires new upstream CRDs (`make download-crds` update)
- [ ] New AddOnDeploymentConfig keys needed

## Risks & Open Questions
- [ ] <Risk or question>

## Proposed Task Breakdown
| # | Title | Layer | Signal | Complexity |
|---|-------|-------|--------|------------|
| 1 | ...   | ...   | ...    | S/M/L      |

---
**STATUS**: AWAITING HUMAN REVIEW
