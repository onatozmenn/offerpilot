# File Spec: `internal/simulation/profiles.go`

## Status

`specified`

## Purpose

Define deterministic synthetic context distributions and hidden offer-affinity outcome probabilities.

## Depends On

- Domain context/outcome enums and fictional demo offer slugs.

## Public Surface

- Versioned `Profile`, context generator, outcome probability lookup, and per-context uniform-random/oracle expected-reward helpers.

## Required Behavior

- Generate only approved context enums using an injected seeded random source.
- Keep hidden affinity parameters separate from policy-visible features/state.
- Produce valid probabilities for ignored/clicked/converted that sum to one and finite expected rewards bounded to `[0,1]`.
- Use fictional offer/category keys and a documented profile version.
- Return deterministic output for seed and call order.

## Failure Cases

- Unknown offer/category, invalid probability sum, protected attributes, unstable map iteration, or global randomness.

## Non-Goals

- Real user modeling or claims of realistic conversion rates.

## Validation

- Profile tests in `runner_test.go`, including enum/probability/property loops.

## Completion Checklist

- [ ] Profiles are deterministic and privacy-safe.
- [ ] Hidden rewards cannot leak into policy input.
- [ ] Probability invariants pass.
