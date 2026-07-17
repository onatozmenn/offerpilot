# OfferPilot

OfferPilot is a Go-first online-learning system for fictional marketplace offers. It selects an offer from privacy-safe session context, logs the full action distribution and exact propensity, applies terminal feedback once, and exposes persisted learning behavior through an operational dashboard.

The MVP is implemented end to end: PostgreSQL persistence, random and segmented epsilon-greedy policies, recovery, IPS/SNIPS evaluation, seeded simulation, OpenAPI HTTP endpoints, React dashboard, hardened containers, smoke tests, and CI gates.

![OfferPilot desktop dashboard after a 500-decision seeded simulation](docs/images/dashboard-desktop.png)

<p align="center">
	<img src="docs/images/dashboard-mobile.png" width="390" alt="OfferPilot mobile dashboard controls and metrics" />
</p>

## Product Boundary

OfferPilot optimizes the ordering of fictional merchant offers using synthetic feedback. It does not determine credit eligibility, personalize loan terms, process payments, use protected attributes, or ingest real customer identities.

## What It Demonstrates

- Exact, persisted propensities for uniform-random and segmented epsilon-greedy selection.
- Idempotent terminal outcomes with consecutive applied policy versions.
- Snapshot recovery plus replay of accepted outcomes after a process interruption.
- Persisted dashboard projections, bounded learning series, policy latency, and nullable reason codes.
- IPS/SNIPS off-policy estimates with effective-sample-size safeguards.
- Seeded in-process simulation with clearly labeled random and oracle simulation-only references.
- An external simulator that exercises only the public decision/outcome API boundary.
- Bounded request bodies, structured problems, request IDs, redacted logs, and bounded-cardinality metrics.
- Responsive, keyboard-accessible dashboard states with stale-data preservation and independent panel failures.

## Quick Start

Prerequisites: Docker 29.x with Docker Compose.

```powershell
docker compose up --build --wait
```

Open:

- Dashboard: `http://127.0.0.1:4173`
- API readiness: `http://127.0.0.1:8080/health/ready`
- Prometheus metrics: `http://127.0.0.1:8080/metrics`
- OpenAPI contract: [openapi/openapi.yaml](openapi/openapi.yaml)

The default Compose credentials are disposable local-development placeholders. To customize ports or local values:

```powershell
Copy-Item .env.example .env
docker compose up --build --wait
```

Stop the stack and remove its local database volume:

```powershell
docker compose down --volumes
```

## Smoke Test

With Compose healthy, run the bounded seeded workflow:

```powershell
.\scripts\smoke.ps1 -ApiBaseUrl http://127.0.0.1:8080 -Seed 20260717
```

It creates a fresh fictional experiment, completes 40 decisions and outcomes, checks persisted summary values, then replays one captured outcome and verifies that counts and policy version do not change. It does not remove data or stop Compose unless `-CleanupCompose` is explicitly supplied.

## Architecture

```mermaid
flowchart LR
		Browser[React dashboard] -->|OpenAPI JSON| API[Go HTTP API]
		Simulator[External seeded simulator] -->|Public decisions and outcomes| API
		API --> Service[Use-case engine]
		Service --> Policy[In-memory policy state]
		Service --> Store[PostgreSQL adapter]
		Store --> DB[(PostgreSQL 18)]
		Policy --> Snapshot[Persisted snapshots]
		Snapshot --> Store
		API --> Metrics[Prometheus metrics]
```

One API replica owns in-memory policy state. Startup restores the latest snapshot, replays consecutive accepted outcomes, fails closed on a version gap, and marks interrupted simulation runs failed before readiness succeeds.

The dashboard never recomputes policy probabilities, rewards, benchmarks, or aggregates. It renders projections returned by the API and preserves the last valid summary/feed during transient refresh failures.

## API Workflows

| Workflow | Endpoints |
| --- | --- |
| Demo catalog | `POST /v1/demo/experiments` |
| Selection and feedback | `POST /v1/decisions`, `POST /v1/outcomes` |
| Dashboard data | `GET /v1/experiments/{id}/summary`, `GET /v1/experiments/{id}/decisions` |
| In-process simulation | `POST /v1/experiments/{id}/simulation-runs`, `GET /v1/simulation-runs/{id}`, `POST /v1/simulation-runs/{id}/stop` |
| Operations | `/health/live`, `/health/ready`, `/metrics` |

## Local Development

Validated toolchain:

- Go `1.26.5`
- Node.js `22.16.0` and npm `10.9.2`
- PostgreSQL `18`

Start only PostgreSQL, then run the API and Vite from source:

```powershell
docker compose up -d db
$env:OFFERPILOT_DATABASE_URL = 'postgres://offerpilot:offerpilot_local@127.0.0.1:5432/offerpilot?sslmode=disable'
go run ./cmd/api
```

In a second terminal:

```powershell
Set-Location web
npm ci
npm run dev
```

Vite serves `http://127.0.0.1:5173` and proxies API paths to `http://127.0.0.1:8080` by default.

## Validation

```powershell
# Documentation and contracts
.\scripts\validate-docs.ps1

# Go
go mod verify
go vet ./...
go test ./...
go test -race ./...
golangci-lint run
govulncheck ./...

# Frontend
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run test:coverage
npm --prefix web run build
npm --prefix web audit --audit-level=high

# Packaging
docker compose config --quiet
docker build -t offerpilot-api .
docker build -t offerpilot-web web
```

The committed CI workflow runs documentation/Compose checks, Go formatting/vet/tests/race/lint/vulnerability scanning, frontend install/audit/lint/typecheck/tests/build, and both container builds with read-only repository permissions.

## Measured Evidence

Measured on `2026-07-17T21:55:15Z` using Windows 11 Pro `10.0.26200`, Intel Core i7-13650HX (20 logical processors), 15.7 GB visible memory, Go 1.26.5, Node 22.16.0, npm 10.9.2, and Docker 29.5.3.

External public-API simulation configuration:

```text
seed=20260717 rate=50 max_decisions=1000 workers=8 max_errors=100
```

The simulator binary was built with `go build -trimpath -o "$env:TEMP\offerpilot-simulator-benchmark.exe" ./cmd/simulator` and invoked against a fresh adaptive demo experiment on a clean Compose stack.

| Measurement | Result |
| --- | ---: |
| Attempts / decisions / outcomes | `1000 / 1000 / 1000` |
| Terminal errors | `0` |
| Wall-clock elapsed | `22.932 s` |
| Observed reward sum | `109.25` |
| Random expected reward sum | `98.2152` |
| Oracle expected reward sum | `148.0313` |
| Decision HTTP observations | `1000` |
| Decision HTTP mean | `3.315 ms` |
| Decision HTTP p95 histogram upper bound | `<= 5 ms` (`983/1000` in the 5 ms cumulative bucket) |

The p95 value is a Prometheus histogram bucket bound, not an exact percentile. The random/oracle sums are synthetic profile expectations, not causal production uplift. External simulator traffic intentionally does not enter dashboard benchmark attribution; the screenshots use a separate in-process run with seed `20260717` and 500 outcomes.

Frontend validation produced 9 passing integration/client tests and V8 coverage of 83.74% statements, 76.72% branches, 83.22% functions, and 84.91% lines. Browser checks covered 1440, 1280, 768, and 320 pixel widths, keyboard focus, reduced motion, contrast, nonblank chart paths, internal table scrolling, and zero root overflow.

## Known Limitations

- All offers, contexts, and outcomes are fictional and synthetic.
- Policy ownership is single-replica; horizontal API scaling requires a new ownership protocol.
- Authentication and multi-tenant authorization are outside the MVP.
- Decisions and outcomes have no automatic retention job.
- The dashboard uses polling rather than sockets.
- Recharts 2.15.4 is pinned because its React 19 declarations pass the repository's strict TypeScript configuration without `skipLibCheck`; migration to Recharts 3 is deferred until its transitive declarations are compatible.
- OPE estimates evaluate logged data under the current candidate policy; they do not establish causal lift in a real marketplace.

## Repository Map

- [Product and architecture documentation](docs/README.md)
- [File manifest: 70/70 validated](docs/13-file-manifest.md)
- [Testing strategy](docs/09-testing.md)
- [Security and privacy](docs/07-security-privacy.md)
- [Frontend contract](docs/10-frontend.md)
- [Editorial design reference](DESIGN.md)
- [Coding-agent playbook](docs/14-coding-agent-playbook.md)

## Status

| Area | Status |
| --- | --- |
| Product, architecture, API, data, security docs | Validated |
| Manifest implementation | `70 / 70` validated |
| Go tests and race detector | Passing |
| Frontend lint, typecheck, tests, coverage, build | Passing |
| OpenAPI response contract tests | Passing |
| PostgreSQL 18 migrations/integration tests | Passing |
| Hardened API/web images and clean Compose startup | Passing |
| PS5.1 and PowerShell 7 Linux smoke paths | Passing |
| CI workflow | Implemented and actionlint-validated |

Licensed under the [MIT License](LICENSE).
