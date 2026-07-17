# File Spec: `.github/workflows/ci.yml`

## Status

`validated`

## Purpose

Run deterministic backend, frontend, documentation, security, and container checks on pull requests and main.

## Depends On

- All build/lint/test commands documented in `docs/09-testing.md`.
- Dependency lock files.

## Public Surface

- GitHub Actions workflow with least-privilege permissions and concurrency cancellation.

## Required Behavior

- Pin major action versions and Go 1.26/Node 22.
- Separate Go and frontend jobs for clear failures.
- Run formatting checks, vet, race tests, linter, frontend lint/typecheck/test/build, Markdown/spec validation, and Compose configuration.
- Cache only tool-managed dependencies.
- Run the official `govulncheck ./...` and fail on reachable known Go vulnerabilities.
- Run `npm audit --prefix web --audit-level=high` and fail on high or critical production/development dependency advisories unless a time-bounded documented exception is committed.

## Failure Cases

- Writing repository contents, exposing secrets, skipping race tests, or using unpinned floating runtimes.

## Non-Goals

- Deployment or publishing images in the MVP.

## Validation

- Validate workflow syntax locally where tooling is available and confirm every command runs in PowerShell/local equivalents.

## Completion Checklist

- [x] Least-privilege workflow is valid.
- [x] All required gates run.
- [x] No deployment credentials are required.
