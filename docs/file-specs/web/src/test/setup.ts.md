# File Spec: `web/src/test/setup.ts`

## Status

`specified`

## Purpose

Install deterministic frontend test matchers and browser API shims required by components.

## Depends On

- Vitest/jsdom and Testing Library packages.

## Public Surface

- Global test setup executed once by Vitest.

## Required Behavior

- Import DOM matchers and install minimal deterministic `matchMedia`, `ResizeObserver`, and other chart-required shims only when jsdom lacks them.
- Reset mocks/timers and cleanup DOM between tests through framework-supported hooks.
- Keep shims behaviorally neutral and typed.

## Failure Cases

- Hiding application errors with broad mocks, leaking fake timers, or modifying production globals outside tests.

## Non-Goals

- API fixtures, component rendering helpers, or production polyfills.

## Validation

- `npm --prefix web run test`

## Completion Checklist

- [ ] Tests run without environment warnings.
- [ ] Cleanup prevents cross-test state.
- [ ] Shims remain minimal.
