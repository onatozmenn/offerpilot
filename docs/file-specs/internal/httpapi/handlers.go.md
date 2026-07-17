# File Spec: `internal/httpapi/handlers.go`

## Status

`validated`

## Purpose

Implement all JSON API handlers as thin adapters over engine and simulation use cases.

## Depends On

- Router, DTOs, problem writer, engine interface, simulation manager interface, and API contract.

## Public Surface

- Handler set constructor and route methods for demo experiments, experiment list/get/summary/feed, decisions, outcomes, and simulation start/get/stop.

## Required Behavior

- Decode exactly one bounded JSON object with unknown-field rejection.
- Parse UUIDs, cursors, limits, and timestamps explicitly.
- Pass request context to every use case.
- Return documented statuses, headers, DTOs, and problems.
- Avoid business calculations; reward, propensity, summary, and status logic remain in services.

## Failure Cases

- Empty/multiple/malformed bodies, wrong content type, invalid path/query values, service conflicts/unavailability, encoding failure, or client cancellation.

## Non-Goals

- SQL, policy state, background goroutine ownership, or frontend rendering.

## Validation

- `go test -run TestHandlers -count=1 ./internal/httpapi`
- OpenAPI representative contract tests.

## Completion Checklist

- [ ] Every API route has success and failure tests.
- [ ] Handlers remain transport-only.
- [ ] Status/header/schema behavior matches OpenAPI.
