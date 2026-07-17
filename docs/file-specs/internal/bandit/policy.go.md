# File Spec: `internal/bandit/policy.go`

## Status

`specified`

## Purpose

Define the algorithm boundary and shared selection/update/snapshot types.

## Depends On

- Domain model and `docs/05-online-learning.md`.

## Public Surface

- `Policy` interface with `Select`, `Update`, `Snapshot`, `Restore`, `Kind`, and `Version` behavior.
- Immutable selection input/output and update records.
- Injectable concurrency-safe random-number abstraction or constructor contract.

## Required Behavior

- Return a complete action distribution and selected action.
- Keep context canonical and actions deterministically ordered.
- Carry both the decision's selection version for audit and storage's reserved applied version. Accept delayed selection versions but require `applied_version == current_version + 1`.
- Define snapshot schema version as integer `1`; unknown integers fail closed.
- Keep interface free of HTTP/storage types.

## Failure Cases

- Empty action set, duplicate actions, non-consecutive applied version, policy/experiment mismatch, invalid snapshot schema, or random source failure where applicable.

## Non-Goals

- Policy registry, database persistence, or metric emission.

## Validation

- Compile both policy implementations and their tests.
- `go test -race ./internal/bandit`

## Completion Checklist

- [ ] Interface supports both policies without type switches in callers.
- [ ] Inputs/outputs preserve audit data.
- [ ] Focused tests pass.
