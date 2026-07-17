# File Spec: `migrations/000001_initial.down.sql`

## Status

`specified`

## Purpose

Remove migration version 1 cleanly for local development and migration tests.

## Depends On

- `migrations/000001_initial.up.sql`.

## Public Surface

- Reverse-order table/index/type removal for the initial schema.

## Required Behavior

- Drop dependent objects in safe reverse order inside a transaction.
- Remove only objects owned by migration 1.
- Permit up, down, and up again on an otherwise empty local database.

## Failure Cases

- Dropping shared extensions/objects, relying on broad `CASCADE`, or leaving schema residue.

## Non-Goals

- Preserving local data during rollback or supporting production downgrade.

## Validation

- Goose up/down/up cycle against testcontainers PostgreSQL followed by schema tests.

## Completion Checklist

- [ ] Reverse migration is scoped and complete.
- [ ] Up/down/up succeeds.
- [ ] No broad destructive cascade is used.
