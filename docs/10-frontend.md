# Frontend Design

## Product Shape

The first screen is the operational experiment dashboard, not a marketing page. It must let a reviewer start a seeded run, watch learning behavior, inspect decisions, and understand uncertainty without reading setup instructions inside the interface.

## Visual Direction

Use the warm editorial-instrument system defined in the root [DESIGN.md](../DESIGN.md):

- Eggshell `#fdfcfc` is the canvas, warm taupe `#f5f3f1` separates tool surfaces, stone `#ebe8e4` supplies hairline rules, and pure black anchors text and primary actions.
- Inter 300 is the locally packaged Waldenburg substitute for display headings; Inter 400/500 owns interface text and IBM Plex Mono remains reserved for values and identifiers.
- Buttons are fully pill-shaped, inputs stay compact at 4 px radius, and genuinely framed tools may use the design system's 20-24 px radii without nesting cards.
- Violet `#0447ff` and orange `#ff4704` are restricted to chart/probability visuals and never become button, link, focus, or status chrome.
- The top bar is an eggshell editorial wordmark row with a stone divider rather than a dark application masthead.
- Full-width sections retain constrained content, generous rhythm, and hairline separation. No glass effects, decorative blobs, or marketing hero composition.
- Letter spacing remains zero throughout the application.

## Page Structure

1. Compact transparent top bar with the OfferPilot wordmark, API health, and experiment selector.
2. Simulation control band with seed, rate, maximum decisions, start, and stop.
3. Stable metric strip for sample count, reward, engagement proxy, exploration, p95 latency, and policy version.
4. Main learning chart comparing observed reward with simulated random and oracle references.
5. Offer performance table with selections, outcomes, empirical mean, and current probability.
6. Recent decision feed with context chips, selected offer, propensity, outcome, and policy version.
7. Inline error or empty state bands when data is unavailable.

## Interaction Rules

- Use Lucide icons for start, stop, refresh, health, and status controls with tooltips where meaning is not obvious; icons remain monochrome.
- Use actual numeric inputs for seed, rate, and maximum events.
- Disable start while a run is active and stop while no run is active.
- Poll summary, run status, and decisions with one coordinated hook; stop polling on unmount.
- Preserve the last valid data during transient refresh and show stale state explicitly.
- Never display undefined metrics as `0`; show an em dash and the API reason in a tooltip.
- Surface that benchmarks are synthetic next to the chart legend.
- Keep all command hierarchy achromatic: black filled primary actions, eggshell outline secondary actions, and text/icon ghost controls.

## Responsive Behavior

- Desktop uses a 12-column grid and keeps controls on one row where space allows.
- Tablet wraps controls and splits chart/table vertically.
- Mobile uses single-column bands, horizontally scrollable data tables with a visible affordance, and touch targets of at least 44 px.
- Charts have stable aspect ratios and accessible text summaries.
- No text, control, legend, or tooltip may overlap at widths from 320 px upward.

## States

Every data surface defines loading, empty, running, paused/completed, stale, and error states. The app must remain useful if metrics load but the decision feed fails, or vice versa.

## Accessibility

- Meet WCAG AA contrast.
- Use semantic headings, forms, tables, and live regions.
- Associate every input with a visible label.
- Do not encode policy or outcome only by color.
- Honor reduced-motion preferences; use motion only for a brief initial reveal and live-point insertion.

## Data Ownership

The frontend formats API projections but does not recompute policy probabilities, rewards, IPS, or business aggregates. TypeScript types mirror the OpenAPI contract.
