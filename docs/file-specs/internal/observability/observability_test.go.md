# File Spec: `internal/observability/observability_test.go`

## Status

`validated`

## Purpose

Verify logger format/redaction and bounded Prometheus registration/labels.

## Depends On

- `internal/observability/observability.go`.

## Public Surface

- `TestLogger_` and `TestMetrics_` tests using in-memory buffers and isolated registries.

## Required Behavior

- Cover valid/invalid log levels and formats, JSON/text output, stable fields, and absence of supplied secret/body/context values.
- Gather every required metric, verify allowed labels, reject/avoid high-cardinality fields, and prove separate registries do not conflict.
- Exercise concurrent observation under the race detector without depending on metric output order.

## Failure Cases

- Global logger/registry pollution, brittle serialized ordering, or secret values in failure output.

## Non-Goals

- HTTP route behavior, product aggregates, or external Prometheus server.

## Validation

- `go test -run 'Test(Logger|Metrics)' -count=1 ./internal/observability`
- `go test -race ./internal/observability`

## Completion Checklist

- [ ] Logging behavior and redaction are covered.
- [ ] Metrics names/labels match the observability contract.
- [ ] Race validation passes.
