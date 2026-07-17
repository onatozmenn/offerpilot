# File Spec: `internal/httpapi/dto.go`

## Status

`specified`

## Purpose

Define HTTP request/response DTOs and explicit mappings to/from domain/service models.

## Depends On

- OpenAPI contract and domain/service projections.

## Public Surface

- Unexported or package-scoped DTO structs and mapping functions used by handlers, including learning-series points and simulation benchmark references.

## Required Behavior

- Match OpenAPI field names, nullability, enum values, and examples.
- Keep `simulation_run_id` out of the public decision request DTO; it exists only on internal service commands/records.
- Keep write DTOs closed to unknown fields through decoder behavior.
- Copy slices/maps so response serialization cannot mutate domain state.
- Format timestamps as UTC RFC 3339 and preserve full action distributions, policy selection latency, selection/applied versions, benchmark labels, and null reasons.

## Failure Cases

- Protected/uncontracted fields, accidental `omitempty` on required values, zero replacing unavailable metrics, or domain pointers exposed directly.

## Non-Goals

- Validation policy, database row mapping, or HTTP status selection.

## Validation

- JSON golden/round-trip tests in `handlers_test.go` and OpenAPI contract checks.

## Completion Checklist

- [ ] DTOs mirror the OpenAPI contract exactly.
- [ ] Null and copy semantics are tested.
- [ ] No adapter fields enter domain models.
