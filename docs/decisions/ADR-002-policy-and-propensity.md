# ADR-002: Use Segmented Epsilon-Greedy First

## Status

Accepted.

## Context

The portfolio value comes from a correct online-learning loop, exact logging, and defensible evaluation. Thompson Sampling action probabilities are easy to log incorrectly, while advanced contextual models add numerical and feature complexity.

## Decision

Implement a uniform random baseline and segmented epsilon-greedy adaptive policy. Both return the complete action distribution, making selected propensities exact. Context is a coarse deterministic segment from four approved features.

## Consequences

- The algorithm is interpretable and testable.
- IPS/SNIPS can use trustworthy behavior probabilities.
- Sparse segments and linear generalization are limitations.
- LinUCB or Thompson Sampling requires a later ADR and benchmark against the baseline.
