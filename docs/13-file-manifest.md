# File Manifest

This is the authoritative list of manually authored implementation and configuration files in the MVP. Each planned file maps to exactly one specification. Generated artifacts are listed separately in [generated-files.md](generated-files.md).

Status progression is `specified` → `in_progress` → `implemented` → `validated`. The manifest row and the spec's `## Status` value must change together.

## Phase 1: Repository Foundation

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| F001 | `.gitignore` | [spec](file-specs/.gitignore.md) | `validated` |
| F002 | `.env.example` | [spec](file-specs/.env.example.md) | `validated` |
| F003 | `LICENSE` | [spec](file-specs/LICENSE.md) | `validated` |
| F004 | `go.mod` | [spec](file-specs/go.mod.md) | `validated` |
| F005 | `.golangci.yml` | [spec](file-specs/.golangci.yml.md) | `validated` |
| F006 | `scripts/validate-docs.ps1` | [spec](file-specs/scripts/validate-docs.ps1.md) | `validated` |
| F007 | `web/package.json` | [spec](file-specs/web/package.json.md) | `validated` |
| F008 | `web/tsconfig.json` | [spec](file-specs/web/tsconfig.json.md) | `validated` |
| F009 | `web/vite.config.ts` | [spec](file-specs/web/vite.config.ts.md) | `validated` |
| F010 | `web/eslint.config.js` | [spec](file-specs/web/eslint.config.js.md) | `validated` |
| F011 | `web/index.html` | [spec](file-specs/web/index.html.md) | `validated` |

## Phase 2: Domain And Learning Core

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| C001 | `internal/config/config.go` | [spec](file-specs/internal/config/config.go.md) | `validated` |
| C012 | `internal/config/config_test.go` | [spec](file-specs/internal/config/config_test.go.md) | `validated` |
| C002 | `internal/domain/model.go` | [spec](file-specs/internal/domain/model.go.md) | `validated` |
| C003 | `internal/domain/validate.go` | [spec](file-specs/internal/domain/validate.go.md) | `validated` |
| C004 | `internal/domain/validate_test.go` | [spec](file-specs/internal/domain/validate_test.go.md) | `validated` |
| C005 | `internal/bandit/policy.go` | [spec](file-specs/internal/bandit/policy.go.md) | `validated` |
| C006 | `internal/bandit/random.go` | [spec](file-specs/internal/bandit/random.go.md) | `validated` |
| C007 | `internal/bandit/random_test.go` | [spec](file-specs/internal/bandit/random_test.go.md) | `validated` |
| C008 | `internal/bandit/epsilon_greedy.go` | [spec](file-specs/internal/bandit/epsilon_greedy.go.md) | `validated` |
| C009 | `internal/bandit/epsilon_greedy_test.go` | [spec](file-specs/internal/bandit/epsilon_greedy_test.go.md) | `validated` |
| C010 | `internal/evaluation/ope.go` | [spec](file-specs/internal/evaluation/ope.go.md) | `validated` |
| C011 | `internal/evaluation/ope_test.go` | [spec](file-specs/internal/evaluation/ope_test.go.md) | `validated` |

## Phase 3: Persistence And Services

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| P001 | `migrations/000001_initial.up.sql` | [spec](file-specs/migrations/000001_initial.up.sql.md) | `validated` |
| P002 | `migrations/000001_initial.down.sql` | [spec](file-specs/migrations/000001_initial.down.sql.md) | `validated` |
| P010 | `migrations/embed.go` | [spec](file-specs/migrations/embed.go.md) | `validated` |
| P003 | `internal/storage/postgres/store.go` | [spec](file-specs/internal/storage/postgres/store.go.md) | `validated` |
| P004 | `internal/storage/postgres/repository.go` | [spec](file-specs/internal/storage/postgres/repository.go.md) | `validated` |
| P005 | `internal/storage/postgres/store_test.go` | [spec](file-specs/internal/storage/postgres/store_test.go.md) | `validated` |
| P006 | `internal/service/engine.go` | [spec](file-specs/internal/service/engine.go.md) | `validated` |
| P007 | `internal/service/summary.go` | [spec](file-specs/internal/service/summary.go.md) | `validated` |
| P008 | `internal/service/recovery.go` | [spec](file-specs/internal/service/recovery.go.md) | `validated` |
| P009 | `internal/service/engine_test.go` | [spec](file-specs/internal/service/engine_test.go.md) | `validated` |

## Phase 4: HTTP API And Operations

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| A001 | `openapi/openapi.yaml` | [spec](file-specs/openapi/openapi.yaml.md) | `validated` |
| A002 | `internal/observability/observability.go` | [spec](file-specs/internal/observability/observability.go.md) | `validated` |
| A009 | `internal/observability/observability_test.go` | [spec](file-specs/internal/observability/observability_test.go.md) | `validated` |
| A003 | `internal/httpapi/dto.go` | [spec](file-specs/internal/httpapi/dto.go.md) | `validated` |
| A004 | `internal/httpapi/problem.go` | [spec](file-specs/internal/httpapi/problem.go.md) | `validated` |
| A005 | `internal/httpapi/handlers.go` | [spec](file-specs/internal/httpapi/handlers.go.md) | `validated` |
| A006 | `internal/httpapi/router.go` | [spec](file-specs/internal/httpapi/router.go.md) | `validated` |
| A007 | `internal/httpapi/handlers_test.go` | [spec](file-specs/internal/httpapi/handlers_test.go.md) | `validated` |
| A008 | `cmd/api/main.go` | [spec](file-specs/cmd/api/main.go.md) | `validated` |

## Phase 5: Simulation And Demo

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| S001 | `internal/simulation/profiles.go` | [spec](file-specs/internal/simulation/profiles.go.md) | `validated` |
| S002 | `internal/simulation/runner.go` | [spec](file-specs/internal/simulation/runner.go.md) | `validated` |
| S003 | `internal/simulation/http_client.go` | [spec](file-specs/internal/simulation/http_client.go.md) | `validated` |
| S004 | `internal/simulation/manager.go` | [spec](file-specs/internal/simulation/manager.go.md) | `validated` |
| S005 | `internal/simulation/runner_test.go` | [spec](file-specs/internal/simulation/runner_test.go.md) | `validated` |
| S006 | `internal/bootstrap/demo.go` | [spec](file-specs/internal/bootstrap/demo.go.md) | `validated` |
| S007 | `internal/bootstrap/demo_test.go` | [spec](file-specs/internal/bootstrap/demo_test.go.md) | `validated` |
| S008 | `cmd/simulator/main.go` | [spec](file-specs/cmd/simulator/main.go.md) | `validated` |

## Phase 6: Dashboard

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| W001 | `web/src/types/api.ts` | [spec](file-specs/web/src/types/api.ts.md) | `validated` |
| W002 | `web/src/api/client.ts` | [spec](file-specs/web/src/api/client.ts.md) | `validated` |
| W003 | `web/src/hooks/useDashboard.ts` | [spec](file-specs/web/src/hooks/useDashboard.ts.md) | `validated` |
| W004 | `web/src/components/MetricStrip.tsx` | [spec](file-specs/web/src/components/MetricStrip.tsx.md) | `validated` |
| W005 | `web/src/components/SimulationControls.tsx` | [spec](file-specs/web/src/components/SimulationControls.tsx.md) | `validated` |
| W006 | `web/src/components/LearningChart.tsx` | [spec](file-specs/web/src/components/LearningChart.tsx.md) | `validated` |
| W007 | `web/src/components/OfferPerformanceTable.tsx` | [spec](file-specs/web/src/components/OfferPerformanceTable.tsx.md) | `validated` |
| W008 | `web/src/components/DecisionFeed.tsx` | [spec](file-specs/web/src/components/DecisionFeed.tsx.md) | `validated` |
| W009 | `web/src/App.tsx` | [spec](file-specs/web/src/App.tsx.md) | `validated` |
| W010 | `web/src/styles.css` | [spec](file-specs/web/src/styles.css.md) | `validated` |
| W011 | `web/src/main.tsx` | [spec](file-specs/web/src/main.tsx.md) | `validated` |
| W012 | `web/src/test/setup.ts` | [spec](file-specs/web/src/test/setup.ts.md) | `validated` |
| W013 | `web/src/App.test.tsx` | [spec](file-specs/web/src/App.test.tsx.md) | `validated` |

## Phase 7: Packaging And Evidence

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| R001 | `Dockerfile` | [spec](file-specs/root-container.md) | `validated` |
| R002 | `web/Dockerfile` | [spec](file-specs/web-container.md) | `validated` |
| R006 | `.dockerignore` | [spec](file-specs/root-dockerignore.md) | `validated` |
| R007 | `web/.dockerignore` | [spec](file-specs/web-dockerignore.md) | `validated` |
| R003 | `docker-compose.yml` | [spec](file-specs/docker-compose.yml.md) | `validated` |
| R004 | `.github/workflows/ci.yml` | [spec](file-specs/.github/workflows/ci.yml.md) | `validated` |
| R005 | `scripts/smoke.ps1` | [spec](file-specs/scripts/smoke.ps1.md) | `validated` |

## Manifest Rules

- Do not add a row without first adding the governing architecture/ADR change and matching spec.
- Do not create a planned file while its status is only `specified`; set both manifest and spec to `in_progress` in the same initial edit.
- Set `implemented` only when the file's required behavior exists.
- Set `validated` only after every command in the file spec succeeds.
- Tool-generated `go.sum`, `web/package-lock.json`, `web/dist/**`, and coverage output are governed by [generated-files.md](generated-files.md), not this manifest.
