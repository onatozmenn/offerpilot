# File Spec: `internal/evaluation/ope_test.go`

## Status

`validated`

## Purpose

Verify IPS/SNIPS calculations and defensive numerical behavior.

## Depends On

- `internal/evaluation/ope.go`.

## Public Surface

- Table-driven hand-calculated estimator tests.

## Required Behavior

- Cover on-policy equality at the minimum sample threshold, non-uniform weights, zero candidate probability, empty samples, effective sample size immediately below `10`, invalid behavior propensities, invalid rewards, `NaN`, infinity, and weight overflow.
- Compare floating values using documented tolerances and include intermediate expected values in fixtures.

## Failure Cases

- Using the implementation to generate expected values or hiding instability with broad tolerance.

## Non-Goals

- Benchmarking or validating advanced estimators.

## Validation

- `go test -run TestEvaluate -count=10 ./internal/evaluation`

## Completion Checklist

- [ ] Expected results are independently calculated.
- [ ] Every invalid numeric class is covered.
- [ ] Repeated runs are deterministic.
