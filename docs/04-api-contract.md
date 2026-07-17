# API Contract

## Conventions

- Base path: `/v1`.
- JSON fields use `snake_case`.
- Timestamps use RFC 3339 UTC strings.
- Identifiers use UUID strings.
- Unknown JSON fields are rejected on write requests.
- Request bodies are limited to 1 MiB.
- Every response includes `X-Request-ID`; a valid incoming value may be reused.
- Errors use `application/problem+json` with stable machine-readable codes.

Example problem response:

```json
{
  "type": "https://offerpilot.local/problems/invalid-context",
  "title": "Invalid session context",
  "status": 422,
  "code": "invalid_context",
  "detail": "device_class must be one of mobile, desktop, tablet",
  "request_id": "2d95d530-e877-4f1e-a875-df52c163ef64"
}
```

## Operational Endpoints

### `GET /health/live`

Returns `200` when the process is alive. It does not check PostgreSQL.

### `GET /health/ready`

Returns `200` only when PostgreSQL is reachable, migrations are compatible, demo bootstrap completed, and active policy state is recoverable. Otherwise returns `503`.

### `GET /metrics`

Returns Prometheus text exposition. It is not placed under `/v1`.

## Demo Bootstrap

### `POST /v1/demo/experiments`

Creates a fresh experiment with the documented fictional offer catalog. Historical experiments remain unchanged.

Request:

```json
{
  "name": "Evening marketplace demo",
  "policy_kind": "segmented_epsilon_greedy",
  "epsilon": 0.15
}
```

Response: `201 Created` with the experiment resource and `Location` header. Supported policy kinds are `random` and `segmented_epsilon_greedy`. Epsilon is required only for the adaptive policy.

## Experiments

### `GET /v1/experiments`

Returns experiments newest first. MVP pagination uses `limit` from 1 to 100 and an opaque `cursor`.

### `GET /v1/experiments/{experiment_id}`

Returns experiment configuration, active fictional offers, current policy version, and latest simulation status.

### `GET /v1/experiments/{experiment_id}/summary`

Returns aggregate counts, reward, outcome rates, offer selection distribution, current empirical means, policy version, p50/p95 policy-selection latency, simulation-only random/oracle benchmarks when available, a bounded cumulative-average learning series, and IPS/SNIPS estimates when enough valid logged data exists. The engagement-rate proxy is the persisted terminal outcomes classified as clicked or converted divided by all terminal outcomes; it is `null` with reason `no_outcomes` before the first outcome.

Each learning-series point contains a UTC bucket timestamp, cumulative sample count, and cumulative observed average reward. Random and oracle expected-average references are returned separately from the series and rendered as horizontal simulation-only references. The default maximum is 120 points; the server downsamples deterministically when needed.

Undefined statistics are represented as `null` with a reason code, never as zero.

### `GET /v1/experiments/{experiment_id}/decisions`

Returns a cursor-paginated decision feed. `limit` defaults to 50 and cannot exceed 200. Public projections omit raw internal snapshot blobs.

## Decisions

### `POST /v1/decisions`

Request:

```json
{
  "experiment_id": "742fb9d5-2b29-475f-b593-cbae1c748548",
  "context": {
    "device_class": "mobile",
    "daypart": "evening",
    "category_affinity": "travel",
    "visitor_type": "returning"
  }
}
```

Response: `201 Created`.

```json
{
  "decision_id": "7923cfb7-0edc-4f84-a624-68457f45bb38",
  "experiment_id": "742fb9d5-2b29-475f-b593-cbae1c748548",
  "selected_offer": {
    "id": "f2dd0caa-99d7-4f58-bbae-c440969304f1",
    "slug": "northstar-travel",
    "merchant_name": "Northstar Travel",
    "title": "Weekend fare drop",
    "category": "travel"
  },
  "propensity": 0.79,
  "distribution": [
    {"offer_id": "f2dd0caa-99d7-4f58-bbae-c440969304f1", "probability": 0.79},
    {"offer_id": "4e101797-6ea7-4481-8282-2c07ff2bca29", "probability": 0.07},
    {"offer_id": "43039a27-4750-402d-839f-82dc5f147709", "probability": 0.07},
    {"offer_id": "9892c51a-bbca-4d4b-b7dd-d164c2386a20", "probability": 0.07}
  ],
  "policy_kind": "segmented_epsilon_greedy",
  "policy_version": 84,
  "created_at": "2026-07-17T19:20:30Z"
}
```

The endpoint rejects non-running experiments, fewer than two active offers, invalid context, unknown fields including any client-supplied simulation-run ID, or invalid policy output. A database failure never returns a decision that was not persisted.

## Outcomes

### `POST /v1/outcomes`

Request:

```json
{
  "event_id": "32ac570c-ec1e-4f06-922d-58e636ebff63",
  "decision_id": "7923cfb7-0edc-4f84-a624-68457f45bb38",
  "outcome": "converted",
  "occurred_at": "2026-07-17T19:20:34Z"
}
```

Response: `201 Created` for a new outcome and `200 OK` with the original resource for an exact retry. A second distinct event for the same decision returns `409 outcome_already_recorded`. Future timestamps beyond the configured `OFFERPILOT_OUTCOME_MAX_FUTURE_SKEW` allowance are rejected.

The server maps outcome to reward and returns the applied policy version.

## Simulation Runs

### `POST /v1/experiments/{experiment_id}/simulation-runs`

Starts one bounded in-process simulation.

```json
{
  "seed": 20260717,
  "requests_per_second": 20,
  "max_decisions": 5000
}
```

Rates are clamped to documented safe limits. If another run is active, return `409 simulation_already_running`.

### `GET /v1/simulation-runs/{run_id}`

Returns status, seed, rate, counters, observed reward, simulation-only random/oracle expected rewards, start/stop timestamps, and terminal error summary.

### `POST /v1/simulation-runs/{run_id}/stop`

Requests cancellation and returns `202 Accepted` while stopping. Repeated stop requests are idempotent.

## OpenAPI

The implementation must maintain `openapi/openapi.yaml` as the machine-readable version of this contract. Contract tests verify representative requests and responses against it.
