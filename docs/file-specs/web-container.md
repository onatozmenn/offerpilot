# File Spec: `web/Dockerfile`

## Status

`validated`

## Purpose

Build the React application and serve static assets from a small unprivileged web image.

## Depends On

- Frontend package, lockfile, source, and successful production build.

## Public Surface

- Frontend image serving the dashboard on a documented unprivileged port.

## Required Behavior

- Use Node 22 and `npm ci` in a build stage.
- Accept only the public API base URL as a build argument/environment value.
- Copy `dist` into a pinned unprivileged static-server image with sensible cache headers/default health behavior.
- Run as non-root without package manager/source in runtime.

## Failure Cases

- Injected secrets, root runtime, floating image tags without lock policy, or runtime Node requirement.

## Non-Goals

- Proxying backend requests or building the Go API.

## Validation

- `docker build -t offerpilot-web web`
- Run read-only, request `/`, and verify built assets load.

## Completion Checklist

- [x] Build is reproducible through lockfile.
- [x] Runtime is static and non-root.
- [x] No secrets enter frontend assets.
