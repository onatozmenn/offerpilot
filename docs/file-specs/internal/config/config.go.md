# File Spec: `internal/config/config.go`

## Status

`validated`

## Purpose

Load, default, and validate runtime configuration from environment variables.

## Depends On

- `.env.example` specification.
- `docs/07-security-privacy.md` limits.

## Public Surface

- `Config` grouped into HTTP, Database, Logging, CORS, Shutdown, and Simulation settings.
- `Load() (Config, error)`.
- Environment names and defaults exactly match `.env.example`: body limit 1 MiB, shutdown 15 seconds, future-outcome skew 2 minutes, maximum 100 simulation requests/second, 100,000 decisions, 32 workers, 30-minute duration, and 100 errors.
- Database pool defaults are max 10/min 1 connections, 30-minute maximum lifetime, 5-minute maximum idle time, and 30-second health-check period; HTTP timeout defaults are 5/10/30/60 seconds for read-header/read/write/idle.

## Required Behavior

- Use `os.LookupEnv` and standard parsing; do not silently load `.env` files.
- Apply safe defaults for local development.
- Require a database URL and validate durations, pool min/max relationships, ports, origins, positive limits, and rate ceilings.
- Parse `OFFERPILOT_CORS_ALLOWED_ORIGINS` as comma-separated absolute `http`/`https` origins with no paths, queries, fragments, duplicates, wildcard, or credentials. CORS credentials remain disabled.
- Treat `OFFERPILOT_API_PROXY_TARGET` and `VITE_API_BASE_URL` as frontend-owned documentation values rather than API runtime config.
- Return one contextual error identifying the invalid variable without echoing secret values.

## Failure Cases

- Missing required database URL, malformed duration/number, wildcard credentialed CORS, or unsafe simulation bounds.

## Non-Goals

- Opening network/database connections or configuring loggers.

## Validation

- Table-driven tests in `internal/config/config_test.go` for defaults and every invalid class.
- `go test ./internal/config`

## Completion Checklist

- [ ] Keys match `.env.example`.
- [ ] Defaults and failures are tested.
- [ ] Secrets never appear in errors.
