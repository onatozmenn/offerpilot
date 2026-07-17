# File Spec: `web/src/main.tsx`

## Status

`specified`

## Purpose

Initialize React, global styles/fonts, and the root application.

## Depends On

- `App.tsx`, `styles.css`, and packaged IBM Plex fonts.

## Public Surface

- Browser entry point mounted under React strict mode.

## Required Behavior

- Import font weights actually used and one global stylesheet.
- Validate the root element and fail with a clear developer error if absent.
- Render `App` under `StrictMode` without global mutable clients.

## Failure Cases

- Missing root, duplicate mounts, remote font fetch, or side-effectful API calls before render.

## Non-Goals

- Routing, data fetching, metrics computation, or layout markup.

## Validation

- Typecheck, unit tests, production build, and browser boot smoke test.

## Completion Checklist

- [ ] StrictMode boot works.
- [ ] Fonts/styles are local.
- [ ] No business logic enters the entry point.
