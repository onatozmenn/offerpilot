# File Spec: `internal/evaluation/ope.go`

## Status

`specified`

## Purpose

Compute auditable IPS and self-normalized IPS estimates from logged decisions and outcomes.

## Depends On

- Domain decision/outcome models.
- `docs/05-online-learning.md`.

## Public Surface

- Evaluation record containing reward, behavior propensity, and candidate action probability.
- Result containing IPS, SNIPS, sample count, effective sample size, weight diagnostics, and nullable reason.
- `Evaluate(records []Record) (Result, error)`.
- Minimum reportable effective sample size `10`; stable null reasons `no_samples`, `zero_candidate_weight`, and `low_effective_sample_size`.

## Required Behavior

- Validate finite bounded rewards and probabilities; behavior propensity must be greater than zero.
- Compute importance weights, IPS, SNIPS, and effective sample size with numerically stable accumulation.
- Return the documented insufficient-data reason for empty input, zero candidate weight, or effective sample size below `10`.
- Never clamp invalid input into apparently valid estimates.

## Failure Cases

- Missing actions, zero/negative propensity, probability above one, `NaN`/infinity, or overflow/non-finite aggregate.

## Non-Goals

- Direct-method, doubly robust, confidence intervals, or querying storage.

## Validation

- `go test -run TestEvaluate -count=1 ./internal/evaluation`

## Completion Checklist

- [ ] Hand-calculated fixtures match.
- [ ] Diagnostics and null reasons are explicit.
- [ ] Invalid numeric inputs fail.
