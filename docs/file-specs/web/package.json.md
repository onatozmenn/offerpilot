# File Spec: `web/package.json`

## Status

`validated`

## Purpose

Define the frontend package, scripts, runtime dependencies, and development toolchain.

## Depends On

- Node 22.16.0 and npm 10.9.2.
- Frontend file specs and `docs/10-frontend.md`.

## Public Surface

- Scripts `dev`, `build`, `preview`, `lint`, `typecheck`, `test`, and `test:coverage`.

## Required Behavior

- Mark package private and use ESM.
- Include React, Recharts, Lucide React, and locally packaged IBM Plex Sans/Mono fonts as runtime dependencies.
- Pin a React 19-compatible Recharts release whose published declarations pass strict typechecking without `skipLibCheck`.
- Include Vite, TypeScript, `@types/node`, ESLint flat-config plugins, Vitest, jsdom, and Testing Library as development dependencies.
- Pin compatible versions through `package-lock.json`; do not use runtime wildcard ranges.

## Failure Cases

- Missing CI script, duplicate formatting stack, server-only secret package, or unnecessary state-management framework.

## Non-Goals

- Backend scripts, deployment publishing, or generated API clients.

## Validation

- `npm install --prefix web`
- `npm --prefix web pkg get scripts`
- Full lint/typecheck/test/build scripts become release gates after Phase 6 source exists.

## Completion Checklist

- [ ] Scripts match CI/testing docs.
- [ ] Dependencies are minimal and compatible.
- [ ] Generated lockfile is committed unedited.
