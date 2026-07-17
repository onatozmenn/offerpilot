# File Spec: `.dockerignore`

## Status

`validated`

## Purpose

Keep secrets, frontend/development output, VCS data, and irrelevant documentation out of the API build context.

## Depends On

- Root Dockerfile and generated-files policy.

## Public Surface

- Docker build-context exclusion rules for the repository root.

## Required Behavior

- Exclude `.git`, `.github`, `.env*` except safe examples where needed, `docs`, `web`, coverage/profiles, binaries, editor state, and local volumes.
- Keep Go source, `go.mod`, `go.sum`, and root `migrations` available to the build stage.
- Never exclude files needed by `go build ./cmd/api`.

## Failure Cases

- Secret files enter build context or required Go/migration files disappear.

## Non-Goals

- Git ignore behavior or frontend context rules.

## Validation

- `docker build --no-cache -t offerpilot-api .`
- Inspect build context size and final image contents without printing secrets.

## Completion Checklist

- [x] Sensitive/irrelevant paths are excluded.
- [x] API image still builds from clean source.
- [x] Migration embed inputs remain present.
