# File Spec: `internal/service/engine_test.go`

## Status

`validated`

## Purpose

Verify engine orchestration, idempotency, versioning, summaries, cancellation, and recovery through narrow fakes.

## Depends On

- All `internal/service` implementation files and deterministic fake store/policy/clock types defined in this test file.

## Public Surface

- Behavior-focused tests grouped by engine operation.

## Required Behavior

- Cover successful create/decide/outcome paths, atomic initial snapshot, invalid policy output, persist-before-return, exact retry, competing event, delayed older-decision feedback, policy/store failures, applied versions ordered by acceptance under concurrency, random-policy version advancement, snapshot failure health, cancellation, bounded learning series/benchmark null reasons, and every recovery gap/corruption/crash-window case.
- Assert call ordering only where it is a contractual transaction/update requirement.

## Failure Cases

- Sleeps, real network/database use, uncontrolled goroutines, or tests coupled to private lock types.

## Non-Goals

- PostgreSQL SQL correctness or HTTP schema testing.

## Validation

- `go test -count=1 ./internal/service`
- `go test -race -count=10 ./internal/service`

## Completion Checklist

- [ ] Critical success and failure branches are covered.
- [ ] Concurrent tests are deterministic.
- [ ] Race validation passes.
