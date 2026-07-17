# File Spec: `internal/service/summary.go`

## Status

`validated`

## Purpose

Assemble experiment dashboard summaries from persisted aggregates, policy view, and OPE calculations.

## Depends On

- Engine store port, policy state projection, evaluation package, and domain summary values.

## Public Surface

- Summary, bounded learning-series, benchmark, offer-performance models and `Engine.Summary` plus recent-decision pagination methods.

## Required Behavior

- Query persisted counts, rewards, outcomes, policy-selection latencies, a deterministic maximum-120-point cumulative-average series, offer statistics, simulation benchmark sums, and logged OPE records.
- Project the nullable engagement-rate proxy from persisted clicked plus converted counts divided by terminal outcomes; return `no_outcomes` when unavailable.
- Add current empirical means/probabilities without mutating policy state.
- Return unavailable metrics as nil values with stable reasons such as `insufficient_samples` or `not_simulated`.
- Divide persisted simulation expected-reward sums by their contributing decision count to return random/oracle expected-average horizontal references; distinguish them from observed metrics and return `not_simulated` when absent.
- Preserve cursor and deterministic ordering.

## Failure Cases

- Unknown experiment, inconsistent aggregate rows, invalid OPE input, store failure, or unavailable policy view.

## Non-Goals

- Recomputing aggregates in the frontend, Prometheus metrics, or candidate policy training.

## Validation

- `go test -run TestEngine_Summary ./internal/service`

## Completion Checklist

- [ ] Null reasons are stable and tested.
- [ ] Synthetic references are clearly labeled.
- [ ] Summary uses persisted facts.
