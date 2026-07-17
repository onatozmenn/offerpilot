# File Spec: `internal/bandit/epsilon_greedy_test.go`

## Status

`specified`

## Purpose

Prove epsilon-greedy probability, learning, concurrency, and persistence invariants.

## Depends On

- `internal/bandit/epsilon_greedy.go`.

## Public Surface

- Deterministic tests named with the `TestEpsilonGreedy_` prefix.

## Required Behavior

- Cover epsilon `0`, `1`, and a middle value; all-tie and partial-tie cases; cold-start priors; multiple segments; fractional rewards; delayed decisions, non-consecutive and duplicate updates; version increments; seeded sequences; snapshot round trip; corrupt/unknown integer snapshot schema.
- Run concurrent selection and update loops under `-race`.
- Property-style loops assert finite complete distributions summing to one and explicit behavior at/beyond `1e-9` tolerance.

## Failure Cases

- Flaky aggregate-frequency thresholds or tests coupled to lock implementation.

## Non-Goals

- HTTP, SQL, or long-running performance benchmarks.

## Validation

- `go test -run TestEpsilonGreedy -count=10 ./internal/bandit`
- `go test -race ./internal/bandit`

## Completion Checklist

- [ ] Every formula branch is asserted directly.
- [ ] Snapshot/version failures are covered.
- [ ] Race validation passes.
