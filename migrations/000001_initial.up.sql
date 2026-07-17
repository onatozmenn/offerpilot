-- +goose Up
CREATE TABLE experiments (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE CHECK (btrim(slug) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    status TEXT NOT NULL CHECK (status IN ('draft', 'running', 'paused', 'completed')),
    policy_kind TEXT NOT NULL CHECK (policy_kind IN ('random', 'segmented_epsilon_greedy')),
    epsilon DOUBLE PRECISION,
    policy_version BIGINT NOT NULL CHECK (policy_version >= 1),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT experiments_policy_configuration_check CHECK (
        (policy_kind = 'random' AND epsilon IS NULL)
        OR
        (policy_kind = 'segmented_epsilon_greedy' AND epsilon >= 0 AND epsilon <= 1)
    ),
    CONSTRAINT experiments_timestamps_check CHECK (updated_at >= created_at)
);

CREATE INDEX experiments_status_id_idx
    ON experiments (status, id);

CREATE INDEX experiments_created_at_id_idx
    ON experiments (created_at DESC, id DESC);

CREATE TABLE offers (
    id UUID PRIMARY KEY,
    experiment_id UUID NOT NULL REFERENCES experiments (id),
    slug TEXT NOT NULL CHECK (btrim(slug) <> ''),
    merchant_name TEXT NOT NULL CHECK (btrim(merchant_name) <> ''),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    description TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('travel', 'dining', 'wellness', 'home', 'technology', 'entertainment')),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT offers_experiment_slug_key UNIQUE (experiment_id, slug),
    CONSTRAINT offers_experiment_id_id_key UNIQUE (experiment_id, id)
);

CREATE INDEX offers_experiment_active_id_idx
    ON offers (experiment_id, active, id);

CREATE TABLE simulation_runs (
    id UUID PRIMARY KEY,
    experiment_id UUID NOT NULL REFERENCES experiments (id),
    seed BIGINT NOT NULL,
    requests_per_second INTEGER NOT NULL CHECK (requests_per_second BETWEEN 1 AND 100),
    max_decisions INTEGER NOT NULL CHECK (max_decisions BETWEEN 1 AND 100000),
    status TEXT NOT NULL CHECK (status IN ('starting', 'running', 'stopping', 'completed', 'failed', 'cancelled')),
    decision_count BIGINT NOT NULL DEFAULT 0 CHECK (decision_count >= 0),
    outcome_count BIGINT NOT NULL DEFAULT 0 CHECK (outcome_count >= 0),
    error_count BIGINT NOT NULL DEFAULT 0 CHECK (error_count >= 0),
    observed_reward_sum DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (
        observed_reward_sum >= 0 AND observed_reward_sum < 'Infinity'::DOUBLE PRECISION
    ),
    random_expected_reward_sum DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (
        random_expected_reward_sum >= 0 AND random_expected_reward_sum < 'Infinity'::DOUBLE PRECISION
    ),
    oracle_expected_reward_sum DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (
        oracle_expected_reward_sum >= 0 AND oracle_expected_reward_sum < 'Infinity'::DOUBLE PRECISION
    ),
    started_at TIMESTAMPTZ NOT NULL,
    stopped_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    error_code TEXT,
    error_detail TEXT,
    CONSTRAINT simulation_runs_experiment_id_id_key UNIQUE (experiment_id, id),
    CONSTRAINT simulation_runs_counts_check CHECK (outcome_count <= decision_count),
    CONSTRAINT simulation_runs_timestamps_check CHECK (
        updated_at >= started_at AND (stopped_at IS NULL OR stopped_at >= started_at)
    ),
    CONSTRAINT simulation_runs_terminal_timestamp_check CHECK (
        (status IN ('starting', 'running', 'stopping') AND stopped_at IS NULL)
        OR
        (status IN ('completed', 'failed', 'cancelled') AND stopped_at IS NOT NULL)
    ),
    CONSTRAINT simulation_runs_error_detail_check CHECK (error_detail IS NULL OR error_code IS NOT NULL)
);

CREATE UNIQUE INDEX simulation_runs_one_active_per_experiment_idx
    ON simulation_runs (experiment_id)
    WHERE status IN ('starting', 'running', 'stopping');

CREATE INDEX simulation_runs_experiment_started_id_idx
    ON simulation_runs (experiment_id, started_at DESC, id DESC);

CREATE INDEX simulation_runs_status_updated_idx
    ON simulation_runs (status, updated_at);

CREATE TABLE decisions (
    id UUID PRIMARY KEY,
    experiment_id UUID NOT NULL REFERENCES experiments (id),
    selected_offer_id UUID NOT NULL,
    context JSONB NOT NULL CHECK (
        jsonb_typeof(context) = 'object'
        AND context ?& ARRAY['device_class', 'daypart', 'category_affinity', 'visitor_type']
        AND context - ARRAY['device_class', 'daypart', 'category_affinity', 'visitor_type'] = '{}'::JSONB
        AND context ->> 'device_class' IN ('mobile', 'desktop', 'tablet')
        AND context ->> 'daypart' IN ('morning', 'afternoon', 'evening', 'night')
        AND context ->> 'category_affinity' IN ('travel', 'dining', 'wellness', 'home', 'technology', 'entertainment')
        AND context ->> 'visitor_type' IN ('new', 'returning')
    ),
    segment_key TEXT NOT NULL CHECK (btrim(segment_key) <> ''),
    eligible_offer_ids UUID[] NOT NULL CHECK (
        cardinality(eligible_offer_ids) >= 2
        AND array_position(eligible_offer_ids, NULL) IS NULL
        AND selected_offer_id = ANY (eligible_offer_ids)
    ),
    distribution JSONB NOT NULL CHECK (
        jsonb_typeof(distribution) = 'array'
        AND jsonb_array_length(distribution) = cardinality(eligible_offer_ids)
    ),
    propensity DOUBLE PRECISION NOT NULL CHECK (propensity > 0 AND propensity <= 1),
    policy_kind TEXT NOT NULL CHECK (policy_kind IN ('random', 'segmented_epsilon_greedy')),
    policy_version BIGINT NOT NULL CHECK (policy_version >= 1),
    policy_latency_micros BIGINT NOT NULL CHECK (policy_latency_micros >= 0),
    simulation_run_id UUID,
    request_id TEXT NOT NULL CHECK (btrim(request_id) <> ''),
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT decisions_selected_offer_fk
        FOREIGN KEY (experiment_id, selected_offer_id)
        REFERENCES offers (experiment_id, id),
    CONSTRAINT decisions_simulation_run_fk
        FOREIGN KEY (experiment_id, simulation_run_id)
        REFERENCES simulation_runs (experiment_id, id)
);

CREATE INDEX decisions_experiment_created_id_idx
    ON decisions (experiment_id, created_at DESC, id DESC);

CREATE INDEX decisions_simulation_run_created_idx
    ON decisions (simulation_run_id, created_at, id)
    WHERE simulation_run_id IS NOT NULL;

CREATE TABLE outcomes (
    event_id UUID PRIMARY KEY,
    decision_id UUID NOT NULL UNIQUE REFERENCES decisions (id),
    kind TEXT NOT NULL CHECK (kind IN ('ignored', 'clicked', 'converted')),
    reward DOUBLE PRECISION NOT NULL CHECK (
        (kind = 'ignored' AND reward = 0)
        OR
        (kind = 'clicked' AND reward = 0.25)
        OR
        (kind = 'converted' AND reward = 1)
    ),
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    applied_policy_version BIGINT NOT NULL CHECK (applied_policy_version >= 1)
);

CREATE INDEX outcomes_applied_policy_version_idx
    ON outcomes (applied_policy_version);

CREATE INDEX outcomes_received_decision_idx
    ON outcomes (received_at, decision_id);

CREATE TABLE policy_snapshots (
    experiment_id UUID NOT NULL REFERENCES experiments (id),
    policy_version BIGINT NOT NULL CHECK (policy_version >= 1),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    state JSONB NOT NULL CHECK (jsonb_typeof(state) = 'object'),
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (experiment_id, policy_version)
);

CREATE INDEX policy_snapshots_experiment_version_desc_idx
    ON policy_snapshots (experiment_id, policy_version DESC);