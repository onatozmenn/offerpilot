# ADR-001: Start With A Modular Monolith

## Status

Accepted.

## Context

OfferPilot needs consistent policy state, transactional outcomes, a small operational surface, and an implementation a single developer can reason about. Premature services would add delivery and consistency mechanisms before scale is measured.

## Decision

Use one Go API process with internal package boundaries, one optional Go simulator process, PostgreSQL, and one React frontend. Do not add a message broker or independent policy service in the MVP.

## Consequences

- Local setup and end-to-end testing remain small.
- In-memory policy ownership is limited to one API replica.
- Package boundaries must be enforced in review because process boundaries do not enforce them.
- Multi-replica serving requires a future ADR for policy ownership and update delivery.
