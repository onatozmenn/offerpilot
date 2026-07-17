# File Spec: `internal/simulation/runner.go`

## Status

`validated`

## Purpose

Generate bounded traffic, request decisions through an abstract client, submit outcomes, and report run progress.

## Depends On

- Simulation profiles, domain/API-neutral client interface, clock/ticker abstraction, and context cancellation.

## Public Surface

- `DecisionClient` interface, run config, progress/result models, and `Runner.Run(ctx, config)`.

## Required Behavior

- Validate seed, rate, max decisions, and worker bounds.
- Use a bounded worker pool and rate limiter without spawning one unbounded goroutine per event.
- Generate context, request a decision, draw one terminal outcome, and submit it with unique event ID.
- Track decisions, outcomes, errors, observed reward sum, and synthetic uniform-random/oracle expected-reward sums; report progress as one coherent snapshot.
- Stop promptly on cancellation and wait for workers before returning.

## Failure Cases

- Client errors, outcome submission conflict, rate/ticker failure, max-error threshold, cancellation, or panic in worker callback.

## Non-Goals

- Owning active-run exclusivity, HTTP encoding, or mutating policy directly.

## Validation

- `go test -run TestRunner -count=10 ./internal/simulation`
- `go test -race ./internal/simulation`

## Completion Checklist

- [ ] Work and goroutines are bounded.
- [ ] Same seed/config/client behavior is reproducible.
- [ ] Cancellation and cleanup are proven.
