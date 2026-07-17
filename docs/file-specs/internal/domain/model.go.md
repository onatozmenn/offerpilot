# File Spec: `internal/domain/model.go`

## Status

`specified`

## Purpose

Define transport-independent domain enums, value objects, entities, and policy records.

## Depends On

- `docs/03-domain-model.md`.

## Public Surface

- Typed constants for statuses, policies, context enums, and outcomes.
- Structs for `Experiment`, `Offer`, `SessionContext`, `Decision`, `Outcome`, `PolicySnapshot`, `SimulationRun`, learning-series/benchmark projections, and action probabilities.

## Required Behavior

- Keep JSON/database tags out unless storage-neutral JSON is explicitly required for snapshot/context values.
- Use UUID and time types consistently.
- Include policy selection latency on decisions, consecutive applied version on outcomes, and observed/random/oracle reward sums on simulation runs.
- Keep entities as data plus small identity-free value behavior; validation lives in `validate.go`.

## Failure Cases

- PII fields, HTTP status knowledge, SQL row types, or untyped string maps replacing explicit domain fields.

## Non-Goals

- Persistence, API DTOs, policy algorithms, or logging.

## Validation

- Compile domain package and exercise construction through validation tests.

## Completion Checklist

- [ ] Every documented entity/enum exists once.
- [ ] No adapter dependencies enter the package.
- [ ] Domain tests compile and pass.
