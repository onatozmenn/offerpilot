# File Spec: `internal/storage/postgres/store.go`

## Status

`specified`

## Purpose

Own PostgreSQL pool lifecycle, migration execution, health checks, and transaction helpers.

## Depends On

- Config, pgx pool, Goose migrations, and initial migration files.

## Public Surface

- `Store` type, `Open`, `Migrate`, `Ping`, `Close`, and internal transaction helper.

## Required Behavior

- Parse and validate the database URL through pgx config.
- Configure bounded pool sizes, lifetimes, idle time, and health-check period from config; honor caller context deadlines while acquiring connections.
- Read the split up/down SQL bytes from the root `migrations` embedded filesystem and register them as one transaction-enabled migration through Goose's provider API; do not scan split files as duplicate versions, use current-working-directory lookup, or mutate Goose process globals.
- Wrap transaction begin/rollback/commit with context and preserve root errors.
- Expose no raw pool outside the package.

## Failure Cases

- Invalid URL, connection/ping failure, incompatible migration, cancelled transaction, commit failure, or double close.

## Non-Goals

- Domain queries or business transaction decisions.

## Validation

- `go test -run TestStore ./internal/storage/postgres`
- Open/migrate/ping against testcontainers PostgreSQL.

## Completion Checklist

- [ ] Pool and migration lifecycle works from an empty database.
- [ ] Context and errors are preserved.
- [ ] Resources close cleanly.
