# File Spec: `web/src/styles.css`

## Status

`specified`

## Purpose

Implement OfferPilot's responsive visual system and component styling.

## Depends On

- `docs/10-frontend.md` and component markup.

## Public Surface

- CSS variables, reset/base styles, layout bands, controls, tables, charts, states, and responsive rules.

## Required Behavior

- Define a multi-accent warm-white/ink palette, typography, spacing, borders, focus rings, and stable dimensions.
- Use IBM Plex families, zero letter spacing, restrained radii, full-width bands, and no gradients/orbs/glass/nested cards.
- Maintain chart aspect ratio, metric/control heights, table overflow affordance, 44 px touch targets, and non-overlap from 320 px upward.
- Style reduced motion, keyboard focus, disabled, loading, stale, error, and screen-reader utility states.

## Failure Cases

- Viewport-scaled font sizes, hidden focus, color-only meaning, clipped long identifiers, or global selectors that break chart internals.

## Non-Goals

- CSS-in-JS, utility framework, or remote assets.

## Validation

- Browser checks at 320, 768, 1280, and 1440 px; reduced-motion and keyboard navigation checks.

## Completion Checklist

- [ ] Visual contract and stable dimensions are implemented.
- [ ] Mobile/desktop layouts do not overlap.
- [ ] Focus/contrast/reduced-motion checks pass.
