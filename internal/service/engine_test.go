package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/bandit"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

func TestEngine_CreateDecideAndRecordOutcome(t *testing.T) {
	store := newFakeStore()
	engine := newTestEngine(t, store, actualPolicyFactory())
	experiment, offers := createRunningExperiment(t, engine, domain.PolicyKindRandom)
	if !engine.IsHealthy(experiment.ID) {
		t.Fatal("new experiment policy is not healthy")
	}
	if len(store.snapshots[experiment.ID]) != 1 || len(store.offers[experiment.ID]) != 2 {
		t.Fatalf("atomic create state: snapshots=%d offers=%d", len(store.snapshots[experiment.ID]), len(store.offers[experiment.ID]))
	}

	decision, err := engine.Decide(context.Background(), DecideCommand{
		ExperimentID: experiment.ID,
		Context:      testContext(),
		RequestID:    "request-1",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if _, persisted := store.decisions[decision.ID]; !persisted {
		t.Fatal("Decide returned before persistence")
	}
	if decision.PolicyVersion != 1 || decision.Propensity != 0.5 || len(decision.Distribution) != 2 || decision.PolicyLatencyMicros < 0 {
		t.Fatalf("decision = %#v", decision)
	}
	if decision.EligibleOfferIDs[0] != offers[0].ID || decision.EligibleOfferIDs[1] != offers[1].ID {
		t.Fatalf("eligible offer order = %v", decision.EligibleOfferIDs)
	}

	command := RecordOutcomeCommand{
		EventID:    testUUID(100),
		DecisionID: decision.ID,
		Kind:       domain.OutcomeKindClicked,
		OccurredAt: testNow().Add(time.Minute),
	}
	result, err := engine.RecordOutcome(context.Background(), command)
	if err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}
	if !result.Created || !result.PolicyUpdated || !result.SnapshotSaved || result.Outcome.AppliedPolicyVersion != 2 || result.Outcome.Reward != 0.25 {
		t.Fatalf("RecordOutcome result = %#v", result)
	}
	view, err := engine.PolicyView(experiment.ID)
	if err != nil || view.Version != 2 {
		t.Fatalf("PolicyView version = %d, error = %v", view.Version, err)
	}
	if len(store.snapshots[experiment.ID]) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(store.snapshots[experiment.ID]))
	}

	retry, err := engine.RecordOutcome(context.Background(), command)
	if err != nil || !retry.ExactRetry || retry.PolicyUpdated || retry.SnapshotSaved {
		t.Fatalf("retry result = %#v, error = %v", retry, err)
	}
	competing := command
	competing.EventID = testUUID(101)
	if _, err := engine.RecordOutcome(context.Background(), competing); !errors.Is(err, ErrOutcomeConflict) {
		t.Fatalf("competing outcome error = %v", err)
	}
}

func TestEngine_DecideRejectsInvalidPolicyOutput(t *testing.T) {
	store := newFakeStore()
	factory := PolicyFactoryFunc(func(experiment domain.Experiment) (bandit.Policy, error) {
		return &invalidSelectionPolicy{experimentID: experiment.ID, version: 1}, nil
	})
	engine := newTestEngine(t, store, factory)
	experiment, _ := createRunningExperiment(t, engine, domain.PolicyKindRandom)

	if _, err := engine.Decide(context.Background(), DecideCommand{
		ExperimentID: experiment.ID,
		Context:      testContext(),
		RequestID:    "request-invalid",
	}); err == nil {
		t.Fatal("Decide() error = nil for invalid policy output")
	}
	if len(store.decisions) != 0 {
		t.Fatal("invalid policy output was persisted")
	}
}

func TestEngine_CreateAndDecideFailures(t *testing.T) {
	t.Run("create store failure", func(t *testing.T) {
		store := newFakeStore()
		store.createExperimentErr = errors.New("database unavailable")
		engine := newTestEngine(t, store, actualPolicyFactory())
		experiment := domain.Experiment{Slug: "failed-create", Name: "Failed Create", Status: domain.ExperimentStatusRunning, PolicyKind: domain.PolicyKindRandom}
		offers := []domain.Offer{
			{Slug: "a", MerchantName: "Fictional A", Title: "A", Category: domain.OfferCategoryTravel, Active: true},
			{Slug: "b", MerchantName: "Fictional B", Title: "B", Category: domain.OfferCategoryHome, Active: true},
		}
		if _, err := engine.CreateExperiment(context.Background(), experiment, offers); err == nil {
			t.Fatal("CreateExperiment() error = nil")
		}
		if len(engine.policies) != 0 {
			t.Fatal("failed persistence published an in-memory policy")
		}
	})

	t.Run("cancelled decision", func(t *testing.T) {
		store := newFakeStore()
		engine := newTestEngine(t, store, actualPolicyFactory())
		experiment, _ := createRunningExperiment(t, engine, domain.PolicyKindRandom)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := engine.Decide(ctx, DecideCommand{ExperimentID: experiment.ID, Context: testContext(), RequestID: "cancelled"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("Decide(cancelled) error = %v", err)
		}
		if len(store.decisions) != 0 {
			t.Fatal("cancelled decision was persisted")
		}
	})
}

func TestEngine_ConcurrentOutcomesUseConsecutiveVersions(t *testing.T) {
	store := newFakeStore()
	engine := newTestEngine(t, store, actualPolicyFactory())
	experiment, _ := createRunningExperiment(t, engine, domain.PolicyKindRandom)

	const count = 24
	decisions := make([]domain.Decision, count)
	for index := range decisions {
		decision, err := engine.Decide(context.Background(), DecideCommand{
			ExperimentID: experiment.ID,
			Context:      testContext(),
			RequestID:    fmt.Sprintf("request-%d", index),
		})
		if err != nil {
			t.Fatalf("Decide(%d): %v", index, err)
		}
		decisions[index] = decision
	}

	versions := make(chan int64, count)
	errorsChannel := make(chan error, count)
	var waitGroup sync.WaitGroup
	for index, decision := range decisions {
		waitGroup.Add(1)
		go func(index int, decision domain.Decision) {
			defer waitGroup.Done()
			result, err := engine.RecordOutcome(context.Background(), RecordOutcomeCommand{
				EventID:    testUUID(1_000 + index),
				DecisionID: decision.ID,
				Kind:       domain.OutcomeKindConverted,
				OccurredAt: testNow().Add(time.Duration(index) * time.Second),
			})
			if err != nil {
				errorsChannel <- err
				return
			}
			versions <- result.Outcome.AppliedPolicyVersion
		}(index, decision)
	}
	waitGroup.Wait()
	close(errorsChannel)
	close(versions)
	for err := range errorsChannel {
		t.Errorf("RecordOutcome: %v", err)
	}
	var got []int
	for version := range versions {
		got = append(got, int(version))
	}
	sort.Ints(got)
	if len(got) != count {
		t.Fatalf("versions = %v", got)
	}
	for index, version := range got {
		if version != index+2 {
			t.Fatalf("versions[%d] = %d, want %d", index, version, index+2)
		}
	}
	view, err := engine.PolicyView(experiment.ID)
	if err != nil || view.Version != count+1 {
		t.Fatalf("final policy version = %d, error = %v", view.Version, err)
	}
}

func TestEngine_SnapshotFailureMarksPolicyUnhealthy(t *testing.T) {
	store := newFakeStore()
	engine := newTestEngine(t, store, actualPolicyFactory())
	experiment, _ := createRunningExperiment(t, engine, domain.PolicyKindRandom)
	decision, err := engine.Decide(context.Background(), DecideCommand{
		ExperimentID: experiment.ID,
		Context:      testContext(),
		RequestID:    "request-snapshot-failure",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	store.saveSnapshotErr = errors.New("snapshot storage unavailable")
	result, err := engine.RecordOutcome(context.Background(), RecordOutcomeCommand{
		EventID:    testUUID(2_000),
		DecisionID: decision.ID,
		Kind:       domain.OutcomeKindConverted,
		OccurredAt: testNow(),
	})
	if err == nil || !result.Created || !result.PolicyUpdated || result.SnapshotSaved {
		t.Fatalf("RecordOutcome result = %#v, error = %v", result, err)
	}
	if engine.IsHealthy(experiment.ID) {
		t.Fatal("policy remained healthy after checkpoint failure")
	}
	if _, err := engine.Decide(context.Background(), DecideCommand{ExperimentID: experiment.ID, Context: testContext(), RequestID: "blocked"}); !errors.Is(err, ErrPolicyUnhealthy) {
		t.Fatalf("Decide after checkpoint failure error = %v", err)
	}
}

func TestEngine_ReadinessRequiresEveryActivePolicy(t *testing.T) {
	store := newFakeStore()
	engine := newTestEngine(t, store, actualPolicyFactory())
	experiment, _ := createRunningExperiment(t, engine, domain.PolicyKindRandom)
	if err := engine.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := engine.Ready(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ready(cancelled) error = %v", err)
	}
	engine.markUnhealthy(experiment.ID, errors.New("checkpoint gap"))
	if err := engine.Ready(context.Background()); !errors.Is(err, ErrPolicyUnhealthy) {
		t.Fatalf("Ready(unhealthy) error = %v", err)
	}
}

func TestEngine_SummaryPersistsNullReasonsAndOPE(t *testing.T) {
	store := newFakeStore()
	engine := newTestEngine(t, store, actualPolicyFactory())
	experiment, offers := createRunningExperiment(t, engine, domain.PolicyKindRandom)
	store.summary = SummaryAggregate{
		DecisionCount:  10,
		OutcomeCount:   10,
		RewardSum:      5,
		IgnoredCount:   5,
		ConvertedCount: 5,
	}
	p50 := int64(10)
	p95 := int64(20)
	store.summary.P50PolicyLatencyMicros = &p50
	store.summary.P95PolicyLatencyMicros = &p95
	store.performance = []OfferPerformanceRecord{
		{Offer: offers[0], SelectionCount: 5, OutcomeCount: 5, ConvertedCount: 5, RewardSum: 5},
		{Offer: offers[1], SelectionCount: 5, OutcomeCount: 5, IgnoredCount: 5},
	}
	store.series = []domain.LearningSeriesPoint{
		{Timestamp: testNow(), SampleCount: 5, CumulativeAverageReward: 0.4},
		{Timestamp: testNow().Add(time.Minute), SampleCount: 10, CumulativeAverageReward: 0.5},
	}
	store.benchmark = SimulationBenchmarkRecord{RunID: testUUID(3_000), DecisionCount: 10, OutcomeCount: 10, ObservedRewardSum: 5, RandomExpectedRewardSum: 4, OracleExpectedRewardSum: 7}
	store.benchmarkFound = true
	for index := 0; index < 10; index++ {
		decision := testServiceDecision(testUUID(3_100+index), experiment, offers, testNow().Add(time.Duration(index)*time.Second))
		outcome := domain.Outcome{
			EventID:              testUUID(3_200 + index),
			DecisionID:           decision.ID,
			Kind:                 domain.OutcomeKindClicked,
			Reward:               0.5,
			OccurredAt:           decision.CreatedAt,
			ReceivedAt:           decision.CreatedAt.Add(time.Second),
			AppliedPolicyVersion: int64(index + 2),
		}
		store.decisionOutcomes = append(store.decisionOutcomes, DecisionOutcome{Decision: decision, Outcome: outcome})
	}

	summary, err := engine.Summary(context.Background(), experiment.ID, 2)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.AverageReward == nil || *summary.AverageReward != 0.5 || summary.OPE.IPS == nil || summary.OPE.SNIPS == nil || summary.OPE.Reason != "" {
		t.Fatalf("summary estimates = %#v", summary)
	}
	if summary.RandomBenchmark.ExpectedAverageReward == nil || *summary.RandomBenchmark.ExpectedAverageReward != 0.4 || summary.OracleBenchmark.ExpectedAverageReward == nil || *summary.OracleBenchmark.ExpectedAverageReward != 0.7 {
		t.Fatalf("benchmarks = %#v / %#v", summary.RandomBenchmark, summary.OracleBenchmark)
	}
	if len(summary.OfferPerformance) != 2 || summary.OfferPerformance[0].CurrentProbability == nil || *summary.OfferPerformance[0].CurrentProbability != 0.5 || store.lastMaxPoints != 2 {
		t.Fatalf("offer performance = %#v, max points = %d", summary.OfferPerformance, store.lastMaxPoints)
	}

	store.summary = SummaryAggregate{}
	store.performance = nil
	store.series = nil
	store.decisionOutcomes = nil
	store.benchmarkFound = false
	empty, err := engine.Summary(context.Background(), experiment.ID, 0)
	if err != nil {
		t.Fatalf("Summary(empty): %v", err)
	}
	if empty.AverageReward != nil || empty.RandomBenchmark.ExpectedAverageReward != nil || empty.OPE.IPS != nil || empty.Reasons["average_reward"] != "no_outcomes" || empty.Reasons["random_benchmark"] != "not_simulated" || empty.Reasons["ope"] != "no_samples" {
		t.Fatalf("empty summary = %#v", empty)
	}
}

func TestEngine_RecoverReplaysCrashWindow(t *testing.T) {
	store := newFakeStore()
	experiment, offers, snapshot := storedRecoveryExperiment(3)
	store.experiments[experiment.ID] = experiment
	store.offers[experiment.ID] = offers
	store.snapshots[experiment.ID] = []domain.PolicySnapshot{snapshot}
	for version := int64(2); version <= 3; version++ {
		decision := testServiceDecision(testUUID(int(4_000+version)), experiment, offers, testNow().Add(time.Duration(version)*time.Second))
		decision.PolicyVersion = 1
		outcome := domain.Outcome{
			EventID:              testUUID(int(4_100 + version)),
			DecisionID:           decision.ID,
			Kind:                 domain.OutcomeKindClicked,
			Reward:               0.25,
			OccurredAt:           decision.CreatedAt,
			ReceivedAt:           decision.CreatedAt.Add(time.Second),
			AppliedPolicyVersion: version,
		}
		store.decisionOutcomes = append(store.decisionOutcomes, DecisionOutcome{Decision: decision, Outcome: outcome})
	}
	store.reconciledRuns = 1
	engine := newTestEngine(t, store, actualPolicyFactory())

	result, err := engine.Recover(context.Background())
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if result.ExperimentsRecovered != 1 || result.OutcomesReplayed != 2 || result.SnapshotsSaved != 1 || result.InterruptedRuns != 1 {
		t.Fatalf("RecoveryResult = %#v", result)
	}
	view, err := engine.PolicyView(experiment.ID)
	if err != nil || view.Version != 3 {
		t.Fatalf("recovered policy version = %d, error = %v", view.Version, err)
	}
	latest := store.snapshots[experiment.ID][len(store.snapshots[experiment.ID])-1]
	if latest.PolicyVersion != 3 {
		t.Fatalf("checkpoint version = %d, want 3", latest.PolicyVersion)
	}
}

func TestEngine_RecoverRejectsGapAndCorruptSnapshot(t *testing.T) {
	t.Run("version gap", func(t *testing.T) {
		store := newFakeStore()
		experiment, offers, snapshot := storedRecoveryExperiment(3)
		store.experiments[experiment.ID] = experiment
		store.offers[experiment.ID] = offers
		store.snapshots[experiment.ID] = []domain.PolicySnapshot{snapshot}
		decision := testServiceDecision(testUUID(5_000), experiment, offers, testNow())
		store.decisionOutcomes = []DecisionOutcome{{
			Decision: decision,
			Outcome:  domain.Outcome{EventID: testUUID(5_001), DecisionID: decision.ID, Kind: domain.OutcomeKindConverted, Reward: 1, OccurredAt: testNow(), ReceivedAt: testNow(), AppliedPolicyVersion: 3},
		}}
		engine := newTestEngine(t, store, actualPolicyFactory())
		if _, err := engine.Recover(context.Background()); err == nil {
			t.Fatal("Recover(version gap) error = nil")
		}
		if engine.IsHealthy(experiment.ID) {
			t.Fatal("gap recovery published a policy")
		}
	})

	t.Run("corrupt snapshot", func(t *testing.T) {
		store := newFakeStore()
		experiment, offers, snapshot := storedRecoveryExperiment(1)
		snapshot.State = []byte(`{"corrupt":true}`)
		store.experiments[experiment.ID] = experiment
		store.offers[experiment.ID] = offers
		store.snapshots[experiment.ID] = []domain.PolicySnapshot{snapshot}
		engine := newTestEngine(t, store, actualPolicyFactory())
		if _, err := engine.Recover(context.Background()); err == nil {
			t.Fatal("Recover(corrupt snapshot) error = nil")
		}
	})
}

func newTestEngine(t *testing.T, store Store, factory PolicyFactory) *Engine {
	t.Helper()
	clock := &stepClock{current: testNow(), step: time.Millisecond}
	ids := &sequenceIDs{next: 1}
	engine, err := NewEngine(store, factory, clock, ids, 2*time.Minute)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return engine
}

func actualPolicyFactory() PolicyFactory {
	return PolicyFactoryFunc(func(experiment domain.Experiment) (bandit.Policy, error) {
		switch experiment.PolicyKind {
		case domain.PolicyKindRandom:
			return bandit.NewRandomPolicy(experiment.ID, 1, bandit.NewLockedRandom(20260717))
		case domain.PolicyKindSegmentedEpsilonGreedy:
			if experiment.Epsilon == nil {
				return nil, errors.New("epsilon is required")
			}
			return bandit.NewEpsilonGreedyPolicy(
				experiment.ID,
				*experiment.Epsilon,
				bandit.DefaultPriorCount,
				bandit.DefaultPriorRewardSum,
				1,
				bandit.NewLockedRandom(20260717),
			)
		default:
			return nil, errors.New("unknown policy kind")
		}
	})
}

func createRunningExperiment(t *testing.T, engine *Engine, kind domain.PolicyKind) (domain.Experiment, []domain.Offer) {
	t.Helper()
	experiment := domain.Experiment{Slug: "service-test", Name: "Service Test", Status: domain.ExperimentStatusRunning, PolicyKind: kind}
	if kind == domain.PolicyKindSegmentedEpsilonGreedy {
		epsilon := 0.2
		experiment.Epsilon = &epsilon
	}
	offers := []domain.Offer{
		{Slug: "offer-a", MerchantName: "Fictional A", Title: "Offer A", Category: domain.OfferCategoryTravel, Active: true},
		{Slug: "offer-b", MerchantName: "Fictional B", Title: "Offer B", Category: domain.OfferCategoryHome, Active: true},
	}
	created, err := engine.CreateExperiment(context.Background(), experiment, offers)
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	storedOffers := append([]domain.Offer(nil), engine.store.(*fakeStore).offers[created.ID]...)
	sort.Slice(storedOffers, func(left, right int) bool { return storedOffers[left].ID.String() < storedOffers[right].ID.String() })
	return created, storedOffers
}

func storedRecoveryExperiment(version int64) (domain.Experiment, []domain.Offer, domain.PolicySnapshot) {
	experimentID := testUUID(6_000)
	experiment := domain.Experiment{ID: experimentID, Slug: "recovery", Name: "Recovery", Status: domain.ExperimentStatusRunning, PolicyKind: domain.PolicyKindRandom, PolicyVersion: version, CreatedAt: testNow(), UpdatedAt: testNow()}
	offers := []domain.Offer{
		{ID: testUUID(6_001), ExperimentID: experimentID, Slug: "recovery-a", MerchantName: "Fictional A", Title: "A", Category: domain.OfferCategoryTravel, Active: true},
		{ID: testUUID(6_002), ExperimentID: experimentID, Slug: "recovery-b", MerchantName: "Fictional B", Title: "B", Category: domain.OfferCategoryHome, Active: true},
	}
	sort.Slice(offers, func(left, right int) bool { return offers[left].ID.String() < offers[right].ID.String() })
	snapshot := domain.PolicySnapshot{ExperimentID: experimentID, PolicyKind: domain.PolicyKindRandom, PolicyVersion: 1, SchemaVersion: 1, State: []byte(`{}`), CreatedAt: testNow()}
	return experiment, offers, snapshot
}

func testServiceDecision(id uuid.UUID, experiment domain.Experiment, offers []domain.Offer, createdAt time.Time) domain.Decision {
	return domain.Decision{
		ID: id, ExperimentID: experiment.ID, SelectedOfferID: offers[0].ID,
		Context: testContext(), SegmentKey: "mobile|evening|travel|returning",
		EligibleOfferIDs: []uuid.UUID{offers[0].ID, offers[1].ID},
		Distribution:     []domain.ActionProbability{{OfferID: offers[0].ID, Probability: 0.5}, {OfferID: offers[1].ID, Probability: 0.5}},
		Propensity:       0.5, PolicyKind: experiment.PolicyKind, PolicyVersion: 1,
		PolicyLatencyMicros: 10, RequestID: "request", CreatedAt: createdAt,
	}
}

func testContext() domain.SessionContext {
	return domain.SessionContext{DeviceClass: domain.DeviceClassMobile, Daypart: domain.DaypartEvening, CategoryAffinity: domain.OfferCategoryTravel, VisitorType: domain.VisitorTypeReturning}
}

func testNow() time.Time {
	return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
}

func testUUID(value int) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("service-test-%08d", value)))
}

type stepClock struct {
	mu      sync.Mutex
	current time.Time
	step    time.Duration
}

func (clock *stepClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	value := clock.current
	clock.current = clock.current.Add(clock.step)
	return value
}

type sequenceIDs struct {
	mu   sync.Mutex
	next int
}

func (ids *sequenceIDs) New() uuid.UUID {
	ids.mu.Lock()
	defer ids.mu.Unlock()
	value := testUUID(ids.next)
	ids.next++
	return value
}

type fakeStore struct {
	Store
	mu                  sync.Mutex
	experiments         map[uuid.UUID]domain.Experiment
	offers              map[uuid.UUID][]domain.Offer
	decisions           map[uuid.UUID]domain.Decision
	outcomes            map[uuid.UUID]domain.Outcome
	snapshots           map[uuid.UUID][]domain.PolicySnapshot
	decisionOutcomes    []DecisionOutcome
	summary             SummaryAggregate
	performance         []OfferPerformanceRecord
	series              []domain.LearningSeriesPoint
	benchmark           SimulationBenchmarkRecord
	benchmarkFound      bool
	lastMaxPoints       int
	reconciledRuns      int64
	saveSnapshotErr     error
	createExperimentErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		experiments: make(map[uuid.UUID]domain.Experiment),
		offers:      make(map[uuid.UUID][]domain.Offer),
		decisions:   make(map[uuid.UUID]domain.Decision),
		outcomes:    make(map[uuid.UUID]domain.Outcome),
		snapshots:   make(map[uuid.UUID][]domain.PolicySnapshot),
	}
}

func (store *fakeStore) CreateExperiment(_ context.Context, experiment domain.Experiment, offers []domain.Offer, snapshot domain.PolicySnapshot) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.createExperimentErr != nil {
		return store.createExperimentErr
	}
	store.experiments[experiment.ID] = experiment
	store.offers[experiment.ID] = append([]domain.Offer(nil), offers...)
	store.snapshots[experiment.ID] = append(store.snapshots[experiment.ID], snapshot)
	return nil
}

func (store *fakeStore) GetExperiment(ctx context.Context, id uuid.UUID) (domain.Experiment, error) {
	if err := ctx.Err(); err != nil {
		return domain.Experiment{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	experiment, exists := store.experiments[id]
	if !exists {
		return domain.Experiment{}, ErrNotFound
	}
	return experiment, nil
}

func (store *fakeStore) ListExperiments(context.Context, *ExperimentCursor, int) ([]domain.Experiment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]domain.Experiment, 0, len(store.experiments))
	for _, experiment := range store.experiments {
		result = append(result, experiment)
	}
	return result, nil
}

func (store *fakeStore) ListActiveExperiments(ctx context.Context) ([]domain.Experiment, error) {
	return store.listActiveExperiments(ctx)
}

func (store *fakeStore) listActiveExperiments(ctx context.Context) ([]domain.Experiment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	var result []domain.Experiment
	for _, experiment := range store.experiments {
		if experiment.Status == domain.ExperimentStatusRunning || experiment.Status == domain.ExperimentStatusPaused {
			result = append(result, experiment)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID.String() < result[right].ID.String() })
	return result, nil
}

func (store *fakeStore) ListActiveOffers(_ context.Context, experimentID uuid.UUID) ([]domain.Offer, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]domain.Offer(nil), store.offers[experimentID]...), nil
}

func (store *fakeStore) InsertDecision(ctx context.Context, decision domain.Decision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.decisions[decision.ID] = decision
	return nil
}

func (store *fakeStore) GetDecision(ctx context.Context, id uuid.UUID) (domain.Decision, error) {
	if err := ctx.Err(); err != nil {
		return domain.Decision{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	decision, exists := store.decisions[id]
	if !exists {
		return domain.Decision{}, ErrNotFound
	}
	return decision, nil
}

func (store *fakeStore) ListDecisions(context.Context, uuid.UUID, *DecisionCursor, int) ([]domain.Decision, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]domain.Decision, 0, len(store.decisions))
	for _, decision := range store.decisions {
		result = append(result, decision)
	}
	return result, nil
}

func (store *fakeStore) AcceptOutcome(_ context.Context, experimentID uuid.UUID, candidate domain.Outcome) (OutcomeAcceptance, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, existing := range store.outcomes {
		if existing.EventID == candidate.EventID || existing.DecisionID == candidate.DecisionID {
			status := OutcomeAcceptanceConflict
			if existing.EventID == candidate.EventID && existing.DecisionID == candidate.DecisionID && existing.Kind == candidate.Kind && existing.Reward == candidate.Reward && existing.OccurredAt.Equal(candidate.OccurredAt) {
				status = OutcomeAcceptanceExactRetry
			}
			return OutcomeAcceptance{Status: status, Outcome: existing}, nil
		}
	}
	experiment := store.experiments[experimentID]
	experiment.PolicyVersion++
	store.experiments[experimentID] = experiment
	candidate.AppliedPolicyVersion = experiment.PolicyVersion
	store.outcomes[candidate.EventID] = candidate
	decision := store.decisions[candidate.DecisionID]
	store.decisionOutcomes = append(store.decisionOutcomes, DecisionOutcome{Decision: decision, Outcome: candidate})
	return OutcomeAcceptance{Status: OutcomeAcceptanceCreated, Outcome: candidate}, nil
}

func (store *fakeStore) SavePolicySnapshot(_ context.Context, snapshot domain.PolicySnapshot) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.saveSnapshotErr != nil {
		return store.saveSnapshotErr
	}
	store.snapshots[snapshot.ExperimentID] = append(store.snapshots[snapshot.ExperimentID], snapshot)
	return nil
}

func (store *fakeStore) GetLatestPolicySnapshot(_ context.Context, experimentID uuid.UUID) (domain.PolicySnapshot, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	snapshots := store.snapshots[experimentID]
	if len(snapshots) == 0 {
		return domain.PolicySnapshot{}, ErrNotFound
	}
	latest := snapshots[0]
	for _, snapshot := range snapshots[1:] {
		if snapshot.PolicyVersion > latest.PolicyVersion {
			latest = snapshot
		}
	}
	return latest, nil
}

func (store *fakeStore) ListDecisionOutcomesAfterVersion(_ context.Context, experimentID uuid.UUID, version int64) ([]DecisionOutcome, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var result []DecisionOutcome
	for _, record := range store.decisionOutcomes {
		if record.Decision.ExperimentID == experimentID && record.Outcome.AppliedPolicyVersion > version {
			result = append(result, record)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Outcome.AppliedPolicyVersion < result[right].Outcome.AppliedPolicyVersion
	})
	return result, nil
}

func (store *fakeStore) GetSummaryAggregate(context.Context, uuid.UUID) (SummaryAggregate, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.summary, nil
}

func (store *fakeStore) GetLearningSeries(_ context.Context, _ uuid.UUID, maxPoints int) ([]domain.LearningSeriesPoint, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.lastMaxPoints = maxPoints
	return append([]domain.LearningSeriesPoint(nil), store.series...), nil
}

func (store *fakeStore) GetOfferPerformance(context.Context, uuid.UUID) ([]OfferPerformanceRecord, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]OfferPerformanceRecord(nil), store.performance...), nil
}

func (store *fakeStore) ListDecisionOutcomes(context.Context, uuid.UUID) ([]DecisionOutcome, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]DecisionOutcome(nil), store.decisionOutcomes...), nil
}

func (store *fakeStore) GetLatestSimulationBenchmark(context.Context, uuid.UUID) (SimulationBenchmarkRecord, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.benchmark, store.benchmarkFound, nil
}

func (store *fakeStore) ReconcileInterruptedRuns(context.Context, time.Time) (int64, error) {
	return store.reconciledRuns, nil
}

type invalidSelectionPolicy struct {
	experimentID uuid.UUID
	version      int64
}

func (*invalidSelectionPolicy) Kind() domain.PolicyKind { return domain.PolicyKindRandom }
func (policy *invalidSelectionPolicy) Version() int64   { return policy.version }
func (policy *invalidSelectionPolicy) View() bandit.PolicyView {
	return bandit.PolicyView{Kind: domain.PolicyKindRandom, Version: policy.version}
}
func (policy *invalidSelectionPolicy) Select(input bandit.SelectionInput) (bandit.Selection, error) {
	return bandit.Selection{
		SelectedOfferID: input.OfferIDs[0],
		Distribution: []domain.ActionProbability{
			{OfferID: input.OfferIDs[0], Probability: 0.2},
			{OfferID: input.OfferIDs[1], Probability: 0.2},
		},
		PolicyKind:    domain.PolicyKindRandom,
		PolicyVersion: policy.version,
	}, nil
}
func (policy *invalidSelectionPolicy) Update(update bandit.Update) error {
	policy.version = update.AppliedPolicyVersion
	return nil
}
func (policy *invalidSelectionPolicy) Snapshot() (bandit.Snapshot, error) {
	return bandit.Snapshot{SchemaVersion: 1, ExperimentID: policy.experimentID, PolicyKind: domain.PolicyKindRandom, PolicyVersion: policy.version, State: []byte(`{}`)}, nil
}
func (policy *invalidSelectionPolicy) Restore(snapshot bandit.Snapshot) error {
	policy.version = snapshot.PolicyVersion
	return nil
}
