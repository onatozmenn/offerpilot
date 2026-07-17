# File Spec: `internal/bootstrap/demo_test.go`

## Status

`specified`

## Purpose

Verify demo catalog validity, idempotent startup, fresh creation, and privacy/trademark constraints.

## Depends On

- `internal/bootstrap/demo.go` and fake engine creator.

## Public Surface

- `TestDemo_` behavior tests.

## Required Behavior

- Assert six unique offer slugs/IDs per experiment, category/profile compatibility, no prohibited context fields, valid policy defaults, concurrent ensure idempotency, and distinct fresh experiment identity.
- Keep a denylist test for brands intentionally avoided in demo fixtures.

## Failure Cases

- Snapshotting incidental UUID order or relying on external services.

## Non-Goals

- Testing SQL uniqueness or simulator probabilities.

## Validation

- `go test -run TestDemo -count=10 ./internal/bootstrap`
- `go test -race ./internal/bootstrap`

## Completion Checklist

- [ ] Catalog and safety constraints are explicit.
- [ ] Concurrent idempotency is tested.
- [ ] Race validation passes.
