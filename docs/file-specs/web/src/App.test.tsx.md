# File Spec: `web/src/App.test.tsx`

## Status

`validated`

## Purpose

Verify dashboard integration, API client behavior, polling lifecycle, controls, components, accessibility states, and partial failures.

## Depends On

- All frontend source files and test setup.

## Public Surface

- User-behavior tests using Vitest and Testing Library.

## Required Behavior

- Cover initial load, empty/create-demo flow, experiment selection, running/completed simulation, start/stop validation and duplicate prevention, polling with fake timers, abort/stale-response race, unmount cleanup, partial summary/feed failure, stale data preservation, API problem display, metric/benchmark null reasons, bounded cumulative-series chart labels, offer table, and pending/terminal decision rows with distinct selection/applied versions.
- Query by roles/labels/text rather than class names and assert important keyboard interactions.
- Use typed fixtures matching OpenAPI and controlled fetch promises.

## Failure Cases

- Snapshot-only coverage, real timers/network, implementation-detail queries, act warnings, or unhandled promise rejections.

## Non-Goals

- Pixel-perfect browser layout, Go API correctness, or exhaustive chart-library internals.

## Validation

- `npm --prefix web run test -- --run`
- `npm --prefix web run typecheck`
- `npm --prefix web run lint`

## Completion Checklist

- [x] Critical workflows and partial states are covered.
- [x] Tests are deterministic and accessible-query driven.
- [x] Lint/typecheck/test pass without warnings.
