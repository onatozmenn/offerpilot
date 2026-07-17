# File Spec: `web/.dockerignore`

## Status

`specified`

## Purpose

Keep local dependencies, build output, tests, coverage, and environment secrets out of the frontend build context.

## Depends On

- Web Dockerfile and generated-files policy.

## Public Surface

- Docker build-context exclusion rules for `web`.

## Required Behavior

- Exclude `node_modules`, `dist`, coverage, `.env*`, test output, editor/OS state, and npm debug logs.
- Keep package manifests, TypeScript/Vite/ESLint config, `index.html`, source, and locally packaged dependency inputs needed by `npm ci` and build.

## Failure Cases

- Browser build secrets or host `node_modules` enter context, or required source/config is excluded.

## Non-Goals

- Root API context or runtime cache headers.

## Validation

- `docker build --no-cache -t offerpilot-web web`
- Inspect context size and served assets.

## Completion Checklist

- [ ] Local/generated paths are excluded.
- [ ] Lockfile-driven frontend build succeeds.
- [ ] No environment secrets enter assets.
