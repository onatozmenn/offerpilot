# File Spec: `internal/simulation/http_client.go`

## Status

`specified`

## Purpose

Implement the simulation `DecisionClient` against OfferPilot's public HTTP decision and outcome endpoints.

## Depends On

- API contract, simulation client interface, and standard `net/http`.

## Public Surface

- HTTP client constructor with base URL, timeout/client injection, and decision/outcome methods.

## Required Behavior

- Resolve only relative known API paths against a validated base URL.
- Encode/decode bounded JSON, set content type and request ID, propagate context, and always close/drain response bodies appropriately.
- Decode problem responses into stable typed client errors without exposing raw bodies.
- Do not retry decision/outcome writes automatically; caller owns retry/idempotency policy.

## Failure Cases

- Invalid base URL, transport timeout, non-JSON/problem response, oversized response, schema failure, or unexpected status.

## Non-Goals

- Authentication, discovery, database access, or policy calculations.

## Validation

- `httptest` cases in `runner_test.go` for success, problems, malformed/oversized response, and cancellation.

## Completion Checklist

- [ ] Context and body lifecycle are correct.
- [ ] Write requests are not invisibly retried.
- [ ] Error mapping is bounded and tested.
