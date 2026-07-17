# File Spec: `internal/config/config_test.go`

## Status

`specified`

## Purpose

Prove environment defaults, exact variable mapping, origin parsing, bounds, and secret-safe errors.

## Depends On

- `internal/config/config.go` and `.env.example`.

## Public Surface

- Table-driven `TestLoad_` tests using isolated environment setup.

## Required Behavior

- Cover documented defaults and explicit overrides for every API-owned variable.
- Cover missing database URL, malformed numbers/durations/address, zero/negative/excessive simulation limits, invalid body/skew limits, and duplicate/wildcard/path/query/credential CORS origins.
- Assert errors name variables but never include database passwords or full secret values.
- Restore environment state between tests and avoid parallel tests that mutate process environment.

## Failure Cases

- Leaked environment state, platform-specific assumptions, or comparing complete secret-bearing errors.

## Non-Goals

- Vite-only environment handling or opening runtime resources.

## Validation

- `go test -run TestLoad -count=1 ./internal/config`
- `go test -race ./internal/config`

## Completion Checklist

- [ ] Every API variable/default has coverage.
- [ ] Bounds and CORS grammar are tested.
- [ ] Errors are secret-safe.
