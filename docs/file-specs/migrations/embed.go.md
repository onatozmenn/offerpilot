# File Spec: `migrations/embed.go`

## Status

`specified`

## Purpose

Expose migration SQL as an immutable embedded filesystem for local, test, and container execution.

## Depends On

- Initial up/down SQL migrations and Go standard `embed` package.

## Public Surface

- Package `migrations` and exported read-only `FS` containing `*.sql` migration files.

## Required Behavior

- Use `//go:embed *.sql` directly above an `embed.FS` variable.
- Embed only migration SQL from this directory and contain no init/global mutation.
- Allow Goose's provider API to consume the same bytes in every runtime.

## Failure Cases

- Empty pattern, path traversal, embedding docs/secrets, or mutable process-global setup.

## Non-Goals

- Running migrations or defining schema in Go.

## Validation

- `go test ./migrations ./internal/storage/postgres`
- Storage integration test confirms migration discovery from a non-root working directory.

## Completion Checklist

- [ ] Both initial SQL files are embedded.
- [ ] Package has no runtime side effects.
- [ ] Non-root-CWD migration test passes.
