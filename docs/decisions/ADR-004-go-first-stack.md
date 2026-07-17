# ADR-004: Keep The Runtime Go-First

## Status

Accepted.

## Context

The project should demonstrate Go in an AI system rather than add another Python-first notebook. The MVP algorithms are incremental and small enough to implement transparently with the standard library.

## Decision

Implement API, policies, OPE, simulator, and operational instrumentation in Go. Use PostgreSQL for persistence and React/TypeScript for visualization. Do not add a Python runtime to the MVP. External research implementations may serve as validation references only.

## Consequences

- Deployment remains a small set of binaries and static assets.
- Algorithm behavior is visible and benchmarkable in one language.
- Advanced scientific tooling is less available than in Python.
- Open Bandit Dataset cross-validation may later add an optional research workflow without entering the serving path.
