# File Spec: `internal/bandit/random_test.go`

## Status

`validated`

## Purpose

Verify uniform distribution, deterministic sampling, and baseline lifecycle behavior.

## Depends On

- `internal/bandit/random.go`.

## Public Surface

- Tests named with the `TestRandomPolicy_` prefix.

## Required Behavior

- Test two and many actions, canonical ordering, identical seeded sequences, invalid candidates, delayed-decision valid updates, non-consecutive update rejection, version advancement with unchanged distribution, and snapshot round trip/corruption.
- Assert direct probability validation at and beyond the `1e-9` sum tolerance where shared validators are invoked.
- Run concurrent selections under the race detector.

## Failure Cases

- Statistical flaky assertions or assumptions about map iteration order.

## Non-Goals

- Measuring policy quality.

## Validation

- `go test -run TestRandomPolicy -count=10 ./internal/bandit`
- `go test -race ./internal/bandit`

## Completion Checklist

- [ ] Tests are deterministic and non-statistical.
- [ ] Invalid inputs are covered.
- [ ] Race validation passes.
