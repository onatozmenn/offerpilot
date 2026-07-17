# File Spec: `internal/bandit/random.go`

## Status

`validated`

## Purpose

Implement the uniform random baseline policy.

## Depends On

- `internal/bandit/policy.go`
- Domain distribution validation.

## Public Surface

- Constructor accepting an injected seeded random source.
- `Policy` implementation with kind `random`.

## Required Behavior

- Assign exactly `1/K` probability to each sorted eligible offer.
- Sample through the same distribution mechanism used by tests.
- Accept each valid consecutive update without changing the distribution, advance the application version once, and reject structurally invalid or non-consecutive updates.
- Snapshot/restore integer schema version, policy kind, and current application version only.

## Failure Cases

- Fewer than two actions, duplicates, nil random source, or malformed snapshot.

## Non-Goals

- Learning, weighted offers, or cryptographic randomness.

## Validation

- `go test -run TestRandom -count=1 ./internal/bandit`
- `go test -race ./internal/bandit`

## Completion Checklist

- [ ] Distribution is exactly uniform and complete.
- [ ] Seeded selection is reproducible.
- [ ] Snapshot failures are tested.
