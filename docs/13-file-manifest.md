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
| F011 | `web/index.html` | [spec](file-specs/web/index.html.md) | `specified` |

## Phase 2: Domain And Learning Core

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| C001 | `internal/config/config.go` | [spec](file-specs/internal/config/config.go.md) | `specified` |
| C012 | `internal/config/config_test.go` | [spec](file-specs/internal/config/config_test.go.md) | `specified` |
| C002 | `internal/domain/model.go` | [spec](file-specs/internal/domain/model.go.md) | `specified` |
| C003 | `internal/domain/validate.go` | [spec](file-specs/internal/domain/validate.go.md) | `specified` |
| C004 | `internal/domain/validate_test.go` | [spec](file-specs/internal/domain/validate_test.go.md) | `specified` |
| C005 | `internal/bandit/policy.go` | [spec](file-specs/internal/bandit/policy.go.md) | `specified` |
| C006 | `internal/bandit/random.go` | [spec](file-specs/internal/bandit/random.go.md) | `specified` |
| C007 | `internal/bandit/random_test.go` | [spec](file-specs/internal/bandit/random_test.go.md) | `specified` |
| C008 | `internal/bandit/epsilon_greedy.go` | [spec](file-specs/internal/bandit/epsilon_greedy.go.md) | `specified` |
| C009 | `internal/bandit/epsilon_greedy_test.go` | [spec](file-specs/internal/bandit/epsilon_greedy_test.go.md) | `specified` |
| C010 | `internal/evaluation/ope.go` | [spec](file-specs/internal/evaluation/ope.go.md) | `specified` |
| C011 | `internal/evaluation/ope_test.go` | [spec](file-specs/internal/evaluation/ope_test.go.md) | `specified` |

## Phase 3: Persistence And Services

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| P001 | `migrations/000001_initial.up.sql` | [spec](file-specs/migrations/000001_initial.up.sql.md) | `specified` |
| P002 | `migrations/000001_initial.down.sql` | [spec](file-specs/migrations/000001_initial.down.sql.md) | `specified` |
| P010 | `migrations/embed.go` | [spec](file-specs/migrations/embed.go.md) | `specified` |
| P003 | `internal/storage/postgres/store.go` | [spec](file-specs/internal/storage/postgres/store.go.md) | `specified` |
| P004 | `internal/storage/postgres/repository.go` | [spec](file-specs/internal/storage/postgres/repository.go.md) | `specified` |
| P005 | `internal/storage/postgres/store_test.go` | [spec](file-specs/internal/storage/postgres/store_test.go.md) | `specified` |
| P006 | `internal/service/engine.go` | [spec](file-specs/internal/service/engine.go.md) | `specified` |
| P007 | `internal/service/summary.go` | [spec](file-specs/internal/service/summary.go.md) | `specified` |
| P008 | `internal/service/recovery.go` | [spec](file-specs/internal/service/recovery.go.md) | `specified` |
| P009 | `internal/service/engine_test.go` | [spec](file-specs/internal/service/engine_test.go.md) | `specified` |

## Phase 4: HTTP API And Operations

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| A001 | `openapi/openapi.yaml` | [spec](file-specs/openapi/openapi.yaml.md) | `specified` |
| A002 | `internal/observability/observability.go` | [spec](file-specs/internal/observability/observability.go.md) | `specified` |
| A009 | `internal/observability/observability_test.go` | [spec](file-specs/internal/observability/observability_test.go.md) | `specified` |
| A003 | `internal/httpapi/dto.go` | [spec](file-specs/internal/httpapi/dto.go.md) | `specified` |
| A004 | `internal/httpapi/problem.go` | [spec](file-specs/internal/httpapi/problem.go.md) | `specified` |
| A005 | `internal/httpapi/handlers.go` | [spec](file-specs/internal/httpapi/handlers.go.md) | `specified` |
| A006 | `internal/httpapi/router.go` | [spec](file-specs/internal/httpapi/router.go.md) | `specified` |
| A007 | `internal/httpapi/handlers_test.go` | [spec](file-specs/internal/httpapi/handlers_test.go.md) | `specified` |
| A008 | `cmd/api/main.go` | [spec](file-specs/cmd/api/main.go.md) | `specified` |

## Phase 5: Simulation And Demo

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| S001 | `internal/simulation/profiles.go` | [spec](file-specs/internal/simulation/profiles.go.md) | `specified` |
| S002 | `internal/simulation/runner.go` | [spec](file-specs/internal/simulation/runner.go.md) | `specified` |
| S003 | `internal/simulation/http_client.go` | [spec](file-specs/internal/simulation/http_client.go.md) | `specified` |
| S004 | `internal/simulation/manager.go` | [spec](file-specs/internal/simulation/manager.go.md) | `specified` |
| S005 | `internal/simulation/runner_test.go` | [spec](file-specs/internal/simulation/runner_test.go.md) | `specified` |
| S006 | `internal/bootstrap/demo.go` | [spec](file-specs/internal/bootstrap/demo.go.md) | `specified` |
| S007 | `internal/bootstrap/demo_test.go` | [spec](file-specs/internal/bootstrap/demo_test.go.md) | `specified` |
| S008 | `cmd/simulator/main.go` | [spec](file-specs/cmd/simulator/main.go.md) | `specified` |

## Phase 6: Dashboard

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| W001 | `web/src/types/api.ts` | [spec](file-specs/web/src/types/api.ts.md) | `specified` |
| W002 | `web/src/api/client.ts` | [spec](file-specs/web/src/api/client.ts.md) | `specified` |
| W003 | `web/src/hooks/useDashboard.ts` | [spec](file-specs/web/src/hooks/useDashboard.ts.md) | `specified` |
| W004 | `web/src/components/MetricStrip.tsx` | [spec](file-specs/web/src/components/MetricStrip.tsx.md) | `specified` |
| W005 | `web/src/components/SimulationControls.tsx` | [spec](file-specs/web/src/components/SimulationControls.tsx.md) | `specified` |
| W006 | `web/src/components/LearningChart.tsx` | [spec](file-specs/web/src/components/LearningChart.tsx.md) | `specified` |
| W007 | `web/src/components/OfferPerformanceTable.tsx` | [spec](file-specs/web/src/components/OfferPerformanceTable.tsx.md) | `specified` |
| W008 | `web/src/components/DecisionFeed.tsx` | [spec](file-specs/web/src/components/DecisionFeed.tsx.md) | `specified` |
| W009 | `web/src/App.tsx` | [spec](file-specs/web/src/App.tsx.md) | `specified` |
| W010 | `web/src/styles.css` | [spec](file-specs/web/src/styles.css.md) | `specified` |
| W011 | `web/src/main.tsx` | [spec](file-specs/web/src/main.tsx.md) | `specified` |
| W012 | `web/src/test/setup.ts` | [spec](file-specs/web/src/test/setup.ts.md) | `specified` |
| W013 | `web/src/App.test.tsx` | [spec](file-specs/web/src/App.test.tsx.md) | `specified` |

## Phase 7: Packaging And Evidence

| ID | Planned file | Specification | Status |
| --- | --- | --- | --- |
| R001 | `Dockerfile` | [spec](file-specs/root-container.md) | `specified` |
| R002 | `web/Dockerfile` | [spec](file-specs/web-container.md) | `specified` |
| R006 | `.dockerignore` | [spec](file-specs/root-dockerignore.md) | `specified` |
| R007 | `web/.dockerignore` | [spec](file-specs/web-dockerignore.md) | `specified` |
| R003 | `docker-compose.yml` | [spec](file-specs/docker-compose.yml.md) | `specified` |
| R004 | `.github/workflows/ci.yml` | [spec](file-specs/.github/workflows/ci.yml.md) | `specified` |
| R005 | `scripts/smoke.ps1` | [spec](file-specs/scripts/smoke.ps1.md) | `specified` |

## Manifest Rules

- Do not add a row without first adding the governing architecture/ADR change and matching spec.
- Do not create a planned file while its status is only `specified`; set both manifest and spec to `in_progress` in the same initial edit.
- Set `implemented` only when the file's required behavior exists.
- Set `validated` only after every command in the file spec succeeds.
- Tool-generated `go.sum`, `web/package-lock.json`, `web/dist/**`, and coverage output are governed by [generated-files.md](generated-files.md), not this manifest.
