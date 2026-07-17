# File Spec: `web/vite.config.ts`

## Status

`specified`

## Purpose

Configure Vite React development, test execution, and local API proxying.

## Depends On

- Frontend package and TypeScript configuration.

## Public Surface

- Vite config with React plugin, dev server, proxy, build, and Vitest settings.

## Required Behavior

- Proxy `/v1`, `/health`, and `/metrics` to `OFFERPILOT_API_PROXY_TARGET`, defaulting to `http://127.0.0.1:8080`, during development.
- Bind dev server to loopback by default and use a documented port.
- Configure jsdom tests, setup file, CSS handling, and coverage exclusions for entry/types.
- Do not inject a localhost fallback into production. An empty `VITE_API_BASE_URL` means same-origin; an explicit value must be an absolute `http`/`https` origin with no embedded credentials.

## Failure Cases

- Open network bind by default, hidden CORS workaround, proxying arbitrary paths, or test environment mismatch.

## Non-Goals

- Backend server configuration or deployment reverse proxy.

## Validation

- Typecheck `vite.config.ts` directly with the installed TypeScript/Vite/Node types before application source exists.
- Run full frontend typecheck, tests, and build after Phase 6 source exists.

## Completion Checklist

- [ ] Dev proxy reaches documented endpoints.
- [ ] Test environment loads setup.
- [ ] Production build succeeds.
