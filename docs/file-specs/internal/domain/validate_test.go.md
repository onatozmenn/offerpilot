# File Spec: `internal/domain/validate_test.go`

## Status

`specified`

## Purpose

Prove all domain validation, reward, segment, and probability invariants.

## Depends On

- `internal/domain/model.go`
- `internal/domain/validate.go`

## Public Surface

- Table-driven tests and deterministic property-style loops.

## Required Behavior

- Cover every valid enum and representative invalid value.
- Cover canonical segment ordering, reward mapping, epsilon bounds, `NaN`/infinity, negative latency/reward sums, duplicate actions, incomplete distributions, exact `1e-9` sum-tolerance boundaries, selected mismatch, and timestamp skew.
- Use reflection over approved context/domain structs to fail if future fields introduce a denylisted PII/protected concept such as name, email, phone, address, birth, gender, race, income, credit, device ID, or precise location.
- Include clear case names and avoid third-party assertion libraries.

## Failure Cases

- Tests accept silent defaults or compare unstable full error strings unnecessarily.

## Non-Goals

- Testing adapters or algorithms.

## Validation

- `go test -run Test -count=1 ./internal/domain`
- `go test -race ./internal/domain`

## Completion Checklist

- [ ] Success and failure branches are covered.
- [ ] Tests are deterministic.
- [ ] Race validation passes.
