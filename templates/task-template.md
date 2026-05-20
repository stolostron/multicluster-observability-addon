# MCOA Structured Task

## Signal Type
<!-- metrics / logging / tracing / coo / analytics / core -->

## Layer
<!-- controllers / addon-core / signal-package / helm-charts / deploy -->

## Description
<!-- What this task does and why -->

## Files to Modify
- `<verified-path>` — <what changes>

## Files to Create
- `<verified-path>` — <purpose>

## Implementation Notes
Follow the existing pattern in `<FunctionName>()` at `<file-path>`.
Reuse `<TypeName>` from `<file-path>`.

## Acceptance Criteria
- [ ] <Testable criterion>
- [ ] <Regression check>

## Test Requirements
- [ ] Unit test in `<package>_test.go`
- [ ] Helm render test if chart templates changed

## Verification Commands
```bash
make lint
make test
go test ./...  # matches CI
```

## Commit Convention
```
feat|fix|test(<signal>): <description>
```

## Dependencies
- Depends on: <task or none>
