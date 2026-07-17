# File Spec: `web/src/App.tsx`

## Status

`validated`

## Purpose

Compose the complete operational dashboard and coordinate component-level states.

## Depends On

- Dashboard hook, API types, and all five dashboard components.

## Public Surface

- Default `App` component.

## Required Behavior

- Render top bar/health and experiment selection, controls, metric strip, learning chart, offer table, and decision feed in documented order.
- Delegate server interaction/polling to `useDashboard` and calculations to backend projections.
- Represent initial loading, no experiment, partial panel error, stale data, running/stopping/completed, and fatal API error states.
- Allow creating a fresh demo experiment from the empty state.
- Preserve last valid panel data during transient refresh failures.
- Compose the operational controls and data surfaces using the editorial wordmark/header and section hierarchy in `DESIGN.md` without adding a marketing hero.

## Failure Cases

- One failed panel blanking the whole dashboard, hidden synthetic benchmark label, nested cards, or derived policy/reward math.

## Non-Goals

- Client-side routing, authentication, or marketing sections.

## Validation

- `npm --prefix web run test -- App.test.tsx`
- Desktop/mobile browser screenshots after implementation.

## Completion Checklist

- [x] All required states and components are composed.
- [x] Backend remains the calculation source.
- [x] Responsive/accessibility checks pass.
