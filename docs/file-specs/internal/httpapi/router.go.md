# File Spec: `internal/httpapi/router.go`

## Status

`specified`

## Purpose

Construct the Chi router, middleware chain, route registration, and HTTP server settings.

## Depends On

- Handlers, problem responses, config, observability middleware, readiness checks, and Prometheus handler.

## Public Surface

- Router/server constructor accepting explicit dependencies.

## Required Behavior

- Register every route from the API contract with stable route templates.
- Apply request ID, real-IP policy without trusting arbitrary proxies, recovery, structured access log, exact-origin credential-free CORS, timeout, content-type/configured-body-limit, and response metrics in a safe order.
- Expose liveness, readiness, and metrics with documented behavior.
- Configure read-header, read, write, and idle server timeouts.

## Failure Cases

- Duplicate/missing routes, wildcard credentialed CORS, panic detail leakage, unbounded body, or middleware high-cardinality labels.

## Non-Goals

- Business orchestration or JSON field mapping.

## Validation

- Router table tests plus `go test ./internal/httpapi`.

## Completion Checklist

- [ ] Contract routes and middleware order are tested.
- [ ] Operational endpoints behave independently.
- [ ] Timeouts and limits come from config.
