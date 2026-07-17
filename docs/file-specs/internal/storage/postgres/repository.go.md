# File Spec: `internal/storage/postgres/repository.go`

## Status

`specified`

## Purpose

Implement the service store port with explicit parameterized SQL and domain mapping.

## Depends On

- PostgreSQL store, schema migration, domain models, and service `Store` interface.

## Public Surface

- Store methods required by experiment creation/listing, decision insert/feed, outcome acceptance, snapshot persistence/recovery, aggregates, OPE records, and simulation runs.

## Required Behavior

- Use explicit column lists and parameterized queries.
- Map nullable values intentionally and validate persisted JSON/domain projections. UUIDs use native PostgreSQL `UUID` parameters/columns with application-generated values.
- Create experiment, offers, and the version-one policy snapshot transactionally; policy configuration is never updated in place.
- Insert decisions atomically with immutable distributions.
- Accept outcomes by locking the experiment then decision in one consistent order, distinguishing exact retry and competing event, incrementing `experiments.policy_version`, and reserving that consecutive applied version. Do not reject an older decision selection version.
- Save snapshots idempotently for the same version/content and reject conflicts.
- Persist simulation status, counters, and observed/random/oracle reward sums atomically; terminal status and final totals share one transaction.
- Before startup readiness, atomically mark every persisted `starting`, `running`, or `stopping` run as `failed/process_restarted` while retaining partial totals.
- Use keyset cursors, bounded limits, deterministic ordering, maximum-120-point time-series aggregation, and aggregate SQL without N+1 queries.
- Permit no user-controlled SQL identifiers, order clauses, or fragments; every value is parameterized.

## Failure Cases

- Constraint conflicts, malformed stored JSON, version gaps, missing rows, cancellation, scan errors, or unexpected affected-row counts.

## Non-Goals

- Policy calculations, HTTP errors, logging request bodies, or dynamic SQL from user strings.

## Validation

- `go test -run TestStore -count=1 ./internal/storage/postgres`
- `go test -race ./internal/storage/postgres`

## Completion Checklist

- [ ] Every service port method is implemented explicitly.
- [ ] Idempotency/version transactions are tested concurrently.
- [ ] Queries are bounded and parameterized.
