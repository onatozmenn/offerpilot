# File Spec: `openapi/openapi.yaml`

## Status

`specified`

## Purpose

Provide the machine-readable OfferPilot HTTP contract.

## Depends On

- `docs/04-api-contract.md`
- Domain enums and problem format.

## Public Surface

- OpenAPI 3.1 document for every `/v1`, health, and metrics-facing HTTP behavior where JSON schemas apply.

## Required Behavior

- Define reusable UUID, timestamp, context, offer, distribution, experiment, decision with policy latency, outcome with applied version, simulation benchmark, bounded learning-series point, run, summary, pagination, and problem schemas.
- Include documented examples and response status codes.
- Set `additionalProperties: false` on write request objects.
- Do not expose `simulation_run_id` on the public decision request; reject it as an unknown field.
- Represent unavailable summary metrics and simulation-only random/oracle expected-average references as nullable values plus reason codes.
- Constrain the learning series to at most 120 ordered points and make its cumulative sample/average semantics explicit.
- Keep operation IDs stable for future client generation.

## Failure Cases

- Drift from handlers, ambiguous numeric bounds, missing error responses, or schemas accepting protected attributes.

## Non-Goals

- Generating implementation code in the MVP.

## Validation

- Run an OpenAPI 3.1 validator.
- Execute backend contract tests against representative examples.

## Completion Checklist

- [ ] Every documented route and schema is represented.
- [ ] Examples validate.
- [ ] Contract tests pass.
