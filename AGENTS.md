# OfferPilot Project Guidelines

## Source Of Truth

- Start with [docs/README.md](docs/README.md).
- Treat architecture, API, domain, data, and file-spec documents as contracts.
- Do not create or edit an implementation file unless it appears in [the file manifest](docs/13-file-manifest.md) and its matching file spec has been read.
- If implementation requires a new file, add its spec and manifest entry first.

## Working Method

1. Select the next unblocked file from the delivery plan.
2. Read that file's spec and every listed dependency.
3. Implement the smallest complete behavior described by the spec.
4. Run the focused tests named in the spec before touching another slice.
5. Update implementation status only after validation succeeds.

## Architecture

- Keep the MVP as a modular monolith: one Go API process, one optional Go simulator process, PostgreSQL, and one React application.
- Domain and bandit packages must not import HTTP, PostgreSQL, or frontend concerns.
- Services own use-case orchestration. Adapters own transport and persistence.
- Define interfaces where they are consumed. Do not create generic repository or utility packages.
- Preserve the documented API and database contracts unless an ADR explicitly changes them.

## Go Rules

- Use the standard library first. Add a dependency only when its value is documented.
- Pass `context.Context` through I/O boundaries and honor cancellation.
- Keep randomness injectable and seeded in tests.
- Return errors with useful context; do not log and return the same error.
- Run `gofmt`, `go vet ./...`, `go test -race ./...`, and the configured linter.

## Product Safety

- This is a marketplace-offer personalization simulator, not a lending or credit-decision system.
- Never add age, gender, race, precise location, credit data, income, disability, or other protected/sensitive attributes to decision context.
- Use fictional merchants and synthetic traffic in the built-in demo.
- Log policy probabilities and versions for every decision. Do not claim uplift without a reproducible evaluation.

## Frontend Rules

- Follow [docs/10-frontend.md](docs/10-frontend.md).
- Keep the dashboard operational and data-dense, with no landing-page hero or decorative card nesting.
- Every loading, empty, error, paused, and running state must be implemented.
