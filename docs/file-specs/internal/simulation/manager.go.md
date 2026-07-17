# File Spec: `internal/simulation/manager.go`

## Status

`specified`

## Purpose

Own lifecycle and exclusivity for dashboard-triggered in-process simulation runs.

## Depends On

- Runner, persistent simulation-run store methods, engine client adapter, and clock.

## Public Surface

- `Manager` with `RecoverInterrupted`, `Start`, `Get`, `Stop`, and `Shutdown`.

## Required Behavior

- Permit one active run per experiment and return conflict otherwise.
- Before readiness, transition persisted active states from any prior process to `failed/process_restarted`; do not attempt to resume goroutines.
- Persist starting/running/stopping transitions and bounded periodic counters/reward sums.
- Launch exactly one supervised background run, recover panics into failed status, and release active ownership on every exit.
- Make stop and shutdown idempotent; wait within caller deadlines.
- Persist terminal status and final counters/reward sums synchronously in one transaction before releasing active ownership or reporting completion. If terminal persistence fails, retain an in-memory failed/unhealthy marker, retry only within the bounded shutdown/run context, and make readiness fail rather than reporting a false completed run.

## Failure Cases

- Store transition failure, runner failure/panic, duplicate start, unknown run, cancellation, or shutdown timeout.

## Non-Goals

- Distributed run scheduling or surviving API process restart as an active run.

## Validation

- Manager lifecycle tests in `runner_test.go` under `-race`.

## Completion Checklist

- [ ] Active ownership cannot leak.
- [ ] All terminal paths persist status.
- [ ] Stop/shutdown are idempotent and tested.
