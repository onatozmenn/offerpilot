# Data Model

## Principles

- PostgreSQL is the source of truth.
- Historical decision and outcome records are append-only.
- JSONB is used only for bounded context, distributions, and versioned policy snapshots.
- Foreign keys and unique constraints enforce core idempotency.
- Application and database timestamps are UTC.

## Tables

### `experiments`

| Column | Type | Constraints |
| --- | --- | --- |
| `id` | UUID | Primary key |
| `slug` | TEXT | Unique, non-empty |
| `name` | TEXT | Non-empty |
| `status` | TEXT | Checked enum |
| `policy_kind` | TEXT | Checked enum |
| `epsilon` | DOUBLE PRECISION | Nullable; finite range enforced by app and check |
| `policy_version` | BIGINT | At least 1 |
| `created_at` | TIMESTAMPTZ | Required |
| `updated_at` | TIMESTAMPTZ | Required |

### `offers`

Contains experiment-scoped fictional merchant offers. Unique constraint on `(experiment_id, slug)`. Index active offers by `(experiment_id, active, id)`.

### `decisions`

Contains immutable context, segment key, eligible offer IDs, selected offer, distribution, propensity, policy kind/version, non-negative `policy_latency_micros`, optional simulation run, request ID, and creation time.

Required checks:

- Propensity is greater than zero and at most one.
- Distribution and context are JSON objects/arrays of expected top-level types.
- Selected offer belongs to the same experiment through application validation.

Indexes support recent experiment decisions and simulation-run aggregation.

### `outcomes`

Contains unique `event_id`, unique `decision_id`, outcome enum, reward, occurrence/receipt time, and applied policy version. The two unique constraints make exact retries and competing terminal outcomes distinguishable.

### `policy_snapshots`

Contains experiment ID, policy version, snapshot schema version, JSONB state, and creation time. Primary key is `(experiment_id, policy_version)`. The latest version is loaded at startup.

### `simulation_runs`

Contains seed, requested rate, maximum decisions, status, counters, observed reward sum, uniform-random expected reward sum, oracle expected reward sum, timestamps, and nullable terminal error code/detail. All reward sums are finite, non-negative, and explicitly labeled simulation-only. A partial unique index permits only one active run per experiment.

At API startup, any run left in `starting`, `running`, or `stopping` is transitioned atomically to `failed` with `process_restarted` before new runs or readiness. Partial counters and reward sums remain available for inspection but are not presented as completed benchmarks.

## Transaction Boundaries

### Create Decision

Insert one decision only after policy output validation. No policy state changes occur in this transaction.

### Create Experiment

Insert the experiment, fictional offers, and initial version-one policy snapshot in one transaction. Policy kind and epsilon are immutable after this commit.

### Accept Outcome

The storage transaction:

1. Locks the experiment row and reads its current application version, then locks the decision row.
2. Returns the existing outcome for an exact event retry.
3. Rejects a competing event if a terminal outcome exists.
4. Reserves `current_version + 1`, updates the experiment version, and inserts the outcome with that applied version.
5. Commits before the in-memory policy update. The decision's selection version is not required to equal the current application version.

After commit, the service applies the update and stores the matching snapshot. An outcome/snapshot version gap makes readiness fail and triggers deterministic recovery.

## Recovery

On startup, load the latest snapshot and replay accepted outcomes whose applied version is greater than the snapshot version, ordered by applied version. Gaps or duplicates are corruption errors. A crash after in-memory update but before snapshot persistence simply replays that outcome from the older durable snapshot; a snapshot already present at the applied version is treated as complete. The recovered state is checkpointed idempotently before readiness succeeds.

## Migration Rules

- Migrations are forward-only in shared environments but include a local down migration for the initial schema.
- Never edit an applied migration; add a numbered migration.
- Migration SQL must be transaction-safe.
- Demo data is inserted by Go bootstrap code, not hard-coded into schema migrations.
- UUID columns use PostgreSQL's native `UUID` type. Application-generated UUIDs require no database extension.
- Migration SQL is embedded by a dedicated Go file in the root `migrations` package so local and container execution use identical bytes.

## Retention

The MVP does not delete decisions or outcomes automatically. The README must state that all data is synthetic. A future retention job requires an ADR.
