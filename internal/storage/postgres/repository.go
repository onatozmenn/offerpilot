package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/service"
)

type rowScanner interface {
	Scan(...any) error
}

func (store *Store) CreateExperiment(
	ctx context.Context,
	experiment domain.Experiment,
	offers []domain.Offer,
	snapshot domain.PolicySnapshot,
) error {
	if err := domain.ValidateExperiment(experiment); err != nil {
		return fmt.Errorf("validate experiment: %w", err)
	}
	if err := domain.ValidateOffers(experiment.ID, offers); err != nil {
		return fmt.Errorf("validate offers: %w", err)
	}
	if err := domain.ValidatePolicySnapshot(snapshot); err != nil {
		return fmt.Errorf("validate initial snapshot: %w", err)
	}
	if snapshot.ExperimentID != experiment.ID || snapshot.PolicyKind != experiment.PolicyKind || snapshot.PolicyVersion != experiment.PolicyVersion {
		return fmt.Errorf("initial snapshot does not match experiment")
	}

	return store.withTx(ctx, func(transaction pgx.Tx) error {
		_, err := transaction.Exec(ctx, `
            INSERT INTO experiments (
                id, slug, name, status, policy_kind, epsilon,
                policy_version, created_at, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        `,
			experiment.ID,
			experiment.Slug,
			experiment.Name,
			experiment.Status,
			experiment.PolicyKind,
			experiment.Epsilon,
			experiment.PolicyVersion,
			experiment.CreatedAt,
			experiment.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert experiment: %w", err)
		}

		for _, offer := range offers {
			_, err := transaction.Exec(ctx, `
                INSERT INTO offers (
                    id, experiment_id, slug, merchant_name,
                    title, description, category, active
                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
            `,
				offer.ID,
				offer.ExperimentID,
				offer.Slug,
				offer.MerchantName,
				offer.Title,
				offer.Description,
				offer.Category,
				offer.Active,
			)
			if err != nil {
				return fmt.Errorf("insert offer %s: %w", offer.ID, err)
			}
		}

		_, err = transaction.Exec(ctx, `
            INSERT INTO policy_snapshots (
                experiment_id, policy_version, schema_version, state, created_at
            ) VALUES ($1, $2, $3, $4::jsonb, $5)
        `,
			snapshot.ExperimentID,
			snapshot.PolicyVersion,
			snapshot.SchemaVersion,
			string(snapshot.State),
			snapshot.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert initial policy snapshot: %w", err)
		}

		return nil
	})
}

func (store *Store) GetExperiment(ctx context.Context, experimentID uuid.UUID) (domain.Experiment, error) {
	if err := store.ensureOpen(); err != nil {
		return domain.Experiment{}, err
	}
	experiment, err := scanExperiment(store.pool.QueryRow(ctx, `
        SELECT id, slug, name, status, policy_kind, epsilon,
               policy_version, created_at, updated_at
        FROM experiments
        WHERE id = $1
    `, experimentID))
	if err != nil {
		return domain.Experiment{}, mapRowError("get experiment", err)
	}
	return experiment, nil
}

func (store *Store) ListExperiments(
	ctx context.Context,
	cursor *service.ExperimentCursor,
	limit int,
) ([]domain.Experiment, error) {
	if err := store.ensureOpen(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("experiment limit must be between 1 and 100")
	}

	var rows pgx.Rows
	var err error
	if cursor == nil {
		rows, err = store.pool.Query(ctx, `
            SELECT id, slug, name, status, policy_kind, epsilon,
                   policy_version, created_at, updated_at
            FROM experiments
            ORDER BY created_at DESC, id DESC
            LIMIT $1
        `, limit)
	} else {
		if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
			return nil, fmt.Errorf("experiment cursor is invalid")
		}
		rows, err = store.pool.Query(ctx, `
            SELECT id, slug, name, status, policy_kind, epsilon,
                   policy_version, created_at, updated_at
            FROM experiments
            WHERE (created_at, id) < ($1, $2)
            ORDER BY created_at DESC, id DESC
            LIMIT $3
        `, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list experiments: %w", err)
	}
	defer rows.Close()

	experiments := make([]domain.Experiment, 0, limit)
	for rows.Next() {
		experiment, err := scanExperiment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan experiment: %w", err)
		}
		experiments = append(experiments, experiment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experiments: %w", err)
	}
	return experiments, nil
}

func (store *Store) ListActiveExperiments(ctx context.Context) ([]domain.Experiment, error) {
	if err := store.ensureOpen(); err != nil {
		return nil, err
	}
	rows, err := store.pool.Query(ctx, `
        SELECT id, slug, name, status, policy_kind, epsilon,
               policy_version, created_at, updated_at
        FROM experiments
        WHERE status IN ('running', 'paused')
        ORDER BY id
    `)
	if err != nil {
		return nil, fmt.Errorf("list active experiments: %w", err)
	}
	defer rows.Close()

	var experiments []domain.Experiment
	for rows.Next() {
		experiment, err := scanExperiment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active experiment: %w", err)
		}
		experiments = append(experiments, experiment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active experiments: %w", err)
	}
	return experiments, nil
}

func (store *Store) ListActiveOffers(ctx context.Context, experimentID uuid.UUID) ([]domain.Offer, error) {
	if err := store.ensureOpen(); err != nil {
		return nil, err
	}
	rows, err := store.pool.Query(ctx, `
        SELECT id, experiment_id, slug, merchant_name,
               title, description, category, active
        FROM offers
        WHERE experiment_id = $1 AND active = TRUE
        ORDER BY id
    `, experimentID)
	if err != nil {
		return nil, fmt.Errorf("list active offers: %w", err)
	}
	defer rows.Close()

	var offers []domain.Offer
	for rows.Next() {
		offer, err := scanOffer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active offer: %w", err)
		}
		offers = append(offers, offer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active offers: %w", err)
	}
	return offers, nil
}

func (store *Store) InsertDecision(ctx context.Context, decision domain.Decision) error {
	if err := store.ensureOpen(); err != nil {
		return err
	}
	if err := domain.ValidateDecision(decision); err != nil {
		return fmt.Errorf("validate decision: %w", err)
	}
	contextJSON, err := json.Marshal(decision.Context)
	if err != nil {
		return fmt.Errorf("encode decision context: %w", err)
	}
	distributionJSON, err := json.Marshal(decision.Distribution)
	if err != nil {
		return fmt.Errorf("encode decision distribution: %w", err)
	}

	_, err = store.pool.Exec(ctx, `
        INSERT INTO decisions (
            id, experiment_id, selected_offer_id, context, segment_key,
            eligible_offer_ids, distribution, propensity, policy_kind,
            policy_version, policy_latency_micros, simulation_run_id,
            request_id, created_at
        ) VALUES (
            $1, $2, $3, $4::jsonb, $5,
            $6, $7::jsonb, $8, $9,
            $10, $11, $12,
            $13, $14
        )
    `,
		decision.ID,
		decision.ExperimentID,
		decision.SelectedOfferID,
		string(contextJSON),
		decision.SegmentKey,
		decision.EligibleOfferIDs,
		string(distributionJSON),
		decision.Propensity,
		decision.PolicyKind,
		decision.PolicyVersion,
		decision.PolicyLatencyMicros,
		decision.SimulationRunID,
		decision.RequestID,
		decision.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert decision: %w", err)
	}
	return nil
}

func (store *Store) GetDecision(ctx context.Context, decisionID uuid.UUID) (domain.Decision, error) {
	if err := store.ensureOpen(); err != nil {
		return domain.Decision{}, err
	}
	decision, err := scanDecision(store.pool.QueryRow(ctx, decisionSelect+` WHERE d.id = $1`, decisionID))
	if err != nil {
		return domain.Decision{}, mapRowError("get decision", err)
	}
	return decision, nil
}

func (store *Store) ListDecisions(
	ctx context.Context,
	experimentID uuid.UUID,
	cursor *service.DecisionCursor,
	limit int,
) ([]domain.Decision, error) {
	if err := store.ensureOpen(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		return nil, fmt.Errorf("decision limit must be between 1 and 200")
	}

	var rows pgx.Rows
	var err error
	if cursor == nil {
		rows, err = store.pool.Query(ctx, decisionSelect+`
            WHERE d.experiment_id = $1
            ORDER BY d.created_at DESC, d.id DESC
            LIMIT $2
        `, experimentID, limit)
	} else {
		if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
			return nil, fmt.Errorf("decision cursor is invalid")
		}
		rows, err = store.pool.Query(ctx, decisionSelect+`
            WHERE d.experiment_id = $1
              AND (d.created_at, d.id) < ($2, $3)
            ORDER BY d.created_at DESC, d.id DESC
            LIMIT $4
        `, experimentID, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	defer rows.Close()

	decisions := make([]domain.Decision, 0, limit)
	for rows.Next() {
		decision, err := scanDecision(rows)
		if err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}
		decisions = append(decisions, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decisions: %w", err)
	}
	return decisions, nil
}

func (store *Store) AcceptOutcome(
	ctx context.Context,
	experimentID uuid.UUID,
	candidate domain.Outcome,
) (service.OutcomeAcceptance, error) {
	var acceptance service.OutcomeAcceptance
	err := store.withTx(ctx, func(transaction pgx.Tx) error {
		var currentVersion int64
		if err := transaction.QueryRow(ctx, `
            SELECT policy_version
            FROM experiments
            WHERE id = $1
            FOR UPDATE
        `, experimentID).Scan(&currentVersion); err != nil {
			return mapRowError("lock experiment for outcome", err)
		}

		var decisionExperimentID uuid.UUID
		if err := transaction.QueryRow(ctx, `
            SELECT experiment_id
            FROM decisions
            WHERE id = $1
            FOR UPDATE
        `, candidate.DecisionID).Scan(&decisionExperimentID); err != nil {
			return mapRowError("lock decision for outcome", err)
		}
		if decisionExperimentID != experimentID {
			return service.ErrNotFound
		}

		existing, err := scanOutcome(transaction.QueryRow(ctx, `
            SELECT event_id, decision_id, kind, reward,
                   occurred_at, received_at, applied_policy_version
            FROM outcomes
            WHERE event_id = $1 OR decision_id = $2
            ORDER BY CASE WHEN event_id = $1 THEN 0 ELSE 1 END
            LIMIT 1
        `, candidate.EventID, candidate.DecisionID))
		switch {
		case err == nil:
			acceptance.Outcome = existing
			if exactOutcomeRetry(existing, candidate) {
				acceptance.Status = service.OutcomeAcceptanceExactRetry
			} else {
				acceptance.Status = service.OutcomeAcceptanceConflict
			}
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("check existing outcome: %w", err)
		}

		candidate.AppliedPolicyVersion = currentVersion + 1
		commandTag, err := transaction.Exec(ctx, `
            UPDATE experiments
            SET policy_version = $2, updated_at = $3
            WHERE id = $1 AND policy_version = $4
        `, experimentID, candidate.AppliedPolicyVersion, candidate.ReceivedAt, currentVersion)
		if err != nil {
			return fmt.Errorf("reserve policy version: %w", err)
		}
		if commandTag.RowsAffected() != 1 {
			return fmt.Errorf("reserve policy version: unexpected affected rows")
		}

		_, err = transaction.Exec(ctx, `
            INSERT INTO outcomes (
                event_id, decision_id, kind, reward,
                occurred_at, received_at, applied_policy_version
            ) VALUES ($1, $2, $3, $4, $5, $6, $7)
        `,
			candidate.EventID,
			candidate.DecisionID,
			candidate.Kind,
			candidate.Reward,
			candidate.OccurredAt,
			candidate.ReceivedAt,
			candidate.AppliedPolicyVersion,
		)
		if err != nil {
			return fmt.Errorf("insert outcome: %w", err)
		}

		acceptance = service.OutcomeAcceptance{
			Status:  service.OutcomeAcceptanceCreated,
			Outcome: candidate,
		}
		return nil
	})
	if err != nil {
		return service.OutcomeAcceptance{}, err
	}
	return acceptance, nil
}

func (store *Store) SavePolicySnapshot(ctx context.Context, snapshot domain.PolicySnapshot) error {
	if err := store.ensureOpen(); err != nil {
		return err
	}
	if err := domain.ValidatePolicySnapshot(snapshot); err != nil {
		return fmt.Errorf("validate policy snapshot: %w", err)
	}

	var policyKind domain.PolicyKind
	if err := store.pool.QueryRow(ctx, `
        SELECT policy_kind FROM experiments WHERE id = $1
    `, snapshot.ExperimentID).Scan(&policyKind); err != nil {
		return mapRowError("load experiment for snapshot", err)
	}
	if policyKind != snapshot.PolicyKind {
		return fmt.Errorf("snapshot policy kind does not match experiment")
	}

	commandTag, err := store.pool.Exec(ctx, `
        INSERT INTO policy_snapshots (
            experiment_id, policy_version, schema_version, state, created_at
        ) VALUES ($1, $2, $3, $4::jsonb, $5)
        ON CONFLICT (experiment_id, policy_version) DO NOTHING
    `,
		snapshot.ExperimentID,
		snapshot.PolicyVersion,
		snapshot.SchemaVersion,
		string(snapshot.State),
		snapshot.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save policy snapshot: %w", err)
	}
	if commandTag.RowsAffected() == 1 {
		return nil
	}

	var identical bool
	if err := store.pool.QueryRow(ctx, `
        SELECT schema_version = $3
               AND state = $4::jsonb
        FROM policy_snapshots
        WHERE experiment_id = $1 AND policy_version = $2
    `,
		snapshot.ExperimentID,
		snapshot.PolicyVersion,
		snapshot.SchemaVersion,
		string(snapshot.State),
	).Scan(&identical); err != nil {
		return mapRowError("compare existing policy snapshot", err)
	}
	if !identical {
		return service.ErrSnapshotConflict
	}
	return nil
}

func (store *Store) GetLatestPolicySnapshot(ctx context.Context, experimentID uuid.UUID) (domain.PolicySnapshot, error) {
	if err := store.ensureOpen(); err != nil {
		return domain.PolicySnapshot{}, err
	}
	snapshot, err := scanPolicySnapshot(store.pool.QueryRow(ctx, `
        SELECT ps.experiment_id, e.policy_kind, ps.policy_version,
               ps.schema_version, ps.state::text, ps.created_at
        FROM policy_snapshots ps
        JOIN experiments e ON e.id = ps.experiment_id
        WHERE ps.experiment_id = $1
        ORDER BY ps.policy_version DESC
        LIMIT 1
    `, experimentID))
	if err != nil {
		return domain.PolicySnapshot{}, mapRowError("get latest policy snapshot", err)
	}
	return snapshot, nil
}

func (store *Store) ListDecisionOutcomesAfterVersion(
	ctx context.Context,
	experimentID uuid.UUID,
	afterVersion int64,
) ([]service.DecisionOutcome, error) {
	return store.listDecisionOutcomes(ctx, experimentID, &afterVersion)
}

func (store *Store) ListDecisionOutcomes(ctx context.Context, experimentID uuid.UUID) ([]service.DecisionOutcome, error) {
	return store.listDecisionOutcomes(ctx, experimentID, nil)
}

func (store *Store) GetSummaryAggregate(ctx context.Context, experimentID uuid.UUID) (service.SummaryAggregate, error) {
	if err := store.ensureOpen(); err != nil {
		return service.SummaryAggregate{}, err
	}
	var aggregate service.SummaryAggregate
	var p50 pgtype.Int8
	var p95 pgtype.Int8
	err := store.pool.QueryRow(ctx, `
        SELECT
            count(d.id),
            count(o.event_id),
            coalesce(sum(o.reward), 0)::double precision,
            count(*) FILTER (WHERE o.kind = 'ignored'),
            count(*) FILTER (WHERE o.kind = 'clicked'),
            count(*) FILTER (WHERE o.kind = 'converted'),
            percentile_disc(0.50) WITHIN GROUP (ORDER BY d.policy_latency_micros),
            percentile_disc(0.95) WITHIN GROUP (ORDER BY d.policy_latency_micros)
        FROM decisions d
        LEFT JOIN outcomes o ON o.decision_id = d.id
        WHERE d.experiment_id = $1
    `, experimentID).Scan(
		&aggregate.DecisionCount,
		&aggregate.OutcomeCount,
		&aggregate.RewardSum,
		&aggregate.IgnoredCount,
		&aggregate.ClickedCount,
		&aggregate.ConvertedCount,
		&p50,
		&p95,
	)
	if err != nil {
		return service.SummaryAggregate{}, fmt.Errorf("get summary aggregate: %w", err)
	}
	if p50.Valid {
		value := p50.Int64
		aggregate.P50PolicyLatencyMicros = &value
	}
	if p95.Valid {
		value := p95.Int64
		aggregate.P95PolicyLatencyMicros = &value
	}
	return aggregate, nil
}

func (store *Store) GetLearningSeries(
	ctx context.Context,
	experimentID uuid.UUID,
	maxPoints int,
) ([]domain.LearningSeriesPoint, error) {
	if err := store.ensureOpen(); err != nil {
		return nil, err
	}
	if maxPoints < 1 || maxPoints > 120 {
		return nil, fmt.Errorf("learning series max points must be between 1 and 120")
	}
	rows, err := store.pool.Query(ctx, `
        SELECT o.received_at, o.reward
        FROM outcomes o
        JOIN decisions d ON d.id = o.decision_id
        WHERE d.experiment_id = $1
        ORDER BY o.received_at, o.event_id
    `, experimentID)
	if err != nil {
		return nil, fmt.Errorf("query learning series: %w", err)
	}
	defer rows.Close()

	var allPoints []domain.LearningSeriesPoint
	rewardSum := 0.0
	for rows.Next() {
		var timestamp time.Time
		var reward float64
		if err := rows.Scan(&timestamp, &reward); err != nil {
			return nil, fmt.Errorf("scan learning series row: %w", err)
		}
		rewardSum += reward
		allPoints = append(allPoints, domain.LearningSeriesPoint{
			Timestamp:               timestamp.UTC(),
			SampleCount:             int64(len(allPoints) + 1),
			CumulativeAverageReward: rewardSum / float64(len(allPoints)+1),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate learning series: %w", err)
	}
	if len(allPoints) <= maxPoints {
		return allPoints, nil
	}

	points := make([]domain.LearningSeriesPoint, 0, maxPoints)
	for index := 0; index < maxPoints; index++ {
		selectedIndex := ((index+1)*len(allPoints)+maxPoints-1)/maxPoints - 1
		points = append(points, allPoints[selectedIndex])
	}
	return points, nil
}

func (store *Store) GetOfferPerformance(ctx context.Context, experimentID uuid.UUID) ([]service.OfferPerformanceRecord, error) {
	if err := store.ensureOpen(); err != nil {
		return nil, err
	}
	rows, err := store.pool.Query(ctx, `
        SELECT
            f.id, f.experiment_id, f.slug, f.merchant_name,
            f.title, f.description, f.category, f.active,
            count(d.id),
            count(o.event_id),
            count(*) FILTER (WHERE o.kind = 'ignored'),
            count(*) FILTER (WHERE o.kind = 'clicked'),
            count(*) FILTER (WHERE o.kind = 'converted'),
            coalesce(sum(o.reward), 0)::double precision
        FROM offers f
        LEFT JOIN decisions d ON d.selected_offer_id = f.id
        LEFT JOIN outcomes o ON o.decision_id = d.id
        WHERE f.experiment_id = $1
        GROUP BY f.id
        ORDER BY f.id
    `, experimentID)
	if err != nil {
		return nil, fmt.Errorf("query offer performance: %w", err)
	}
	defer rows.Close()

	var records []service.OfferPerformanceRecord
	for rows.Next() {
		var record service.OfferPerformanceRecord
		if err := rows.Scan(
			&record.Offer.ID,
			&record.Offer.ExperimentID,
			&record.Offer.Slug,
			&record.Offer.MerchantName,
			&record.Offer.Title,
			&record.Offer.Description,
			&record.Offer.Category,
			&record.Offer.Active,
			&record.SelectionCount,
			&record.OutcomeCount,
			&record.IgnoredCount,
			&record.ClickedCount,
			&record.ConvertedCount,
			&record.RewardSum,
		); err != nil {
			return nil, fmt.Errorf("scan offer performance: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate offer performance: %w", err)
	}
	return records, nil
}

func (store *Store) GetLatestSimulationBenchmark(
	ctx context.Context,
	experimentID uuid.UUID,
) (service.SimulationBenchmarkRecord, bool, error) {
	if err := store.ensureOpen(); err != nil {
		return service.SimulationBenchmarkRecord{}, false, err
	}
	var record service.SimulationBenchmarkRecord
	err := store.pool.QueryRow(ctx, `
        SELECT id, decision_count, outcome_count,
               observed_reward_sum, random_expected_reward_sum,
               oracle_expected_reward_sum
        FROM simulation_runs
        WHERE experiment_id = $1 AND status = 'completed'
        ORDER BY stopped_at DESC, id DESC
        LIMIT 1
    `, experimentID).Scan(
		&record.RunID,
		&record.DecisionCount,
		&record.OutcomeCount,
		&record.ObservedRewardSum,
		&record.RandomExpectedRewardSum,
		&record.OracleExpectedRewardSum,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.SimulationBenchmarkRecord{}, false, nil
	}
	if err != nil {
		return service.SimulationBenchmarkRecord{}, false, fmt.Errorf("get latest simulation benchmark: %w", err)
	}
	return record, true, nil
}

func (store *Store) CreateSimulationRun(ctx context.Context, run domain.SimulationRun) error {
	if err := store.ensureOpen(); err != nil {
		return err
	}
	if err := domain.ValidateSimulationRun(run); err != nil {
		return fmt.Errorf("validate simulation run: %w", err)
	}
	_, err := store.pool.Exec(ctx, `
        INSERT INTO simulation_runs (
            id, experiment_id, seed, requests_per_second, max_decisions,
            status, decision_count, outcome_count, error_count,
            observed_reward_sum, random_expected_reward_sum, oracle_expected_reward_sum,
            started_at, stopped_at, updated_at, error_code, error_detail
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9,
            $10, $11, $12,
            $13, $14, $15, $16, $17
        )
    `,
		run.ID,
		run.ExperimentID,
		run.Seed,
		run.RequestsPerSecond,
		run.MaxDecisions,
		run.Status,
		run.DecisionCount,
		run.OutcomeCount,
		run.ErrorCount,
		run.ObservedRewardSum,
		run.RandomExpectedRewardSum,
		run.OracleExpectedRewardSum,
		run.StartedAt,
		run.StoppedAt,
		run.UpdatedAt,
		run.ErrorCode,
		run.ErrorDetail,
	)
	if isUniqueViolation(err) {
		return service.ErrSimulationConflict
	}
	if err != nil {
		return fmt.Errorf("create simulation run: %w", err)
	}
	return nil
}

func (store *Store) GetSimulationRun(ctx context.Context, runID uuid.UUID) (domain.SimulationRun, error) {
	if err := store.ensureOpen(); err != nil {
		return domain.SimulationRun{}, err
	}
	run, err := scanSimulationRun(store.pool.QueryRow(ctx, simulationRunSelect+` WHERE id = $1`, runID))
	if err != nil {
		return domain.SimulationRun{}, mapRowError("get simulation run", err)
	}
	return run, nil
}

func (store *Store) UpdateSimulationRun(ctx context.Context, run domain.SimulationRun) error {
	if err := store.ensureOpen(); err != nil {
		return err
	}
	if err := domain.ValidateSimulationRun(run); err != nil {
		return fmt.Errorf("validate simulation run update: %w", err)
	}
	commandTag, err := store.pool.Exec(ctx, `
        UPDATE simulation_runs
        SET status = $3,
            decision_count = $4,
            outcome_count = $5,
            error_count = $6,
            observed_reward_sum = $7,
            random_expected_reward_sum = $8,
            oracle_expected_reward_sum = $9,
            stopped_at = $10,
            updated_at = $11,
            error_code = $12,
            error_detail = $13
        WHERE id = $1 AND experiment_id = $2
    `,
		run.ID,
		run.ExperimentID,
		run.Status,
		run.DecisionCount,
		run.OutcomeCount,
		run.ErrorCount,
		run.ObservedRewardSum,
		run.RandomExpectedRewardSum,
		run.OracleExpectedRewardSum,
		run.StoppedAt,
		run.UpdatedAt,
		run.ErrorCode,
		run.ErrorDetail,
	)
	if err != nil {
		return fmt.Errorf("update simulation run: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return service.ErrNotFound
	}
	return nil
}

func (store *Store) ReconcileInterruptedRuns(ctx context.Context, now time.Time) (int64, error) {
	if err := store.ensureOpen(); err != nil {
		return 0, err
	}
	commandTag, err := store.pool.Exec(ctx, `
        UPDATE simulation_runs
        SET status = 'failed',
            stopped_at = $1,
            updated_at = $1,
            error_code = 'process_restarted',
            error_detail = 'API process restarted before the run completed'
        WHERE status IN ('starting', 'running', 'stopping')
    `, now)
	if err != nil {
		return 0, fmt.Errorf("reconcile interrupted simulation runs: %w", err)
	}
	return commandTag.RowsAffected(), nil
}

func (store *Store) listDecisionOutcomes(
	ctx context.Context,
	experimentID uuid.UUID,
	afterVersion *int64,
) ([]service.DecisionOutcome, error) {
	if err := store.ensureOpen(); err != nil {
		return nil, err
	}
	query := decisionOutcomeSelect + ` WHERE d.experiment_id = $1 ORDER BY o.applied_policy_version`
	arguments := []any{experimentID}
	if afterVersion != nil {
		query = decisionOutcomeSelect + `
            WHERE d.experiment_id = $1 AND o.applied_policy_version > $2
            ORDER BY o.applied_policy_version
        `
		arguments = append(arguments, *afterVersion)
	}
	rows, err := store.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list decision outcomes: %w", err)
	}
	defer rows.Close()

	var records []service.DecisionOutcome
	for rows.Next() {
		record, err := scanDecisionOutcome(rows)
		if err != nil {
			return nil, fmt.Errorf("scan decision outcome: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decision outcomes: %w", err)
	}
	return records, nil
}

const decisionSelect = `
    SELECT
        d.id, d.experiment_id, d.selected_offer_id,
        d.context::text, d.segment_key, d.eligible_offer_ids,
        d.distribution::text, d.propensity, d.policy_kind,
        d.policy_version, d.policy_latency_micros,
        d.simulation_run_id, d.request_id, d.created_at
    FROM decisions d
`

const decisionOutcomeSelect = `
    SELECT
        d.id, d.experiment_id, d.selected_offer_id,
        d.context::text, d.segment_key, d.eligible_offer_ids,
        d.distribution::text, d.propensity, d.policy_kind,
        d.policy_version, d.policy_latency_micros,
        d.simulation_run_id, d.request_id, d.created_at,
        o.event_id, o.decision_id, o.kind, o.reward,
        o.occurred_at, o.received_at, o.applied_policy_version
    FROM decisions d
    JOIN outcomes o ON o.decision_id = d.id
`

const simulationRunSelect = `
    SELECT
        id, experiment_id, seed, requests_per_second, max_decisions,
        status, decision_count, outcome_count, error_count,
        observed_reward_sum, random_expected_reward_sum, oracle_expected_reward_sum,
        started_at, stopped_at, updated_at, error_code, error_detail
    FROM simulation_runs
`

func scanExperiment(row rowScanner) (domain.Experiment, error) {
	var experiment domain.Experiment
	var epsilon pgtype.Float8
	if err := row.Scan(
		&experiment.ID,
		&experiment.Slug,
		&experiment.Name,
		&experiment.Status,
		&experiment.PolicyKind,
		&epsilon,
		&experiment.PolicyVersion,
		&experiment.CreatedAt,
		&experiment.UpdatedAt,
	); err != nil {
		return domain.Experiment{}, err
	}
	if epsilon.Valid {
		value := epsilon.Float64
		experiment.Epsilon = &value
	}
	experiment.CreatedAt = experiment.CreatedAt.UTC()
	experiment.UpdatedAt = experiment.UpdatedAt.UTC()
	if err := domain.ValidateExperiment(experiment); err != nil {
		return domain.Experiment{}, fmt.Errorf("validate persisted experiment: %w", err)
	}
	return experiment, nil
}

func scanOffer(row rowScanner) (domain.Offer, error) {
	var offer domain.Offer
	if err := row.Scan(
		&offer.ID,
		&offer.ExperimentID,
		&offer.Slug,
		&offer.MerchantName,
		&offer.Title,
		&offer.Description,
		&offer.Category,
		&offer.Active,
	); err != nil {
		return domain.Offer{}, err
	}
	if err := domain.ValidateOffer(offer); err != nil {
		return domain.Offer{}, fmt.Errorf("validate persisted offer: %w", err)
	}
	return offer, nil
}

func scanDecision(row rowScanner) (domain.Decision, error) {
	var decision domain.Decision
	var contextJSON string
	var distributionJSON string
	var simulationRunID pgtype.UUID
	if err := row.Scan(
		&decision.ID,
		&decision.ExperimentID,
		&decision.SelectedOfferID,
		&contextJSON,
		&decision.SegmentKey,
		&decision.EligibleOfferIDs,
		&distributionJSON,
		&decision.Propensity,
		&decision.PolicyKind,
		&decision.PolicyVersion,
		&decision.PolicyLatencyMicros,
		&simulationRunID,
		&decision.RequestID,
		&decision.CreatedAt,
	); err != nil {
		return domain.Decision{}, err
	}
	if err := decodeStrictJSON([]byte(contextJSON), &decision.Context); err != nil {
		return domain.Decision{}, fmt.Errorf("decode decision context: %w", err)
	}
	if err := decodeStrictJSON([]byte(distributionJSON), &decision.Distribution); err != nil {
		return domain.Decision{}, fmt.Errorf("decode decision distribution: %w", err)
	}
	if simulationRunID.Valid {
		value := uuid.UUID(simulationRunID.Bytes)
		decision.SimulationRunID = &value
	}
	decision.CreatedAt = decision.CreatedAt.UTC()
	if err := domain.ValidateDecision(decision); err != nil {
		return domain.Decision{}, fmt.Errorf("validate persisted decision: %w", err)
	}
	return decision, nil
}

func scanOutcome(row rowScanner) (domain.Outcome, error) {
	var outcome domain.Outcome
	if err := row.Scan(
		&outcome.EventID,
		&outcome.DecisionID,
		&outcome.Kind,
		&outcome.Reward,
		&outcome.OccurredAt,
		&outcome.ReceivedAt,
		&outcome.AppliedPolicyVersion,
	); err != nil {
		return domain.Outcome{}, err
	}
	outcome.OccurredAt = outcome.OccurredAt.UTC()
	outcome.ReceivedAt = outcome.ReceivedAt.UTC()
	return outcome, nil
}

func scanPolicySnapshot(row rowScanner) (domain.PolicySnapshot, error) {
	var snapshot domain.PolicySnapshot
	var state string
	if err := row.Scan(
		&snapshot.ExperimentID,
		&snapshot.PolicyKind,
		&snapshot.PolicyVersion,
		&snapshot.SchemaVersion,
		&state,
		&snapshot.CreatedAt,
	); err != nil {
		return domain.PolicySnapshot{}, err
	}
	snapshot.State = json.RawMessage(state)
	snapshot.CreatedAt = snapshot.CreatedAt.UTC()
	if err := domain.ValidatePolicySnapshot(snapshot); err != nil {
		return domain.PolicySnapshot{}, fmt.Errorf("validate persisted policy snapshot: %w", err)
	}
	return snapshot, nil
}

func scanDecisionOutcome(row rowScanner) (service.DecisionOutcome, error) {
	var record service.DecisionOutcome
	var contextJSON string
	var distributionJSON string
	var simulationRunID pgtype.UUID
	if err := row.Scan(
		&record.Decision.ID,
		&record.Decision.ExperimentID,
		&record.Decision.SelectedOfferID,
		&contextJSON,
		&record.Decision.SegmentKey,
		&record.Decision.EligibleOfferIDs,
		&distributionJSON,
		&record.Decision.Propensity,
		&record.Decision.PolicyKind,
		&record.Decision.PolicyVersion,
		&record.Decision.PolicyLatencyMicros,
		&simulationRunID,
		&record.Decision.RequestID,
		&record.Decision.CreatedAt,
		&record.Outcome.EventID,
		&record.Outcome.DecisionID,
		&record.Outcome.Kind,
		&record.Outcome.Reward,
		&record.Outcome.OccurredAt,
		&record.Outcome.ReceivedAt,
		&record.Outcome.AppliedPolicyVersion,
	); err != nil {
		return service.DecisionOutcome{}, err
	}
	if err := decodeStrictJSON([]byte(contextJSON), &record.Decision.Context); err != nil {
		return service.DecisionOutcome{}, fmt.Errorf("decode decision context: %w", err)
	}
	if err := decodeStrictJSON([]byte(distributionJSON), &record.Decision.Distribution); err != nil {
		return service.DecisionOutcome{}, fmt.Errorf("decode decision distribution: %w", err)
	}
	if simulationRunID.Valid {
		value := uuid.UUID(simulationRunID.Bytes)
		record.Decision.SimulationRunID = &value
	}
	record.Decision.CreatedAt = record.Decision.CreatedAt.UTC()
	record.Outcome.OccurredAt = record.Outcome.OccurredAt.UTC()
	record.Outcome.ReceivedAt = record.Outcome.ReceivedAt.UTC()
	if err := domain.ValidateDecision(record.Decision); err != nil {
		return service.DecisionOutcome{}, fmt.Errorf("validate persisted decision: %w", err)
	}
	return record, nil
}

func scanSimulationRun(row rowScanner) (domain.SimulationRun, error) {
	var run domain.SimulationRun
	var stoppedAt pgtype.Timestamptz
	var errorCode pgtype.Text
	var errorDetail pgtype.Text
	if err := row.Scan(
		&run.ID,
		&run.ExperimentID,
		&run.Seed,
		&run.RequestsPerSecond,
		&run.MaxDecisions,
		&run.Status,
		&run.DecisionCount,
		&run.OutcomeCount,
		&run.ErrorCount,
		&run.ObservedRewardSum,
		&run.RandomExpectedRewardSum,
		&run.OracleExpectedRewardSum,
		&run.StartedAt,
		&stoppedAt,
		&run.UpdatedAt,
		&errorCode,
		&errorDetail,
	); err != nil {
		return domain.SimulationRun{}, err
	}
	run.StartedAt = run.StartedAt.UTC()
	run.UpdatedAt = run.UpdatedAt.UTC()
	if stoppedAt.Valid {
		value := stoppedAt.Time.UTC()
		run.StoppedAt = &value
	}
	if errorCode.Valid {
		value := errorCode.String
		run.ErrorCode = &value
	}
	if errorDetail.Valid {
		value := errorDetail.String
		run.ErrorDetail = &value
	}
	if err := domain.ValidateSimulationRun(run); err != nil {
		return domain.SimulationRun{}, fmt.Errorf("validate persisted simulation run: %w", err)
	}
	return run, nil
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON contains trailing data")
	}
	return nil
}

func exactOutcomeRetry(existing, candidate domain.Outcome) bool {
	return existing.EventID == candidate.EventID &&
		existing.DecisionID == candidate.DecisionID &&
		existing.Kind == candidate.Kind &&
		existing.Reward == candidate.Reward &&
		existing.OccurredAt.Equal(candidate.OccurredAt)
}

func mapRowError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

var _ service.Store = (*Store)(nil)
