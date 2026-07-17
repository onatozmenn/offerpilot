# File Spec: `web/src/types/api.ts`

## Status

`validated`

## Purpose

Mirror the OpenAPI response/request shapes used by the dashboard.

## Depends On

- `openapi/openapi.yaml` and frontend consumers.

## Public Surface

- String unions and interfaces for problems, contexts, offers, experiments, decisions with policy latency, outcomes with applied versions, bounded learning-series points, simulation benchmark references, summaries, offer performance, pagination, and simulation runs.

## Required Behavior

- Match snake-case JSON through mapping strategy chosen in API client; do not pretend wire fields are camelCase without conversion.
- Represent nullable values/reasons explicitly and avoid optional fields where API guarantees presence.
- Use `unknown` for unvalidated external errors, never `any`.

## Failure Cases

- Divergence from OpenAPI, loose string enums, non-null metrics that can be unavailable, or frontend-only fields mixed into API types.

## Non-Goals

- Runtime validation, React state, or chart-specific view models.

## Validation

- Typecheck and compile representative fixtures in `App.test.tsx`.

## Completion Checklist

- [x] Used API schemas are represented exactly.
- [x] Nullability/enums are strict.
- [x] No `any` escapes exist.
