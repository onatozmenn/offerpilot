# File Spec: `internal/observability/observability.go`

## Status

`specified`

## Purpose

Construct structured logging, bounded Prometheus metrics, and instrumentation helpers.

## Depends On

- Config and `docs/08-observability.md`.

## Public Surface

- Logger constructor, `Metrics` registry wrapper, HTTP middleware hooks, and service/storage/simulation observation methods.

## Required Behavior

- Use `slog` text locally and JSON when configured without setting mutable global defaults.
- Register required metrics on an injected Prometheus registry and prevent duplicate registration.
- Use stable bounded labels only; expose helpers rather than raw vectors where that prevents misuse.
- Redact errors and never log contexts, distributions, secrets, or request bodies.

## Failure Cases

- Invalid log level/format, duplicate metric registration, high-cardinality label input, or nil dependencies.

## Non-Goals

- Product summary aggregation, tracing backend, or alert delivery.

## Validation

- Unit tests through handler/service tests and metric gathering assertions.
- `go test -race ./internal/observability ./internal/httpapi`

## Completion Checklist

- [ ] Required metrics exist with bounded labels.
- [ ] Logging is structured and redacted.
- [ ] Registry behavior is testable and race-safe.
