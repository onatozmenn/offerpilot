# File Spec: `cmd/simulator/main.go`

## Status

`specified`

## Purpose

Run a reproducible external traffic simulation against the public HTTP API.

## Depends On

- Simulation HTTP client, profiles, and runner.

## Public Surface

- CLI flags for base URL, experiment ID, seed, requests per second, max decisions, timeout, and output format.

## Required Behavior

- Parse and validate flags without interactive prompts.
- Use signal-aware context cancellation and bounded request timeout.
- Print seed/config before running and a final machine-readable or human summary.
- Exit non-zero on invalid input, unreachable API, or terminal run failure; cancellation by user exits cleanly with partial counts.

## Failure Cases

- Invalid UUID/rate/count, API problem response, repeated transport failure, or context deadline.

## Non-Goals

- Opening PostgreSQL, importing service internals, or controlling in-process dashboard runs.

## Validation

- `go run ./cmd/simulator -help`
- Run against `httptest`/local API with a fixed seed and compare summary.

## Completion Checklist

- [ ] CLI is non-interactive and deterministic.
- [ ] Errors produce actionable output and exit codes.
- [ ] It depends only on the public API boundary.
