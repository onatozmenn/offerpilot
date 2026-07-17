# File Spec: `go.mod`

## Status

`validated`

## Purpose

Define the Go module and direct runtime/test dependencies.

## Depends On

- Local Go 1.26.5 toolchain.
- All Go package specs.

## Public Surface

- Module path `github.com/onatozmenn/offerpilot`.
- Go language version `1.26`.

## Required Behavior

- Add only dependencies used by committed code.
- Expected direct families are Chi, pgx, UUID, Goose, Prometheus client, kin-openapi for contract tests, and testcontainers-go when their owning files are implemented.
- The file is intentionally reopened as later packages introduce imports; do not predeclare unused dependencies that `go mod tidy` would remove.
- Keep indirect dependencies tool-managed through `go mod tidy`.

## Failure Cases

- Unused direct modules, replace directives to local paths, or mismatched Go/Docker/CI versions.

## Non-Goals

- Tool source code or vendoring.

## Validation

- `go mod tidy`
- `go mod verify`
- `go list -m all`

## Completion Checklist

- [ ] Module path and Go version match contracts.
- [ ] Dependencies are minimal and verified.
- [ ] Generated `go.sum` is not manually edited.
