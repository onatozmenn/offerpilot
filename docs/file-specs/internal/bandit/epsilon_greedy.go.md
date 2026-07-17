# File Spec: `internal/bandit/epsilon_greedy.go`

## Status

`validated`

## Purpose

Implement the segmented epsilon-greedy online policy with exact propensities and versioned snapshots.

## Depends On

- `internal/bandit/policy.go`
- `docs/05-online-learning.md`.

## Public Surface

- Constructor accepting epsilon, priors, initial version, and seeded random source.
- `Policy` implementation with kind `segmented_epsilon_greedy`.

## Required Behavior

- Lazily initialize every segment-offer statistic with count 2 and reward sum 1.
- Compute tied best arms and the exact documented probability distribution.
- Sample deterministically for a fixed seed and ordered inputs.
- Update only the selected segment-offer once when the reserved applied version is current plus one, regardless of an older valid decision selection version, then set the reserved version.
- Protect state with minimal locking and deep-copy snapshots.
- Encode snapshots with an explicit schema version and deterministic ordering.

## Failure Cases

- Invalid epsilon/reward, non-consecutive applied version, experiment/policy mismatch, unknown selected action, malformed state, non-finite values, duplicate actions, or snapshot version regression.

## Non-Goals

- LinUCB, decay, delayed feedback, database persistence, or distributed locks.

## Validation

- `go test -run TestEpsilonGreedy -count=1 ./internal/bandit`
- `go test -race ./internal/bandit`

## Completion Checklist

- [ ] Formula and tie behavior match the design document.
- [ ] Updates/versioning are exactly-once at the policy boundary.
- [ ] Snapshot and concurrency tests pass.
