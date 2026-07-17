# File Spec: `internal/domain/validate.go`

## Status

`specified`

## Purpose

Enforce domain invariants and canonical transformations.

## Depends On

- `internal/domain/model.go`.
- Domain and learning contracts.

## Public Surface

- Validation methods/functions for experiment configuration, offers, session context, distributions, outcomes, and timestamps.
- `SegmentKey(SessionContext) (string, error)`.
- `RewardForOutcome(OutcomeKind) (float64, error)`.

## Required Behavior

- Reject unknown enums, empty IDs/slugs, non-finite numbers, invalid epsilon/propensity, negative policy latency/reward sums, incomplete distributions, probability sums outside `1e-9`, and selected-propensity mismatch.
- Sort or require canonical offer order without mutating caller-owned slices unexpectedly.
- Derive rewards solely from the documented table.

## Failure Cases

- `NaN`, infinity, duplicate/missing offers, future occurrence beyond allowed skew, or protected context keys entering generic maps.

## Non-Goals

- HTTP-friendly error formatting or database constraints.

## Validation

- `go test ./internal/domain`
- `go test -race ./internal/domain`

## Completion Checklist

- [ ] Every global invariant has a testable guard.
- [ ] Errors identify the invalid field/class.
- [ ] Focused tests pass.
