# File Spec: `web/src/components/MetricStrip.tsx`

## Status

`validated`

## Purpose

Render stable, accessible headline experiment metrics without recomputation.

## Depends On

- Summary API types and visual contract.

## Public Surface

- `MetricStrip` props containing sample count, reward, engagement, exploration, p95 policy-selection latency, policy version, loading/stale status, and null reasons.

## Required Behavior

- Preserve fixed layout dimensions across loading/value/error states.
- Format server values consistently and show an em dash plus accessible reason for unavailable metrics.
- Label synthetic/observed distinctions and stale state without color-only meaning.
- Use semantic list/definition markup.

## Failure Cases

- Converting null to zero, layout shift, invented percentages, or unlabeled abbreviations.

## Non-Goals

- Fetching data or calculating aggregates.

## Validation

- Component cases in `App.test.tsx` for values, null reasons, loading, and stale state.

## Completion Checklist

- [x] Null and stale semantics are visible/accessibly named.
- [x] Dimensions remain stable.
- [x] No metric math occurs locally.
