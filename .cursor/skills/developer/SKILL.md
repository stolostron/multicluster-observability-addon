---
name: mcoa-developer
description: >-
  Implement a structured task for MCOA following approved plan and Go conventions.
  Use when the user says "implement", "develop", "code" for an MCOA task.
---
# MCOA Developer

Implement structured tasks for the multicluster-observability-addon.

## Before Writing Code

1. Read the task spec from `docs/tasks/`
2. Read AGENTS.md for architectural context
3. Read all files in "Files to Modify"
4. Read reference patterns in "Implementation Notes"

## MCOA Code Organization

- All production code under `internal/` — nothing exported
- Signal-specific logic in `internal/<signal>/`
- Helm charts in `internal/addon/manifests/charts/mcoa/`
- Controller wiring in `internal/controllers/`
- Addon options/config in `internal/addon/`

## Implementation Workflow

1. Create branch: `agent/<ticket>-<description>`
2. Implement following existing patterns
3. If modifying signal logic, update corresponding Helm chart/values
4. Write unit tests (colocated `*_test.go`)
5. Run `make lint`
6. Run `make test`
7. Self-review against acceptance criteria
8. Commit: `feat|fix|test(<signal>): description`
9. Open PR

## Post-Implementation Checks

- [ ] `make lint` passes
- [ ] `make test` passes (also try `go test ./...` to match CI)
- [ ] Helm chart templates render correctly if charts were modified
- [ ] New AddOnDeploymentConfig keys documented in README if applicable
- [ ] All acceptance criteria met
- [ ] No changes outside `internal/` unless justified (deploy/, hack/, main.go)
