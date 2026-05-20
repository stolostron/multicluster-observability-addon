---
name: mcoa-ci-fixer
description: >-
  Diagnose and fix CI failures on MCOA PRs. Handles golangci-lint, test failures,
  build errors, and container image issues.
---
# MCOA CI Fixer

Fix CI failures on multicluster-observability-addon PRs.

## MCOA CI Pipeline

GitHub Actions (`checks.yaml`):
1. **Lint**: `golangci-lint` v2.11.4 with `--timeout=4m`
2. **Test**: `go test ./...` with coverage → Coveralls
3. **Build** (`build.yaml`): `make addon`
4. **Images** (`images.yaml`): multi-arch buildx (amd64 + arm64)

Konflux/Tekton: container image build via `Dockerfile.Konflux`.

## Common Failures

| Failure | Fix |
|---------|-----|
| golangci-lint error | Fix the lint issue (check `.golangci.yml` for enabled linters) |
| err113 / errorlint | Use `fmt.Errorf("...: %w", err)` or define sentinel errors |
| testifylint | Use correct testify assertion pattern |
| modernize | Use modern Go idioms (1.25+) |
| gci import order | Run goimports/gci formatter |
| Test failure | Read test output, fix code or test expectation |
| Build failure | Fix compilation error |
| Dockerfile label | Fix labels in Dockerfile / Dockerfile.Konflux |

## Fix Workflow

1. Read CI failure logs via `gh` CLI
2. Identify the failing step (lint/test/build)
3. Apply minimal fix
4. Run locally: `make lint` then `go test ./...`
5. Commit and push
6. Re-check CI
7. Max 5 attempts, then escalate

## Note on Lint Config

The `.golangci.yml` has a known typo in the `gci` prefix (`obsevability` instead of `observability`, `multi-cluster` instead of `multicluster`). If you see import ordering issues, this may be the cause — work around it, don't fix config unless asked.
