# Definition Of Done

A file is complete when its spec behavior is implemented, focused tests pass, formatting and static analysis pass, public contracts are synchronized, and no unrelated changes are included.

## Repository Release Gate

- Every manual implementation file has exactly one manifest entry and one file spec.
- Generated files are identified and never hand-edited.
- All Markdown local links resolve.
- OpenAPI examples match handler behavior.
- Migrations apply to an empty PostgreSQL database.
- `go vet ./...`, `go test ./...`, `go test -race ./...`, and `golangci-lint run` pass.
- Frontend lint, typecheck, test, and production build pass.
- Docker Compose configuration validates and services become healthy.
- End-to-end smoke test passes from a clean database.
- No credentials or real personal data are committed.
- Accessibility and responsive checks cover desktop and mobile.
- Benchmark claims include command, environment, dataset/seed, sample size, and date.

## Behavioral Acceptance

- Creating a demo experiment produces only fictional offers.
- The random policy emits a uniform valid distribution.
- Segmented epsilon-greedy emits exact documented propensities.
- One decision has at most one terminal outcome.
- Exact feedback retry is idempotent and does not update the policy twice.
- Policy state recovers deterministically after restart.
- A seeded simulation is reproducible.
- Summary values derive from persisted records and represent unavailable statistics as null with reasons.
- The dashboard can start, observe, stop, and inspect a run without manual API calls.

## Documentation Acceptance

- README explains the measured product, not planned features as if completed.
- Architecture diagrams match runtime behavior.
- Security documentation states that the project is not a credit or lending system.
- Known limitations identify synthetic feedback and single-replica policy ownership.
- CV bullets use only measured and implemented claims.
