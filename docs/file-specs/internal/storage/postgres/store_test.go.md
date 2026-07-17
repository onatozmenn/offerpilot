# File Spec: `internal/storage/postgres/store_test.go`

## Status

`validated`

## Purpose

Validate migrations, constraints, query mapping, transaction semantics, pagination, and recovery data against real PostgreSQL.

## Depends On

- Store/repository implementation, migrations, and Docker-accessible testcontainers-go.

## Public Surface

- Package integration tests with one reusable container per test package where isolation remains reliable.

## Required Behavior

- Apply migrations from empty state.
- Test experiment/offer/initial-snapshot atomicity, native UUID mapping, immutable decision inserts including policy latency, exact/competing outcomes, delayed selection versions, concurrently accepted outcomes with consecutive applied versions, snapshot conflicts and both crash windows, version-gap retrieval, aggregate/time-series/benchmark correctness, cursor pagination, simulation active-run uniqueness/terminal totals, and transaction cancellation.
- Run migration discovery from a working directory outside the repository to prove embedded-FS behavior.
- Verify interrupted-run startup reconciliation releases the active-run uniqueness constraint and preserves partial totals.
- Reset database state transactionally or with isolated schemas between tests.

## Failure Cases

- Tests passing against mocks only, order dependence, leaked containers/pools, or reliance on developer-local PostgreSQL.

## Non-Goals

- Policy quality or HTTP response tests.

## Validation

- `go test -run TestStore -count=1 ./internal/storage/postgres`
- `go test -race ./internal/storage/postgres`

## Completion Checklist

- [ ] Empty-database migrations pass.
- [ ] Concurrency/idempotency constraints are proven.
- [ ] Test resources always clean up.
