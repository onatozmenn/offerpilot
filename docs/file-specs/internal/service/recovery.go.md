# File Spec: `internal/service/recovery.go`

## Status

`validated`

## Purpose

Restore every active experiment policy and replay accepted outcomes not present in its latest snapshot.

## Depends On

- Store recovery queries, policy factory/snapshot behavior, domain versions, and engine policy registry.

## Public Surface

- `Engine.Recover(ctx)` or constructor-level recovery operation and recovery report.

## Required Behavior

- Load active experiments in deterministic order.
- Restore the latest compatible snapshot or initialize only a brand-new version-one experiment.
- Replay unapplied outcomes by consecutive applied version, regardless of their older decision selection versions. Random policies advance version through the same replay path.
- Reject gaps, duplicates, unknown snapshot schema, and experiment/policy mismatch.
- Persist one final checkpoint after replay and install policies atomically only after all required state is valid. Snapshot insertion is idempotent only when schema, kind, version, and bytes match exactly.

## Failure Cases

- Missing snapshot for a non-new experiment, corrupt state, version gap, replay update failure, cancellation, or store failure.

## Non-Goals

- Best-effort partial readiness or random fallback without explicit experiment configuration.

## Validation

- `go test -run TestEngine_Recover ./internal/service`
- Restart integration scenarios against PostgreSQL, including a crash after in-memory update but before snapshot persistence and a crash after snapshot commit but before response.

## Completion Checklist

- [ ] Replay is deterministic and consecutive.
- [ ] Partial/corrupt state fails readiness.
- [ ] Recovery tests pass.
