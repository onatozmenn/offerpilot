# File Spec: `internal/httpapi/handlers_test.go`

## Status

`specified`

## Purpose

Verify router, middleware, DTO, problem, pagination, health, and handler contracts through `httptest`.

## Depends On

- All HTTP adapter files and deterministic fake engine/simulation dependencies.

## Public Surface

- Route-level tests organized by endpoint and middleware behavior.

## Required Behavior

- Cover every success status/schema and representative service error mapping.
- Test malformed, empty, multiple, unknown-field, wrong-content-type, and oversized bodies.
- Test invalid UUID/cursor/limit/timestamp/future skew, exact retry status, location/request ID headers, exact-origin CORS, panic recovery, liveness/readiness, and stable route metric labels.
- Capture structured logs for malformed/internal-error requests containing canary body/secret strings and assert that raw bodies, database details, headers, and canaries never appear.
- Validate summary learning-series bounds/order, simulation benchmark null reasons, and selection/applied version projections.
- Load `openapi/openapi.yaml` with kin-openapi and validate representative requests/responses for every operation without network access.
- Ensure cancellation reaches fake dependencies.

## Failure Cases

- Starting a real server/database, brittle full JSON string equality, or missing negative cases.

## Non-Goals

- Domain algorithm or SQL correctness.

## Validation

- `go test -run 'Test(Router|Handlers|Problems)' -count=1 ./internal/httpapi`
- `go test -race ./internal/httpapi`

## Completion Checklist

- [ ] Every endpoint contract is represented.
- [ ] Security/limit failures are covered.
- [ ] Tests pass under the race detector.
