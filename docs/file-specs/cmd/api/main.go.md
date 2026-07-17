# File Spec: `cmd/api/main.go`

## Status

`validated`

## Purpose

Compose and run the OfferPilot API process with graceful startup and shutdown.

## Depends On

- Config, observability, PostgreSQL store, engine recovery, demo bootstrap, simulation manager, and HTTP router.

## Public Surface

- `main` command and process exit behavior.

## Required Behavior

- Load validated config before opening resources.
- Construct an explicit logger/metrics registry, database store, recovered engine, bootstrap state, simulation manager, and HTTP server.
- Run migrations, reconcile interrupted simulation runs, and recover policy state before readiness can succeed.
- Handle interrupt/termination signals, stop simulations, shut down HTTP within the configured deadline, close database resources, and return non-zero on startup/fatal shutdown errors.
- Keep wiring visible; do not hide composition in global init functions.

## Failure Cases

- Invalid config, migration/database failure, recovery gap, bootstrap failure, bind failure, or shutdown timeout.

## Non-Goals

- Domain logic, policy calculations, request handling, or environment-file loading.

## Validation

- `go test ./cmd/api ./internal/...`
- Start against Compose PostgreSQL, verify readiness, send termination, and verify clean exit.

## Completion Checklist

- [ ] Startup order enforces readiness guarantees.
- [ ] Shutdown releases all resources.
- [ ] No package globals own runtime state.
