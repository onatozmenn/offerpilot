# File Spec: `migrations/000001_initial.up.sql`

## Status

`validated`

## Purpose

Create the complete initial PostgreSQL schema and constraints described by the data model.

## Depends On

- `docs/06-data-model.md`.

## Public Surface

- Tables, constraints, indexes, and partial unique active-run index for migration version 1.

## Required Behavior

- Use PostgreSQL native `UUID` columns with application-generated values; do not install `uuid-ossp` or `pgcrypto` merely for UUID storage.
- Create experiments, offers, simulation runs, decisions, outcomes, and policy snapshots in foreign-key order using native PostgreSQL `UUID` identifiers.
- Add policy latency to decisions and observed/random/oracle reward sums to simulation runs.
- Add checked enums/ranges/finite-value guards, uniqueness, immutable-history-oriented constraints, timestamps, JSON top-level checks, and documented query indexes.
- Run transactionally and remain free of demo rows.

## Failure Cases

- Missing idempotency/version constraints, floating non-finite acceptance where PostgreSQL checks can prevent it, unindexed foreign keys/query paths, or destructive statements.

## Non-Goals

- Seed data, retention jobs, or future algorithms.

## Validation

- Apply with Goose to an empty PostgreSQL container and run storage integration tests.

## Completion Checklist

- [ ] Schema matches every documented table/invariant.
- [ ] Migration is transactional and empty-data only.
- [ ] Integration tests pass.
