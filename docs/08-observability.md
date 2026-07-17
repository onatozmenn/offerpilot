# Observability

## Structured Logging

Use Go `slog` with JSON output outside local development. Every request log includes:

- Timestamp and level.
- Request ID.
- Method, route template, status, and duration.
- Experiment, decision, or simulation-run ID when present.
- Stable error code.

Never log context bodies, distributions, database URLs, authorization headers, or environment contents. Policy updates log aggregate identifiers and versions, not full snapshots.

## Metrics

Prometheus-compatible metric names use the `offerpilot_` prefix.

Required metrics:

- `offerpilot_http_requests_total{route,method,status}`
- `offerpilot_http_request_duration_seconds{route,method}`
- `offerpilot_decisions_total{experiment,policy,offer}`
- `offerpilot_outcomes_total{experiment,outcome}`
- `offerpilot_reward_total{experiment,policy}`
- `offerpilot_policy_version{experiment}`
- `offerpilot_policy_updates_total{experiment,result}`
- `offerpilot_policy_update_duration_seconds{experiment}`
- `offerpilot_simulation_active{experiment}`
- `offerpilot_simulation_events_total{experiment,type}`
- `offerpilot_storage_operations_total{operation,result}`
- `offerpilot_storage_operation_duration_seconds{operation}`
- `offerpilot_recovery_replayed_outcomes_total{experiment}`

Experiment labels use bounded slugs created by the application. Request IDs, decision IDs, segment keys, and error text are prohibited metric labels.

## Health

Liveness checks only the process. Readiness checks database connectivity, schema compatibility, bootstrap completion, policy recovery, and absence of unapplied outcome gaps.

## Dashboard Statistics

Product statistics are computed from persisted data, not Prometheus. The summary endpoint reports sample counts and null reasons alongside estimates. Approximate percentiles are acceptable only if documented and tested.

## Tracing

OpenTelemetry tracing is deferred from the MVP implementation but package boundaries must allow HTTP and database instrumentation later. Do not add a tracing backend to Docker Compose initially.

## Operational Alerts

Document alert conditions even when no alert manager is shipped:

- Readiness failing for more than two minutes.
- Outcome accepted but policy snapshot version lagging.
- Policy output validation failure.
- Simulation error rate above the configured threshold.
- HTTP 5xx rate or p95 latency above the demo SLO.

## Demo SLOs

On a documented local machine and seeded workload:

- p95 decision endpoint latency below 100 ms at 50 requests per second.
- No lost or duplicated accepted outcomes.
- No data races under the test workload.

These are project targets, not measured claims until benchmark results are committed.
