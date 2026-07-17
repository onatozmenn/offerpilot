# File Spec: `internal/httpapi/problem.go`

## Status

`specified`

## Purpose

Map decoding and typed service errors to stable RFC 9457-style problem responses.

## Depends On

- API problem contract and service error taxonomy.

## Public Surface

- Problem DTO, stable code constants, error mapping, and JSON writer helpers.

## Required Behavior

- Set `application/problem+json`, status, request ID, stable code/type/title, and safe detail.
- Map invalid input, not found, conflict, unavailable/unhealthy, deadline/cancellation, and internal errors consistently.
- Log internal root cause once through injected logger; return redacted detail.
- Handle JSON encoding failure without recursive writes.

## Failure Cases

- Leaking SQL/config/stack details, inconsistent status/code, multiple response writes, or missing request ID.

## Non-Goals

- Defining domain errors or localizing messages.

## Validation

- Table-driven error mapping tests in `handlers_test.go`.

## Completion Checklist

- [ ] Every documented problem code maps predictably.
- [ ] Internal details stay server-side.
- [ ] Content type and request ID are tested.
