package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/onatozmenn/offerpilot/internal/config"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/service"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testDatabaseURL string

func TestMain(testMain *testing.M) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:18-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_DB":       "offerpilot_test",
				"POSTGRES_PASSWORD": "offerpilot_test",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "start PostgreSQL testcontainer: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "resolve PostgreSQL host: %v\n", err)
		_ = testcontainers.TerminateContainer(container)
		os.Exit(1)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "resolve PostgreSQL port: %v\n", err)
		_ = testcontainers.TerminateContainer(container)
		os.Exit(1)
	}
	testDatabaseURL = fmt.Sprintf(
		"postgres://postgres:offerpilot_test@%s:%s/offerpilot_test?sslmode=disable",
		host,
		port.Port(),
	)

	exitCode := testMain.Run()
	if err := testcontainers.TerminateContainer(container); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "terminate PostgreSQL testcontainer: %v\n", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func TestStore_MigrationsAndLifecycle(t *testing.T) {
	store := newTestStore(t)

	version, err := store.provider.GetDBVersion(context.Background())
	if err != nil || version != 1 {
		t.Fatalf("migration version = %d, error = %v", version, err)
	}
	if _, err := store.provider.Down(context.Background()); err != nil {
		t.Fatalf("migration down: %v", err)
	}
	var tableCount int
	if err := store.sqlDB.QueryRowContext(context.Background(), `
        SELECT count(*)
        FROM pg_tables
        WHERE schemaname = 'public' AND tablename <> 'goose_db_version'
    `).Scan(&tableCount); err != nil {
		t.Fatalf("count tables after down: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("tables after down = %d, want 0", tableCount)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migration up again: %v", err)
	}

	t.Chdir(t.TempDir())
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migration from non-root cwd: %v", err)
	}
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	rootErr := errors.New("root transaction error")
	err = store.withTx(context.Background(), func(transaction pgx.Tx) error {
		_, execErr := transaction.Exec(context.Background(), `
            INSERT INTO experiments (
                id, slug, name, status, policy_kind, epsilon,
                policy_version, created_at, updated_at
            ) VALUES (
                '00000000-0000-0000-0000-000000000001',
                'rollback-check', 'Rollback Check', 'running', 'random', NULL,
                1, now(), now()
            )
        `)
		if execErr != nil {
			return execErr
		}
		return rootErr
	})
	if !errors.Is(err, rootErr) {
		t.Fatalf("withTx error = %v, want root error", err)
	}

	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.withTx(cancelledContext, func(pgx.Tx) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled transaction error = %v, want context canceled", err)
	}
}

func TestStore_ExperimentAtomicityAndPagination(t *testing.T) {
	store := newTestStore(t)
	base := testTime()

	first, firstOffers, firstSnapshot := testExperiment(1, base)
	if err := store.CreateExperiment(context.Background(), first, firstOffers, firstSnapshot); err != nil {
		t.Fatalf("CreateExperiment(first): %v", err)
	}
	loaded, err := store.GetExperiment(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("GetExperiment: %v", err)
	}
	if loaded.ID != first.ID || loaded.PolicyKind != domain.PolicyKindRandom || loaded.Epsilon != nil {
		t.Fatalf("loaded experiment = %#v", loaded)
	}
	activeOffers, err := store.ListActiveOffers(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("ListActiveOffers: %v", err)
	}
	if len(activeOffers) != 2 || activeOffers[0].ID.String() > activeOffers[1].ID.String() {
		t.Fatalf("active offers = %#v", activeOffers)
	}
	loadedSnapshot, err := store.GetLatestPolicySnapshot(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("GetLatestPolicySnapshot: %v", err)
	}
	if loadedSnapshot.PolicyKind != first.PolicyKind || loadedSnapshot.PolicyVersion != 1 || string(loadedSnapshot.State) != "{}" {
		t.Fatalf("loaded snapshot = %#v", loadedSnapshot)
	}

	duplicateSlug, duplicateOffers, duplicateSnapshot := testExperiment(2, base.Add(time.Second))
	duplicateSlug.Slug = first.Slug
	if err := store.CreateExperiment(context.Background(), duplicateSlug, duplicateOffers, duplicateSnapshot); err == nil {
		t.Fatal("CreateExperiment(duplicate slug) error = nil")
	}
	if _, err := store.GetExperiment(context.Background(), duplicateSlug.ID); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("GetExperiment(partial insert) error = %v, want not found", err)
	}

	for index := 2; index <= 4; index++ {
		experiment, offers, snapshot := testExperiment(index+1, base.Add(time.Duration(index)*time.Second))
		if err := store.CreateExperiment(context.Background(), experiment, offers, snapshot); err != nil {
			t.Fatalf("CreateExperiment(%d): %v", index, err)
		}
	}
	page, err := store.ListExperiments(context.Background(), nil, 2)
	if err != nil {
		t.Fatalf("ListExperiments(first page): %v", err)
	}
	if len(page) != 2 || page[0].CreatedAt.Before(page[1].CreatedAt) {
		t.Fatalf("first experiment page = %#v", page)
	}
	cursor := service.ExperimentCursor{CreatedAt: page[1].CreatedAt, ID: page[1].ID}
	secondPage, err := store.ListExperiments(context.Background(), &cursor, 2)
	if err != nil {
		t.Fatalf("ListExperiments(second page): %v", err)
	}
	if len(secondPage) != 2 || secondPage[0].ID == page[0].ID || secondPage[0].ID == page[1].ID {
		t.Fatalf("second experiment page = %#v", secondPage)
	}
	activeExperiments, err := store.ListActiveExperiments(context.Background())
	if err != nil {
		t.Fatalf("ListActiveExperiments: %v", err)
	}
	if len(activeExperiments) != 4 {
		t.Fatalf("active experiments = %d, want 4", len(activeExperiments))
	}
}

func TestStore_DecisionMappingAndFeed(t *testing.T) {
	store := newTestStore(t)
	experiment, offers, snapshot := testExperiment(10, testTime())
	mustCreateExperiment(t, store, experiment, offers, snapshot)

	first := testDecision(1, experiment, offers, testTime().Add(time.Minute), 11)
	second := testDecision(2, experiment, offers, testTime().Add(2*time.Minute), 22)
	third := testDecision(3, experiment, offers, testTime().Add(3*time.Minute), 33)
	for _, decision := range []domain.Decision{first, second, third} {
		if err := store.InsertDecision(context.Background(), decision); err != nil {
			t.Fatalf("InsertDecision(%s): %v", decision.ID, err)
		}
	}

	loaded, err := store.GetDecision(context.Background(), second.ID)
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}
	if loaded.ID != second.ID || loaded.Context != second.Context || loaded.PolicyLatencyMicros != 22 || len(loaded.Distribution) != 2 {
		t.Fatalf("loaded decision = %#v", loaded)
	}
	if loaded.EligibleOfferIDs[0] != offers[0].ID || loaded.EligibleOfferIDs[1] != offers[1].ID {
		t.Fatalf("native UUID array mapping = %#v", loaded.EligibleOfferIDs)
	}

	page, err := store.ListDecisions(context.Background(), experiment.ID, nil, 2)
	if err != nil {
		t.Fatalf("ListDecisions(first page): %v", err)
	}
	if len(page) != 2 || page[0].ID != third.ID || page[1].ID != second.ID {
		t.Fatalf("first decision page = %#v", page)
	}
	cursor := service.DecisionCursor{CreatedAt: page[1].CreatedAt, ID: page[1].ID}
	remaining, err := store.ListDecisions(context.Background(), experiment.ID, &cursor, 2)
	if err != nil {
		t.Fatalf("ListDecisions(second page): %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != first.ID {
		t.Fatalf("second decision page = %#v", remaining)
	}

	feed, err := store.ListDecisionFeed(context.Background(), experiment.ID, nil, 3)
	if err != nil {
		t.Fatalf("ListDecisionFeed(before outcome): %v", err)
	}
	if len(feed) != 3 || feed[0].Decision.ID != third.ID || feed[0].SelectedOffer.ID != third.SelectedOfferID || feed[0].Outcome != nil {
		t.Fatalf("decision feed before outcome = %#v", feed)
	}
	outcome := testOutcome(20, second.ID, second.CreatedAt.Add(time.Second), domain.OutcomeKindClicked)
	acceptance, err := store.AcceptOutcome(context.Background(), experiment.ID, outcome)
	if err != nil || acceptance.Status != service.OutcomeAcceptanceCreated {
		t.Fatalf("AcceptOutcome(feed) = %#v, error = %v", acceptance, err)
	}
	feed, err = store.ListDecisionFeed(context.Background(), experiment.ID, nil, 3)
	if err != nil {
		t.Fatalf("ListDecisionFeed(after outcome): %v", err)
	}
	if feed[1].Decision.ID != second.ID || feed[1].Outcome == nil || feed[1].Outcome.Kind != domain.OutcomeKindClicked || feed[1].Outcome.AppliedPolicyVersion != 2 {
		t.Fatalf("decision feed after outcome = %#v", feed)
	}
}

func TestStore_OutcomeIdempotencyAndConcurrentVersions(t *testing.T) {
	store := newTestStore(t)
	experiment, offers, snapshot := testExperiment(20, testTime())
	mustCreateExperiment(t, store, experiment, offers, snapshot)

	firstDecision := testDecision(1, experiment, offers, testTime().Add(time.Minute), 10)
	if err := store.InsertDecision(context.Background(), firstDecision); err != nil {
		t.Fatalf("InsertDecision: %v", err)
	}
	firstOutcome := testOutcome(1, firstDecision.ID, testTime().Add(2*time.Minute), domain.OutcomeKindClicked)
	created, err := store.AcceptOutcome(context.Background(), experiment.ID, firstOutcome)
	if err != nil {
		t.Fatalf("AcceptOutcome(created): %v", err)
	}
	if created.Status != service.OutcomeAcceptanceCreated || created.Outcome.AppliedPolicyVersion != 2 {
		t.Fatalf("created acceptance = %#v", created)
	}
	retry, err := store.AcceptOutcome(context.Background(), experiment.ID, firstOutcome)
	if err != nil {
		t.Fatalf("AcceptOutcome(retry): %v", err)
	}
	if retry.Status != service.OutcomeAcceptanceExactRetry || retry.Outcome.AppliedPolicyVersion != 2 {
		t.Fatalf("retry acceptance = %#v", retry)
	}
	competing := firstOutcome
	competing.EventID = deterministicUUID(999)
	conflict, err := store.AcceptOutcome(context.Background(), experiment.ID, competing)
	if err != nil {
		t.Fatalf("AcceptOutcome(conflict): %v", err)
	}
	if conflict.Status != service.OutcomeAcceptanceConflict || conflict.Outcome.EventID != firstOutcome.EventID {
		t.Fatalf("conflict acceptance = %#v", conflict)
	}

	const concurrentOutcomes = 12
	decisions := make([]domain.Decision, concurrentOutcomes)
	for index := range decisions {
		decisions[index] = testDecision(index+2, experiment, offers, testTime().Add(time.Duration(index+3)*time.Minute), int64(index+20))
		if err := store.InsertDecision(context.Background(), decisions[index]); err != nil {
			t.Fatalf("InsertDecision(%d): %v", index, err)
		}
	}
	versions := make(chan int64, concurrentOutcomes)
	errorsChannel := make(chan error, concurrentOutcomes)
	var waitGroup sync.WaitGroup
	for index, decision := range decisions {
		waitGroup.Add(1)
		go func(index int, decision domain.Decision) {
			defer waitGroup.Done()
			outcome := testOutcome(index+2, decision.ID, decision.CreatedAt.Add(time.Second), domain.OutcomeKindConverted)
			acceptance, err := store.AcceptOutcome(context.Background(), experiment.ID, outcome)
			if err != nil {
				errorsChannel <- err
				return
			}
			if acceptance.Status != service.OutcomeAcceptanceCreated {
				errorsChannel <- fmt.Errorf("status = %s", acceptance.Status)
				return
			}
			versions <- acceptance.Outcome.AppliedPolicyVersion
		}(index, decision)
	}
	waitGroup.Wait()
	close(errorsChannel)
	close(versions)
	for err := range errorsChannel {
		t.Errorf("concurrent AcceptOutcome: %v", err)
	}
	var appliedVersions []int
	for version := range versions {
		appliedVersions = append(appliedVersions, int(version))
	}
	sort.Ints(appliedVersions)
	if len(appliedVersions) != concurrentOutcomes {
		t.Fatalf("applied versions = %v", appliedVersions)
	}
	for index, version := range appliedVersions {
		if want := index + 3; version != want {
			t.Fatalf("appliedVersions[%d] = %d, want %d", index, version, want)
		}
	}

	loadedExperiment, err := store.GetExperiment(context.Background(), experiment.ID)
	if err != nil {
		t.Fatalf("GetExperiment: %v", err)
	}
	if loadedExperiment.PolicyVersion != concurrentOutcomes+2 {
		t.Fatalf("policy version = %d, want %d", loadedExperiment.PolicyVersion, concurrentOutcomes+2)
	}
	recoveryRecords, err := store.ListDecisionOutcomesAfterVersion(context.Background(), experiment.ID, 1)
	if err != nil {
		t.Fatalf("ListDecisionOutcomesAfterVersion: %v", err)
	}
	if len(recoveryRecords) != concurrentOutcomes+1 {
		t.Fatalf("recovery records = %d, want %d", len(recoveryRecords), concurrentOutcomes+1)
	}
	for index, record := range recoveryRecords {
		if record.Outcome.AppliedPolicyVersion != int64(index+2) {
			t.Fatalf("recovery version[%d] = %d", index, record.Outcome.AppliedPolicyVersion)
		}
	}
}

func TestStore_SnapshotIdempotencyAndConflict(t *testing.T) {
	store := newTestStore(t)
	experiment, offers, initialSnapshot := testExperiment(30, testTime())
	mustCreateExperiment(t, store, experiment, offers, initialSnapshot)

	if err := store.SavePolicySnapshot(context.Background(), initialSnapshot); err != nil {
		t.Fatalf("SavePolicySnapshot(exact retry): %v", err)
	}
	conflicting := initialSnapshot
	conflicting.State = []byte(`{"changed":true}`)
	if err := store.SavePolicySnapshot(context.Background(), conflicting); !errors.Is(err, service.ErrSnapshotConflict) {
		t.Fatalf("SavePolicySnapshot(conflict) error = %v", err)
	}

	second := initialSnapshot
	second.PolicyVersion = 2
	second.State = []byte(`{"version":2}`)
	second.CreatedAt = second.CreatedAt.Add(time.Minute)
	if err := store.SavePolicySnapshot(context.Background(), second); err != nil {
		t.Fatalf("SavePolicySnapshot(version 2): %v", err)
	}
	latest, err := store.GetLatestPolicySnapshot(context.Background(), experiment.ID)
	if err != nil {
		t.Fatalf("GetLatestPolicySnapshot: %v", err)
	}
	if latest.PolicyVersion != 2 || string(latest.State) != `{"version": 2}` {
		t.Fatalf("latest snapshot = %#v", latest)
	}
}

func TestStore_AggregatesLearningSeriesAndBenchmark(t *testing.T) {
	store := newTestStore(t)
	experiment, offers, snapshot := testExperiment(40, testTime())
	mustCreateExperiment(t, store, experiment, offers, snapshot)

	kinds := []domain.OutcomeKind{domain.OutcomeKindIgnored, domain.OutcomeKindClicked, domain.OutcomeKindConverted}
	latencies := []int64{10, 20, 30}
	for index, kind := range kinds {
		decision := testDecision(index+1, experiment, offers, testTime().Add(time.Duration(index+1)*time.Minute), latencies[index])
		if index == 2 {
			decision.SelectedOfferID = offers[1].ID
			decision.Propensity = 0.5
		}
		if err := store.InsertDecision(context.Background(), decision); err != nil {
			t.Fatalf("InsertDecision(%d): %v", index, err)
		}
		outcome := testOutcome(index+1, decision.ID, decision.CreatedAt.Add(time.Second), kind)
		if _, err := store.AcceptOutcome(context.Background(), experiment.ID, outcome); err != nil {
			t.Fatalf("AcceptOutcome(%d): %v", index, err)
		}
	}

	aggregate, err := store.GetSummaryAggregate(context.Background(), experiment.ID)
	if err != nil {
		t.Fatalf("GetSummaryAggregate: %v", err)
	}
	if aggregate.DecisionCount != 3 || aggregate.OutcomeCount != 3 || aggregate.RewardSum != 1.25 || aggregate.IgnoredCount != 1 || aggregate.ClickedCount != 1 || aggregate.ConvertedCount != 1 {
		t.Fatalf("summary aggregate = %#v", aggregate)
	}
	if aggregate.P50PolicyLatencyMicros == nil || *aggregate.P50PolicyLatencyMicros != 20 || aggregate.P95PolicyLatencyMicros == nil || *aggregate.P95PolicyLatencyMicros != 30 {
		t.Fatalf("latency percentiles = %#v", aggregate)
	}
	series, err := store.GetLearningSeries(context.Background(), experiment.ID, 2)
	if err != nil {
		t.Fatalf("GetLearningSeries: %v", err)
	}
	if len(series) != 2 || series[1].SampleCount != 3 || series[1].CumulativeAverageReward != 1.25/3 {
		t.Fatalf("learning series = %#v", series)
	}
	performance, err := store.GetOfferPerformance(context.Background(), experiment.ID)
	if err != nil {
		t.Fatalf("GetOfferPerformance: %v", err)
	}
	if len(performance) != 2 || performance[0].SelectionCount+performance[1].SelectionCount != 3 {
		t.Fatalf("offer performance = %#v", performance)
	}
	records, err := store.ListDecisionOutcomes(context.Background(), experiment.ID)
	if err != nil || len(records) != 3 {
		t.Fatalf("ListDecisionOutcomes length = %d, error = %v", len(records), err)
	}

	completedRun := testSimulationRun(1, experiment.ID, domain.SimulationRunStatusCompleted, testTime().Add(10*time.Minute))
	completedRun.DecisionCount = 3
	completedRun.OutcomeCount = 3
	completedRun.ObservedRewardSum = 1.25
	completedRun.RandomExpectedRewardSum = 0.9
	completedRun.OracleExpectedRewardSum = 1.8
	if err := store.CreateSimulationRun(context.Background(), completedRun); err != nil {
		t.Fatalf("CreateSimulationRun(completed): %v", err)
	}
	benchmark, found, err := store.GetLatestSimulationBenchmark(context.Background(), experiment.ID)
	if err != nil || !found {
		t.Fatalf("GetLatestSimulationBenchmark found = %v, error = %v", found, err)
	}
	if benchmark.RunID != completedRun.ID || benchmark.RandomExpectedRewardSum != 0.9 || benchmark.OracleExpectedRewardSum != 1.8 {
		t.Fatalf("benchmark = %#v", benchmark)
	}
}

func TestStore_SimulationUniquenessUpdateAndReconciliation(t *testing.T) {
	store := newTestStore(t)
	experiment, offers, snapshot := testExperiment(50, testTime())
	mustCreateExperiment(t, store, experiment, offers, snapshot)

	first := testSimulationRun(1, experiment.ID, domain.SimulationRunStatusRunning, testTime().Add(time.Minute))
	if err := store.CreateSimulationRun(context.Background(), first); err != nil {
		t.Fatalf("CreateSimulationRun(first): %v", err)
	}
	second := testSimulationRun(2, experiment.ID, domain.SimulationRunStatusStarting, testTime().Add(2*time.Minute))
	if err := store.CreateSimulationRun(context.Background(), second); !errors.Is(err, service.ErrSimulationConflict) {
		t.Fatalf("CreateSimulationRun(second active) error = %v", err)
	}

	stopped := first.StartedAt.Add(time.Minute)
	first.Status = domain.SimulationRunStatusCompleted
	first.StoppedAt = &stopped
	first.UpdatedAt = stopped
	first.DecisionCount = 10
	first.OutcomeCount = 9
	first.ErrorCount = 1
	first.ObservedRewardSum = 3
	first.RandomExpectedRewardSum = 2.5
	first.OracleExpectedRewardSum = 4
	if err := store.UpdateSimulationRun(context.Background(), first); err != nil {
		t.Fatalf("UpdateSimulationRun: %v", err)
	}
	loaded, err := store.GetSimulationRun(context.Background(), first.ID)
	if err != nil || loaded.Status != domain.SimulationRunStatusCompleted || loaded.DecisionCount != 10 {
		t.Fatalf("GetSimulationRun = %#v, error = %v", loaded, err)
	}

	if err := store.CreateSimulationRun(context.Background(), second); err != nil {
		t.Fatalf("CreateSimulationRun(after release): %v", err)
	}
	reconcileAt := second.StartedAt.Add(2 * time.Minute)
	count, err := store.ReconcileInterruptedRuns(context.Background(), reconcileAt)
	if err != nil || count != 1 {
		t.Fatalf("ReconcileInterruptedRuns count = %d, error = %v", count, err)
	}
	reconciled, err := store.GetSimulationRun(context.Background(), second.ID)
	if err != nil {
		t.Fatalf("GetSimulationRun(reconciled): %v", err)
	}
	if reconciled.Status != domain.SimulationRunStatusFailed || reconciled.ErrorCode == nil || *reconciled.ErrorCode != "process_restarted" || reconciled.DecisionCount != 0 {
		t.Fatalf("reconciled run = %#v", reconciled)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), config.DatabaseConfig{
		URL:               testDatabaseURL,
		MaxConns:          16,
		MinConns:          1,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := store.pool.Exec(context.Background(), `
        TRUNCATE TABLE
            outcomes,
            decisions,
            policy_snapshots,
            simulation_runs,
            offers,
            experiments
    `); err != nil {
		t.Fatalf("reset database: %v", err)
	}
	return store
}

func mustCreateExperiment(
	t *testing.T,
	store *Store,
	experiment domain.Experiment,
	offers []domain.Offer,
	snapshot domain.PolicySnapshot,
) {
	t.Helper()
	if err := store.CreateExperiment(context.Background(), experiment, offers, snapshot); err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
}

func testExperiment(index int, createdAt time.Time) (domain.Experiment, []domain.Offer, domain.PolicySnapshot) {
	experimentID := deterministicUUID(10_000 + index)
	experiment := domain.Experiment{
		ID:            experimentID,
		Slug:          fmt.Sprintf("experiment-%03d", index),
		Name:          fmt.Sprintf("Experiment %03d", index),
		Status:        domain.ExperimentStatusRunning,
		PolicyKind:    domain.PolicyKindRandom,
		PolicyVersion: 1,
		CreatedAt:     createdAt.UTC(),
		UpdatedAt:     createdAt.UTC(),
	}
	offers := []domain.Offer{
		{
			ID:           deterministicUUID(20_000 + index*10),
			ExperimentID: experimentID,
			Slug:         fmt.Sprintf("offer-%03d-a", index),
			MerchantName: fmt.Sprintf("Fictional Merchant %03d A", index),
			Title:        "Synthetic offer A",
			Description:  "Synthetic test fixture.",
			Category:     domain.OfferCategoryTravel,
			Active:       true,
		},
		{
			ID:           deterministicUUID(20_000 + index*10 + 1),
			ExperimentID: experimentID,
			Slug:         fmt.Sprintf("offer-%03d-b", index),
			MerchantName: fmt.Sprintf("Fictional Merchant %03d B", index),
			Title:        "Synthetic offer B",
			Description:  "Synthetic test fixture.",
			Category:     domain.OfferCategoryHome,
			Active:       true,
		},
	}
	sort.Slice(offers, func(left, right int) bool {
		return offers[left].ID.String() < offers[right].ID.String()
	})
	snapshot := domain.PolicySnapshot{
		ExperimentID:  experimentID,
		PolicyKind:    domain.PolicyKindRandom,
		PolicyVersion: 1,
		SchemaVersion: domain.PolicySnapshotSchemaVersion,
		State:         []byte(`{}`),
		CreatedAt:     createdAt.UTC(),
	}
	return experiment, offers, snapshot
}

func testDecision(
	index int,
	experiment domain.Experiment,
	offers []domain.Offer,
	createdAt time.Time,
	latencyMicros int64,
) domain.Decision {
	return domain.Decision{
		ID:              deterministicUUID(30_000 + index),
		ExperimentID:    experiment.ID,
		SelectedOfferID: offers[0].ID,
		Context: domain.SessionContext{
			DeviceClass:      domain.DeviceClassMobile,
			Daypart:          domain.DaypartEvening,
			CategoryAffinity: domain.OfferCategoryTravel,
			VisitorType:      domain.VisitorTypeReturning,
		},
		SegmentKey:       "mobile|evening|travel|returning",
		EligibleOfferIDs: []uuid.UUID{offers[0].ID, offers[1].ID},
		Distribution: []domain.ActionProbability{
			{OfferID: offers[0].ID, Probability: 0.5},
			{OfferID: offers[1].ID, Probability: 0.5},
		},
		Propensity:          0.5,
		PolicyKind:          experiment.PolicyKind,
		PolicyVersion:       1,
		PolicyLatencyMicros: latencyMicros,
		RequestID:           fmt.Sprintf("request-%03d", index),
		CreatedAt:           createdAt.UTC(),
	}
}

func testOutcome(index int, decisionID uuid.UUID, occurredAt time.Time, kind domain.OutcomeKind) domain.Outcome {
	reward, err := domain.RewardForOutcome(kind)
	if err != nil {
		panic(err)
	}
	return domain.Outcome{
		EventID:    deterministicUUID(40_000 + index),
		DecisionID: decisionID,
		Kind:       kind,
		Reward:     reward,
		OccurredAt: occurredAt.UTC(),
		ReceivedAt: occurredAt.Add(time.Second).UTC(),
	}
}

func testSimulationRun(index int, experimentID uuid.UUID, status domain.SimulationRunStatus, startedAt time.Time) domain.SimulationRun {
	run := domain.SimulationRun{
		ID:                deterministicUUID(50_000 + index),
		ExperimentID:      experimentID,
		Seed:              int64(20260717 + index),
		RequestsPerSecond: 20,
		MaxDecisions:      100,
		Status:            status,
		StartedAt:         startedAt.UTC(),
		UpdatedAt:         startedAt.UTC(),
	}
	if status == domain.SimulationRunStatusCompleted || status == domain.SimulationRunStatusFailed || status == domain.SimulationRunStatusCancelled {
		stoppedAt := startedAt.Add(time.Minute).UTC()
		run.StoppedAt = &stoppedAt
		run.UpdatedAt = stoppedAt
	}
	return run
}

func deterministicUUID(value int) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("offerpilot-test-%08d", value)))
}

func testTime() time.Time {
	return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
}
