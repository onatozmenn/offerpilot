# Testing Strategy

## Principles

- Test observable behavior and invariants, not private implementation details.
- Inject time and randomness.
- Use deterministic seeds and include the seed in failures.
- Keep unit tests fast; mark PostgreSQL integration tests clearly.
- A new behavior is incomplete without its failure-path tests.

## Go Validation

Required commands:

```powershell
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
golangci-lint run
```

### Domain Tests

Cover enum validation, segment canonicalization, probability validation, reward mapping, timestamp constraints, and all global invariants.

### Policy Tests

Cover exact random distributions, epsilon edge cases `0` and `1`, tied best offers, prior state, update arithmetic, delayed feedback from an older selection version, consecutive application-version enforcement, deterministic sampling, concurrent select/update, snapshot round trips, and corrupt snapshots. Verify that the random policy advances version without changing its distribution.

Property-style loops verify probabilities are finite, bounded, complete, and sum to one across many generated candidate sets.

### Evaluation Tests

Use hand-calculated IPS/SNIPS fixtures, invalid propensities, zero weights, low effective sample size, and candidate policies with missing actions.

### Service Tests

Use narrow fakes for storage and policies. Cover atomic behavior, idempotent outcomes, competing outcomes, delayed/out-of-order decision feedback, version progression by acceptance order, crash-after-update-before-snapshot recovery, cancellation, and storage failures.

### HTTP Tests

Use `httptest`. Cover success schemas, content types, unknown fields, malformed and oversized JSON, stable problem codes, request IDs, pagination bounds, health, and simulation conflicts.

### PostgreSQL Tests

Use `testcontainers-go` against a real PostgreSQL version matching Compose. Apply migrations from scratch. Cover constraints, transactions, cursor ordering, idempotency, version-gap queries, and concurrent outcome submissions.

### Simulation Tests

Verify identical event sequences for identical seeds, divergence for different seeds, bounded worker count, cancellation, max decision limits, and plausible outcome ordering without asserting fragile exact aggregate rates.

## Frontend Validation

Required commands:

```powershell
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run build
```

Vitest and Testing Library cover initial loading, empty/error states, start/stop controls, polling cleanup, metric null reasons, table rendering, and accessible labels. Charts are tested through transformed data and accessible summaries rather than canvas pixels in unit tests.

## Contract Tests

Representative HTTP requests and responses are validated against `openapi/openapi.yaml`. Backend and frontend generated types are not required in the MVP, but field names must be checked in CI.

## End-To-End Smoke Test

After Docker Compose starts:

1. Readiness succeeds.
2. A demo experiment is created.
3. A short seeded simulation completes.
4. Summary reports the expected sample count.
5. Reposting one outcome does not change counts or policy version.
6. The web application loads without console or network errors.

## Coverage

Coverage percentage is diagnostic, not a target to game. Core policy, reward, probability, and idempotency branches require direct tests regardless of aggregate percentage.
