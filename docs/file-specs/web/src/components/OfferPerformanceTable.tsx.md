# File Spec: `web/src/components/OfferPerformanceTable.tsx`

## Status

`validated`

## Purpose

Present per-offer selection, outcome, empirical mean, and current probability data for scanning and comparison.

## Depends On

- Offer-performance API types and responsive table styling.

## Public Surface

- `OfferPerformanceTable` with rows and loading/empty/error/stale state.

## Required Behavior

- Use a semantic table with merchant/offer, category, selections, outcome rates, reward mean, and current probability.
- Preserve server ordering or expose an explicit deterministic client sort control without mutating props.
- Format nulls with reason, show policy probability as text plus restrained bar, and label synthetic data.
- Provide horizontal overflow affordance on mobile and keep first identifying column readable.

## Failure Cases

- Client-recomputed means, unlabeled percentages, inaccessible bar-only values, nested cards, or table overflow hiding controls.

## Non-Goals

- Editing offers or policy configuration.

## Validation

- Table states, headers, values/nulls, and immutability tests in `App.test.tsx`.

## Completion Checklist

- [x] Semantic columns and null reasons render correctly.
- [x] Mobile overflow remains usable.
- [x] No aggregate math occurs locally.
