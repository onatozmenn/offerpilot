# File Spec: `web/src/components/SimulationControls.tsx`

## Status

`specified`

## Purpose

Render validated seed/rate/count inputs and start/stop commands for one simulation run.

## Depends On

- Simulation API types, Lucide icons, and dashboard hook callbacks.

## Public Surface

- Controlled props for values, bounds, run status, pending states, errors, and change/start/stop handlers.

## Required Behavior

- Use labeled numeric inputs with min/max/step and visible validation.
- Use Play and Square icons with text where command clarity benefits.
- Disable start while active/pending/invalid and stop while inactive/pending.
- Show current seed and progress without changing input values mid-run.
- Provide status in an accessible live region.

## Failure Cases

- Browser coercion to `NaN`, duplicate submit, icon-only ambiguous action, or controls resizing by status text.

## Non-Goals

- Calling API directly or deciding backend bounds.

## Validation

- Interaction tests in `App.test.tsx` for validation, keyboard submission, disabled states, and callbacks.

## Completion Checklist

- [ ] Inputs and commands are accessible and bounded.
- [ ] Duplicate actions are prevented.
- [ ] Running/stopping states are stable.
