# File Spec: `internal/service/engine.go`

## Status

`validated`

## Purpose

Own experiment, decision, and outcome use cases plus in-memory policy ownership.

## Depends On

- Domain, bandit, storage port behavior, clock/random injection, and observability hooks.

## Public Surface

- Narrow `Store` interface defined from service needs.
- `Engine` constructor and methods to create/list/get experiments, decide, record outcomes, and expose policy views.
- Package-level sentinel/category errors and small typed error structs defined in this file, suitable for adapter mapping without HTTP status imports.

## Required Behavior

- Build one policy instance per active experiment through an injected factory and persist its version-one snapshot atomically with new experiment/offers.
- Expose a separate internal decision command that may carry a manager-owned simulation-run ID; public adapters always omit it.
- Validate all domain inputs and active-offer conditions before selecting.
- Persist a validated decision before returning it.
- Serialize outcome accept/update/snapshot per experiment. Storage locks the experiment and reserves the next consecutive applied version; delayed feedback from an older decision selection version remains valid.
- Return exact retries without another update; reject competing terminal outcomes.
- Advance every policy kind, including random, to the reserved version; save the updated snapshot and mark policy health; expose unhealthy state to readiness.
- Report readiness only when every active persisted experiment has a loaded healthy policy.
- Honor context cancellation on all storage calls.

## Failure Cases

- Unknown/non-running experiment, invalid policy output, store failure, non-consecutive applied version, update/snapshot failure, duplicate/competing outcome, or unhealthy policy.

## Non-Goals

- HTTP DTOs, SQL, simulation outcome generation, aggregate queries, or startup replay implementation.

## Validation

- `go test -run 'TestEngine_(Create|Decide|RecordOutcome)' ./internal/service`
- `go test -race ./internal/service`

## Completion Checklist

- [ ] Decision and outcome semantics match API/domain contracts.
- [ ] Per-experiment update ordering is explicit.
- [ ] Idempotency and failures are tested.
