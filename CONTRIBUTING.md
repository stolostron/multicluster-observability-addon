# Contributing

## Development Setup

1. Install Go 1.25+
2. Clone the repo and run `make download-crds` (fetches upstream CRDs)
3. Build: `make addon`
4. Lint: `make lint`
5. Test: `go test ./...`

## Running Tests

```bash
make test                    # unit tests (internal/ only)
go test ./...                # full suite (matches CI)
go test ./internal/analytics/rightsizing/...  # rightsizing only
go test ./internal/coo/...   # E2E render tests
```

## Submitting Changes

1. Fork the repo and create a branch from `main`.
2. All production code goes under `internal/` — nothing exported.
3. Use `testify` for assertions (`assert` for non-fatal, `require` for fatal).
4. Helm chart changes go in `internal/addon/manifests/charts/mcoa/`.
5. Run `make lint` before submitting PRs.
6. Open a pull request with a clear description of what changed and why.

## Code Style

- Go 1.25+ idioms; favor standard library
- Import groups: stdlib, external, internal (`gci`/`gofumpt`/`goimports` enforced)
- Error handling: wrap with `%w`, handle once (log OR return, not both)
- Lint config: `.golangci.yml` v2 — `err113`, `errorlint`, `revive`, `testifylint`, `modernize`

## Containerfiles

- `Dockerfile` — distroless, CGO_ENABLED=0 (community)
- `Dockerfile.Konflux` — UBI minimal, CGO_ENABLED=1 (FIPS), Red Hat labels

## Project Documentation

See [docs/agents/](docs/agents/) for detailed architecture and subsystem documentation.
