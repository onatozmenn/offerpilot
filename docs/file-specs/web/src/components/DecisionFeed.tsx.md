# File Spec: `web/src/components/DecisionFeed.tsx`

## Status

`specified`

## Purpose

Show recent decisions and outcomes as a compact auditable stream.

## Depends On

- Decision page API types and responsive styling.

## Public Surface

- `DecisionFeed` props for items, loading/empty/error/stale status, and optional load-more callback.

## Required Behavior

- Display time, privacy-safe context values, selected fictional offer, propensity, policy version, and terminal outcome/reward when present.
- Truncate identifiers visually while preserving full value in accessible title/copy affordance if implemented.
- Use semantic list/table structure, bounded height or pagination, and stable row dimensions.
- Distinguish pending outcome from ignored outcome.

## Failure Cases

- Showing raw JSON, treating pending as zero reward, exposing request internals, or unbounded DOM growth.

## Non-Goals

- Live sockets, outcome submission, or policy explanation generation.

## Validation

- Feed state and pending/terminal rendering tests in `App.test.tsx`.

## Completion Checklist

- [ ] Audit fields are clear and bounded.
- [ ] Pending and terminal states differ.
- [ ] Large feeds remain controlled.
