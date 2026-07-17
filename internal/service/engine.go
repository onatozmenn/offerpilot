package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/bandit"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrOutcomeConflict      = errors.New("outcome already recorded")
	ErrSnapshotConflict     = errors.New("policy snapshot conflict")
	ErrSimulationConflict   = errors.New("simulation already running")
	ErrExperimentNotRunning = errors.New("experiment is not running")
	ErrInsufficientOffers   = errors.New("fewer than two active offers")
	ErrPolicyUnhealthy      = errors.New("policy is unhealthy")
)

type ExperimentCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type DecisionCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type OutcomeAcceptanceStatus string

const (
	OutcomeAcceptanceCreated    OutcomeAcceptanceStatus = "created"
	OutcomeAcceptanceExactRetry OutcomeAcceptanceStatus = "exact_retry"
	OutcomeAcceptanceConflict   OutcomeAcceptanceStatus = "conflict"
)

type OutcomeAcceptance struct {
	Status  OutcomeAcceptanceStatus
	Outcome domain.Outcome
}

type DecisionOutcome struct {
	Decision domain.Decision
	Outcome  domain.Outcome
}

type SummaryAggregate struct {
	DecisionCount          int64
	OutcomeCount           int64
	RewardSum              float64
	IgnoredCount           int64
	ClickedCount           int64
	ConvertedCount         int64
	P50PolicyLatencyMicros *int64
	P95PolicyLatencyMicros *int64
}

type OfferPerformanceRecord struct {
	Offer          domain.Offer
	SelectionCount int64
	OutcomeCount   int64
	IgnoredCount   int64
	ClickedCount   int64
	ConvertedCount int64
	RewardSum      float64
}

type SimulationBenchmarkRecord struct {
	RunID                   uuid.UUID
	DecisionCount           int64
	OutcomeCount            int64
	ObservedRewardSum       float64
	RandomExpectedRewardSum float64
	OracleExpectedRewardSum float64
}

type Store interface {
	CreateExperiment(context.Context, domain.Experiment, []domain.Offer, domain.PolicySnapshot) error
	GetExperiment(context.Context, uuid.UUID) (domain.Experiment, error)
	ListExperiments(context.Context, *ExperimentCursor, int) ([]domain.Experiment, error)
	ListActiveExperiments(context.Context) ([]domain.Experiment, error)
	ListActiveOffers(context.Context, uuid.UUID) ([]domain.Offer, error)

	InsertDecision(context.Context, domain.Decision) error
	GetDecision(context.Context, uuid.UUID) (domain.Decision, error)
	ListDecisions(context.Context, uuid.UUID, *DecisionCursor, int) ([]domain.Decision, error)

	AcceptOutcome(context.Context, uuid.UUID, domain.Outcome) (OutcomeAcceptance, error)
	SavePolicySnapshot(context.Context, domain.PolicySnapshot) error
	GetLatestPolicySnapshot(context.Context, uuid.UUID) (domain.PolicySnapshot, error)
	ListDecisionOutcomesAfterVersion(context.Context, uuid.UUID, int64) ([]DecisionOutcome, error)

	GetSummaryAggregate(context.Context, uuid.UUID) (SummaryAggregate, error)
	GetLearningSeries(context.Context, uuid.UUID, int) ([]domain.LearningSeriesPoint, error)
	GetOfferPerformance(context.Context, uuid.UUID) ([]OfferPerformanceRecord, error)
	ListDecisionOutcomes(context.Context, uuid.UUID) ([]DecisionOutcome, error)
	GetLatestSimulationBenchmark(context.Context, uuid.UUID) (SimulationBenchmarkRecord, bool, error)

	CreateSimulationRun(context.Context, domain.SimulationRun) error
	GetSimulationRun(context.Context, uuid.UUID) (domain.SimulationRun, error)
	UpdateSimulationRun(context.Context, domain.SimulationRun) error
	ReconcileInterruptedRuns(context.Context, time.Time) (int64, error)
}

type PolicyFactory interface {
	NewPolicy(domain.Experiment) (bandit.Policy, error)
}

type PolicyFactoryFunc func(domain.Experiment) (bandit.Policy, error)

func (function PolicyFactoryFunc) NewPolicy(experiment domain.Experiment) (bandit.Policy, error) {
	return function(experiment)
}

type Clock interface {
	Now() time.Time
}

type ClockFunc func() time.Time

func (function ClockFunc) Now() time.Time {
	return function()
}

type IDGenerator interface {
	New() uuid.UUID
}

type IDGeneratorFunc func() uuid.UUID

func (function IDGeneratorFunc) New() uuid.UUID {
	return function()
}

type DecideCommand struct {
	ExperimentID    uuid.UUID
	Context         domain.SessionContext
	RequestID       string
	SimulationRunID *uuid.UUID
}

type RecordOutcomeCommand struct {
	EventID    uuid.UUID
	DecisionID uuid.UUID
	Kind       domain.OutcomeKind
	OccurredAt time.Time
}

type RecordOutcomeResult struct {
	Outcome       domain.Outcome
	Created       bool
	ExactRetry    bool
	PolicyUpdated bool
	SnapshotSaved bool
}

type Engine struct {
	store         Store
	policyFactory PolicyFactory
	clock         Clock
	ids           IDGenerator
	maxFutureSkew time.Duration

	mu          sync.RWMutex
	policies    map[uuid.UUID]bandit.Policy
	unhealthy   map[uuid.UUID]error
	updateLocks map[uuid.UUID]*sync.Mutex
}

func NewEngine(
	store Store,
	policyFactory PolicyFactory,
	clock Clock,
	ids IDGenerator,
	maxFutureSkew time.Duration,
) (*Engine, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}
	if policyFactory == nil {
		return nil, errors.New("policy factory is required")
	}
	if clock == nil {
		return nil, errors.New("clock is required")
	}
	if ids == nil {
		return nil, errors.New("id generator is required")
	}
	if maxFutureSkew < 0 {
		return nil, errors.New("max future skew must not be negative")
	}

	return &Engine{
		store:         store,
		policyFactory: policyFactory,
		clock:         clock,
		ids:           ids,
		maxFutureSkew: maxFutureSkew,
		policies:      make(map[uuid.UUID]bandit.Policy),
		unhealthy:     make(map[uuid.UUID]error),
		updateLocks:   make(map[uuid.UUID]*sync.Mutex),
	}, nil
}

func (engine *Engine) CreateExperiment(
	ctx context.Context,
	experiment domain.Experiment,
	offers []domain.Offer,
) (domain.Experiment, error) {
	now := engine.clock.Now().UTC()
	if experiment.ID == uuid.Nil {
		experiment.ID = engine.ids.New()
	}
	if experiment.PolicyVersion == 0 {
		experiment.PolicyVersion = 1
	}
	if experiment.CreatedAt.IsZero() {
		experiment.CreatedAt = now
	}
	if experiment.UpdatedAt.IsZero() {
		experiment.UpdatedAt = experiment.CreatedAt
	}

	normalizedOffers := append([]domain.Offer(nil), offers...)
	for index := range normalizedOffers {
		if normalizedOffers[index].ID == uuid.Nil {
			normalizedOffers[index].ID = engine.ids.New()
		}
		normalizedOffers[index].ExperimentID = experiment.ID
	}
	if err := domain.ValidateExperiment(experiment); err != nil {
		return domain.Experiment{}, fmt.Errorf("validate experiment: %w", err)
	}
	if err := domain.ValidateOffers(experiment.ID, normalizedOffers); err != nil {
		return domain.Experiment{}, fmt.Errorf("validate offers: %w", err)
	}

	policy, err := engine.policyFactory.NewPolicy(experiment)
	if err != nil {
		return domain.Experiment{}, fmt.Errorf("create policy: %w", err)
	}
	if policy.Kind() != experiment.PolicyKind || policy.Version() != experiment.PolicyVersion {
		return domain.Experiment{}, fmt.Errorf("policy factory returned mismatched kind or version")
	}
	snapshot, err := policy.Snapshot()
	if err != nil {
		return domain.Experiment{}, fmt.Errorf("snapshot initial policy: %w", err)
	}
	domainSnapshot, err := domainSnapshot(snapshot, now)
	if err != nil {
		return domain.Experiment{}, err
	}
	if err := engine.store.CreateExperiment(ctx, experiment, normalizedOffers, domainSnapshot); err != nil {
		return domain.Experiment{}, fmt.Errorf("persist experiment: %w", err)
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()
	if _, exists := engine.policies[experiment.ID]; exists {
		return domain.Experiment{}, fmt.Errorf("experiment policy is already loaded")
	}
	engine.policies[experiment.ID] = policy
	engine.updateLocks[experiment.ID] = &sync.Mutex{}
	delete(engine.unhealthy, experiment.ID)

	return experiment, nil
}

func (engine *Engine) GetExperiment(ctx context.Context, experimentID uuid.UUID) (domain.Experiment, error) {
	return engine.store.GetExperiment(ctx, experimentID)
}

func (engine *Engine) ListExperiments(
	ctx context.Context,
	cursor *ExperimentCursor,
	limit int,
) ([]domain.Experiment, error) {
	return engine.store.ListExperiments(ctx, cursor, limit)
}

func (engine *Engine) Decide(ctx context.Context, command DecideCommand) (domain.Decision, error) {
	experiment, err := engine.store.GetExperiment(ctx, command.ExperimentID)
	if err != nil {
		return domain.Decision{}, err
	}
	if experiment.Status != domain.ExperimentStatusRunning {
		return domain.Decision{}, ErrExperimentNotRunning
	}
	segmentKey, err := domain.SegmentKey(command.Context)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("validate decision context: %w", err)
	}
	offers, err := engine.store.ListActiveOffers(ctx, experiment.ID)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("load active offers: %w", err)
	}
	if len(offers) < 2 {
		return domain.Decision{}, ErrInsufficientOffers
	}
	offerIDs := make([]uuid.UUID, len(offers))
	for index, offer := range offers {
		offerIDs[index] = offer.ID
	}
	canonicalOfferIDs, err := domain.CanonicalOfferIDs(offerIDs)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("canonicalize active offers: %w", err)
	}

	policy, err := engine.policy(experiment.ID)
	if err != nil {
		return domain.Decision{}, err
	}
	startedAt := engine.clock.Now()
	selection, err := policy.Select(bandit.SelectionInput{
		ExperimentID: experiment.ID,
		SegmentKey:   segmentKey,
		OfferIDs:     canonicalOfferIDs,
	})
	if err != nil {
		return domain.Decision{}, fmt.Errorf("select offer: %w", err)
	}
	completedAt := engine.clock.Now()
	if completedAt.Before(startedAt) {
		return domain.Decision{}, fmt.Errorf("policy clock moved backwards")
	}
	if selection.PolicyKind != experiment.PolicyKind {
		return domain.Decision{}, fmt.Errorf("policy selection kind is invalid")
	}
	selectedProbability, err := domain.ValidateDistribution(
		canonicalOfferIDs,
		selection.Distribution,
		selection.SelectedOfferID,
	)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("validate policy distribution: %w", err)
	}
	if err := domain.ValidatePropensity(selectedProbability, selectedProbability); err != nil {
		return domain.Decision{}, fmt.Errorf("validate selected propensity: %w", err)
	}

	decision := domain.Decision{
		ID:                  engine.ids.New(),
		ExperimentID:        experiment.ID,
		SelectedOfferID:     selection.SelectedOfferID,
		Context:             command.Context,
		SegmentKey:          segmentKey,
		EligibleOfferIDs:    canonicalOfferIDs,
		Distribution:        append([]domain.ActionProbability(nil), selection.Distribution...),
		Propensity:          selectedProbability,
		PolicyKind:          selection.PolicyKind,
		PolicyVersion:       selection.PolicyVersion,
		PolicyLatencyMicros: completedAt.Sub(startedAt).Microseconds(),
		SimulationRunID:     cloneUUIDPointer(command.SimulationRunID),
		RequestID:           command.RequestID,
		CreatedAt:           completedAt.UTC(),
	}
	if err := domain.ValidateDecision(decision); err != nil {
		return domain.Decision{}, fmt.Errorf("validate decision: %w", err)
	}
	if err := engine.store.InsertDecision(ctx, decision); err != nil {
		return domain.Decision{}, fmt.Errorf("persist decision: %w", err)
	}
	return decision, nil
}

func (engine *Engine) GetDecision(ctx context.Context, decisionID uuid.UUID) (domain.Decision, error) {
	return engine.store.GetDecision(ctx, decisionID)
}

func (engine *Engine) ListDecisions(
	ctx context.Context,
	experimentID uuid.UUID,
	cursor *DecisionCursor,
	limit int,
) ([]domain.Decision, error) {
	return engine.store.ListDecisions(ctx, experimentID, cursor, limit)
}

func (engine *Engine) RecordOutcome(
	ctx context.Context,
	command RecordOutcomeCommand,
) (RecordOutcomeResult, error) {
	decision, err := engine.store.GetDecision(ctx, command.DecisionID)
	if err != nil {
		return RecordOutcomeResult{}, err
	}
	lock := engine.updateLock(decision.ExperimentID)
	lock.Lock()
	defer lock.Unlock()

	experiment, err := engine.store.GetExperiment(ctx, decision.ExperimentID)
	if err != nil {
		return RecordOutcomeResult{}, err
	}
	if experiment.Status != domain.ExperimentStatusRunning {
		return RecordOutcomeResult{}, ErrExperimentNotRunning
	}
	policy, err := engine.policy(experiment.ID)
	if err != nil {
		return RecordOutcomeResult{}, err
	}
	reward, err := domain.RewardForOutcome(command.Kind)
	if err != nil {
		return RecordOutcomeResult{}, err
	}
	now := engine.clock.Now().UTC()
	candidate := domain.Outcome{
		EventID:              command.EventID,
		DecisionID:           command.DecisionID,
		Kind:                 command.Kind,
		Reward:               reward,
		OccurredAt:           command.OccurredAt,
		ReceivedAt:           now,
		AppliedPolicyVersion: 1,
	}
	if err := domain.ValidateOutcome(candidate, now, engine.maxFutureSkew); err != nil {
		return RecordOutcomeResult{}, fmt.Errorf("validate outcome: %w", err)
	}
	candidate.AppliedPolicyVersion = 0

	acceptance, err := engine.store.AcceptOutcome(ctx, experiment.ID, candidate)
	if err != nil {
		return RecordOutcomeResult{}, fmt.Errorf("accept outcome: %w", err)
	}
	switch acceptance.Status {
	case OutcomeAcceptanceExactRetry:
		return RecordOutcomeResult{Outcome: acceptance.Outcome, ExactRetry: true}, nil
	case OutcomeAcceptanceConflict:
		return RecordOutcomeResult{Outcome: acceptance.Outcome}, ErrOutcomeConflict
	case OutcomeAcceptanceCreated:
	default:
		return RecordOutcomeResult{}, fmt.Errorf("unknown outcome acceptance status %q", acceptance.Status)
	}

	result := RecordOutcomeResult{Outcome: acceptance.Outcome, Created: true}
	update := bandit.Update{
		ExperimentID:           decision.ExperimentID,
		SegmentKey:             decision.SegmentKey,
		SelectedOfferID:        decision.SelectedOfferID,
		EligibleOfferIDs:       append([]uuid.UUID(nil), decision.EligibleOfferIDs...),
		SelectionPolicyVersion: decision.PolicyVersion,
		AppliedPolicyVersion:   acceptance.Outcome.AppliedPolicyVersion,
		Reward:                 acceptance.Outcome.Reward,
		PolicyKind:             decision.PolicyKind,
	}
	if err := policy.Update(update); err != nil {
		engine.markUnhealthy(experiment.ID, err)
		return result, fmt.Errorf("update policy after persisted outcome: %w", err)
	}
	result.PolicyUpdated = true

	snapshot, err := policy.Snapshot()
	if err != nil {
		engine.markUnhealthy(experiment.ID, err)
		return result, fmt.Errorf("snapshot updated policy: %w", err)
	}
	domainPolicySnapshot, err := domainSnapshot(snapshot, now)
	if err != nil {
		engine.markUnhealthy(experiment.ID, err)
		return result, err
	}
	if err := engine.store.SavePolicySnapshot(ctx, domainPolicySnapshot); err != nil {
		engine.markUnhealthy(experiment.ID, err)
		return result, fmt.Errorf("save policy snapshot: %w", err)
	}
	result.SnapshotSaved = true
	return result, nil
}

func (engine *Engine) PolicyView(experimentID uuid.UUID) (bandit.PolicyView, error) {
	policy, err := engine.policy(experimentID)
	if err != nil {
		return bandit.PolicyView{}, err
	}
	return policy.View(), nil
}

func (engine *Engine) IsHealthy(experimentID uuid.UUID) bool {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	_, loaded := engine.policies[experimentID]
	_, unhealthy := engine.unhealthy[experimentID]
	return loaded && !unhealthy
}

func (engine *Engine) Close() error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.policies = make(map[uuid.UUID]bandit.Policy)
	engine.unhealthy = make(map[uuid.UUID]error)
	engine.updateLocks = make(map[uuid.UUID]*sync.Mutex)
	return nil
}

func (engine *Engine) policy(experimentID uuid.UUID) (bandit.Policy, error) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	if unhealthy, exists := engine.unhealthy[experimentID]; exists {
		return nil, fmt.Errorf("%w: %v", ErrPolicyUnhealthy, unhealthy)
	}
	policy, exists := engine.policies[experimentID]
	if !exists {
		return nil, fmt.Errorf("%w: policy is not loaded", ErrPolicyUnhealthy)
	}
	return policy, nil
}

func (engine *Engine) updateLock(experimentID uuid.UUID) *sync.Mutex {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	lock, exists := engine.updateLocks[experimentID]
	if !exists {
		lock = &sync.Mutex{}
		engine.updateLocks[experimentID] = lock
	}
	return lock
}

func (engine *Engine) markUnhealthy(experimentID uuid.UUID, err error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.unhealthy[experimentID] = err
}

func (engine *Engine) installPolicy(experimentID uuid.UUID, policy bandit.Policy) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.policies[experimentID] = policy
	if _, exists := engine.updateLocks[experimentID]; !exists {
		engine.updateLocks[experimentID] = &sync.Mutex{}
	}
	delete(engine.unhealthy, experimentID)
}

func domainSnapshot(snapshot bandit.Snapshot, createdAt time.Time) (domain.PolicySnapshot, error) {
	converted := domain.PolicySnapshot{
		ExperimentID:  snapshot.ExperimentID,
		PolicyKind:    snapshot.PolicyKind,
		PolicyVersion: snapshot.PolicyVersion,
		SchemaVersion: snapshot.SchemaVersion,
		State:         append([]byte(nil), snapshot.State...),
		CreatedAt:     createdAt.UTC(),
	}
	if err := domain.ValidatePolicySnapshot(converted); err != nil {
		return domain.PolicySnapshot{}, fmt.Errorf("validate policy snapshot: %w", err)
	}
	return converted, nil
}

func banditSnapshot(snapshot domain.PolicySnapshot) bandit.Snapshot {
	return bandit.Snapshot{
		SchemaVersion: snapshot.SchemaVersion,
		ExperimentID:  snapshot.ExperimentID,
		PolicyKind:    snapshot.PolicyKind,
		PolicyVersion: snapshot.PolicyVersion,
		State:         append([]byte(nil), snapshot.State...),
	}
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
