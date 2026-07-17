# File Spec: `web/src/hooks/useDashboard.ts`

## Status

`specified`

## Purpose

Coordinate experiment selection, polling, stale data, and simulation commands for the dashboard.

## Depends On

- API client/types and React hooks.

## Public Surface

- `useDashboard` returning data, per-panel status/error/staleness, selected experiment/run, form values, and command functions.

## Required Behavior

- Load experiments then selected detail/summary/feed; choose newest when none selected.
- Poll active runs faster and stopped experiments slower through one coordinated timer.
- Abort superseded/unmounted requests and prevent stale responses overwriting newer selection.
- Preserve last valid summary/feed on refresh failure and mark stale.
- Use transitions for experiment switches where appropriate; expose command pending states and refresh after create/start/stop.

## Failure Cases

- Overlapping poll storms, state updates after unmount, stale race, duplicate start/stop, partial request failure, or invalid control values.

## Non-Goals

- Rendering, backend metric calculation, global state framework, or localStorage identity.

## Validation

- Hook behavior exercised through `App.test.tsx` with fake timers and controlled promises.

## Completion Checklist

- [ ] Polling and abort lifecycle are deterministic.
- [ ] Partial/stale states preserve useful data.
- [ ] Commands cannot double-submit.
