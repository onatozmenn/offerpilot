# File Spec: `web/src/components/LearningChart.tsx`

## Status

`specified`

## Purpose

Visualize observed cumulative-average reward against clearly labeled synthetic random and oracle expected-average references.

## Depends On

- Summary time-series projection, Recharts, and frontend visual/accessibility contract.

## Public Surface

- `LearningChart` props for at most 120 server-provided cumulative-average points, nullable random/oracle expected-average horizontal references with reason codes, loading/error/empty/stale state, and benchmark availability.

## Required Behavior

- Use a stable responsive aspect ratio, bounded axis labels, distinct color/dash patterns, accessible legend, and tooltip.
- Display an adjacent text summary/table for screen readers and no-data states; never derive cumulative values from truncated client history.
- Label random/oracle lines as simulation-only; omit unavailable series rather than drawing zero.
- Respect reduced motion and disable decorative animation when requested.

## Failure Cases

- Misleading interpolation, clipped legends/tooltips, color-only series, zero-filled missing data, or chart-driven layout shifts.

## Non-Goals

- Computing rolling windows, OPE, or benchmark values.

## Validation

- Data transformation/accessibility tests in `App.test.tsx` plus desktop/mobile browser screenshot checks.

## Completion Checklist

- [ ] Observed and synthetic series are unmistakable.
- [ ] Missing/empty/error states are implemented.
- [ ] Chart and text summary are accessible.
