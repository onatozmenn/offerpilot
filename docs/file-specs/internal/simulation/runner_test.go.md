# File Spec: `internal/simulation/runner_test.go`

## Status

`specified`

## Purpose

Verify profile, runner, manager, and HTTP client determinism, bounds, lifecycle, and error behavior.

## Depends On

- All simulation implementation files and deterministic fake clients/tickers/stores.

## Public Surface

- Tests grouped as `TestProfile_`, `TestRunner_`, `TestManager_`, and `TestHTTPClient_`.

## Required Behavior

- Compare identical/different seeds, validate outcome/expected-reward bounds, max decisions, rate/worker bounds, cancellation, error threshold, exact event IDs, observed/random/oracle totals, active-run conflict, panic recovery, synchronous terminal persistence failure, stop/shutdown idempotency, and HTTP body/context handling.
- Verify restart reconciliation marks active runs failed, preserves partial totals, and permits a new run.
- Use fake tickers or small deterministic channels rather than sleep-based timing.

## Failure Cases

- Wall-clock sleeps, unbounded goroutines, flaky statistical thresholds, or leaked test servers.

## Non-Goals

- Load benchmarking or policy algorithm assertions.

## Validation

- `go test -run 'Test(Profile|Runner|Manager|HTTPClient)' -count=10 ./internal/simulation`
- `go test -race ./internal/simulation`

## Completion Checklist

- [ ] Determinism and cancellation are proven.
- [ ] Every goroutine/resource terminates.
- [ ] Race validation passes.
