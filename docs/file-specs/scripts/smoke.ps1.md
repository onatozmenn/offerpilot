# File Spec: `scripts/smoke.ps1`

## Status

`validated`

## Purpose

Run a bounded end-to-end API smoke test against a clean local Compose deployment.

## Depends On

- Implemented API contract, healthy Compose services, and HTTP commands available in both Windows PowerShell 5.1 and PowerShell 7.

## Public Surface

- Windows PowerShell 5.1/PowerShell 7-compatible parameters for API base URL, timeout, seed, and optional keep-data behavior; non-zero failure exit.

## Required Behavior

- Wait for readiness within a bounded deadline without an infinite loop.
- Create a fresh demo experiment, start a short seeded run, poll to terminal state, and verify expected decision/outcome counts and non-null core summary values.
- Re-submit a captured outcome request only when the test has a known event payload and verify idempotency without policy-version/count change.
- Include request IDs, redact response bodies on unexpected internal errors, and clean created resources/Compose state only when explicitly requested.
- Print a concise reproducibility summary with seed and elapsed time.

## Failure Cases

- Readiness timeout, non-2xx problem, run failure/timeout, count mismatch, idempotency regression, malformed JSON, or cancellation.

## Non-Goals

- Load testing, browser checks, destructive production cleanup, or deployment.

## Validation

- Run after `docker compose up --build --wait` from root and a nested directory.

## Completion Checklist

- [x] Smoke path is bounded and repeatable.
- [x] Core create/run/summary/idempotency behavior is asserted.
- [x] Failures return non-zero with safe diagnostics.
