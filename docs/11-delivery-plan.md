# Delivery Plan

## Rule

Implementation proceeds by dependency order. Each planned file must have a matching file spec before creation. A phase is complete only after its focused validation passes.

## Phase 0: Documentation Gate

- Complete product, architecture, domain, API, learning, data, security, observability, testing, frontend, and definition-of-done documents.
- Complete ADRs.
- Create the one-to-one file manifest and validate all spec mappings.

Exit: documentation validation passes and no implementation file exists.

## Phase 1: Repository Foundation

- Go module and pinned tooling configuration.
- Environment example, Git ignore rules, and documentation validator.
- React/Vite dependency and configuration shell generated only after its specs are approved; feature build validation is deferred until Phase 6 source exists.
- Container, CI, and OpenAPI implementation remains in its dependency-ordered later phase.

Exit: Go module/tooling and frontend package/config syntax validate without pretending the application source exists.

## Phase 2: Domain And Policy Core

- Domain values and validation.
- Random and segmented epsilon-greedy policies.
- Snapshot/restore and OPE estimators.
- Deterministic unit and race tests.

Exit: core packages pass `go test -race` without PostgreSQL.

## Phase 3: Persistence And Services

- Initial migration and PostgreSQL adapter.
- Decision, outcome, experiment, recovery, and summary orchestration.
- Real PostgreSQL integration tests.

Exit: migrations from empty database and service idempotency/recovery tests pass.

## Phase 4: HTTP API

- Router, DTOs, handlers, problem responses, limits, health, and metrics.
- OpenAPI contract completed alongside handlers.
- HTTP and contract tests.

Exit: API behavior matches the contract and malformed inputs fail safely.

## Phase 5: Simulation

- Fictional offer bootstrap.
- Seeded context/outcome profiles.
- Bounded in-process run manager and external simulator command.
- Random/oracle synthetic benchmarks.

Exit: same seed is reproducible, cancellation works, and no races are reported.

## Phase 6: Dashboard

- Typed API client and coordinated polling hook.
- Controls, metrics, chart, offer table, and decision feed.
- Responsive and accessibility states.

Exit: lint, typecheck, tests, build, and desktop/mobile browser checks pass.

## Phase 7: Packaging And Evidence

- Harden containers and Compose health checks.
- Run end-to-end smoke test and benchmark.
- Record measured results and screenshots in README without invented claims.

Exit: fresh-clone onboarding is reproducible and portfolio evidence is current.

## Deferred Work

Open Bandit Dataset ingestion, LinUCB, Thompson Sampling, Kafka, Kubernetes, AWS deployment, distributed policy ownership, LLM-generated campaign variants, real merchants, and user authentication are separate roadmap decisions.
