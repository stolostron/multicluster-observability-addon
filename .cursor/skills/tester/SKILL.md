---
name: mcoa-tester
description: >-
  Write comprehensive tests for MCOA implementations. Unit tests with testify,
  controller tests with fake client, Helm render tests.
---
# MCOA Tester

Write tests for multicluster-observability-addon.

## Test Framework

- **Standard**: `testing` + `testify` (assert/require)
- **Controller tests**: `controller-runtime` fake client
- **Addon fixtures**: `open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting`
- **Helm tests**: template rendering / snapshot tests

## Test Organization

All tests are colocated `*_test.go` next to source in `internal/`.

| Area | Examples |
|------|---------|
| Addon core | `internal/addon/*_test.go`, `internal/addon/common/*_test.go`, `internal/addon/helm/values_test.go` |
| Controllers | `internal/controllers/watcher/*_test.go` |
| Signal packages | `internal/logging/*_test.go`, `internal/metrics/*_test.go`, `internal/coo/*_test.go` |
| Analytics | `internal/analytics/rightsizing/*_test.go` |
| Helm rendering | `internal/logging/helm_test.go`, `internal/coo/helm_test.go` |

## Testing Patterns

1. **Find existing tests** in the same package — follow their patterns
2. **Table-driven tests** for functions with multiple inputs
3. **Fake client setup** for controller tests:
   ```go
   fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(...).Build()
   ```
4. **Addon testing fixtures** for managed cluster scenarios:
   ```go
   addontesting.NewManagedCluster("cluster1")
   ```
5. **Helm render tests**: render templates with different values, assert output

## Verification

- `make test` — runs `go test ./internal/...`
- `go test ./...` — full suite (matches CI)
- Check coverage against acceptance criteria

## Coverage Report

```
| Criterion | Test File | Test Function |
|-----------|-----------|---------------|
| ...       | ...       | ...           |
```
