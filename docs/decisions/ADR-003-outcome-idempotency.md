# ADR-003: Accept One Terminal Outcome Per Decision

## Status

Accepted.

## Context

Online updates must not be applied twice when clients retry. Supporting arbitrary click and conversion sequences would require attribution windows and delayed-reward semantics beyond the MVP.

## Decision

Accept exactly one terminal outcome per decision: ignored, clicked, or converted. Require a UUID event ID. Exact retries return the original result; a different terminal event conflicts. The server derives reward.

## Consequences

- Policy updates and idempotency are straightforward.
- The model cannot learn from a click followed later by a conversion.
- Delayed multi-event attribution requires a future ADR and schema migration.
