# File Spec: `Dockerfile`

## Status

`specified`

## Purpose

Build and run the Go API in a small non-root container.

## Depends On

- `go.mod`, `go.sum`, API command, and migrations.

## Public Surface

- Runtime image exposing the configured API port and containing migration files.

## Required Behavior

- Use a Go 1.26 multi-stage build with dependency caching.
- Build with `CGO_ENABLED=0` when all selected dependencies permit it.
- Run as an unprivileged user in the final image.
- Copy only the API binary and CA certificates needed at runtime; migration SQL is embedded in the binary.
- Include a health check or allow Compose to call the readiness endpoint.

## Failure Cases

- Compiler/toolchain in final image, root runtime, embedded secrets, or missing CA certificates/migrations.

## Non-Goals

- Building the frontend or running PostgreSQL.

## Validation

- `docker build -t offerpilot-api .`
- Run the image read-only against a test database and call readiness.

## Completion Checklist

- [ ] Multi-stage non-root build works.
- [ ] Runtime contents are minimal.
- [ ] Readiness succeeds with valid configuration.
