package simulation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/service"
)

var ErrManagerUnhealthy = errors.New("simulation manager is unhealthy")

type RunStore interface {
	CreateSimulationRun(context.Context, domain.SimulationRun) error
	GetSimulationRun(context.Context, uuid.UUID) (domain.SimulationRun, error)
	UpdateSimulationRun(context.Context, domain.SimulationRun) error
	ReconcileInterruptedRuns(context.Context, time.Time) (int64, error)
}

type EngineUseCases interface {
	Decide(context.Context, service.DecideCommand) (domain.Decision, error)
	RecordOutcome(context.Context, service.RecordOutcomeCommand) (service.RecordOutcomeResult, error)
	GetExperimentDetail(context.Context, uuid.UUID) (domain.Experiment, []domain.Offer, error)
}

type EngineClient struct {
	engine EngineUseCases
}

func NewEngineClient(engine EngineUseCases) (*EngineClient, error) {
	if engine == nil {
		return nil, fmt.Errorf("engine is required")
	}
	return &EngineClient{engine: engine}, nil
}

func (client *EngineClient) Decide(
	ctx context.Context,
	experimentID uuid.UUID,
	simulationRunID *uuid.UUID,
	contextValue domain.SessionContext,
	requestID string,
) (DecisionResult, error) {
	decision, err := client.engine.Decide(ctx, service.DecideCommand{
		ExperimentID:    experimentID,
		Context:         contextValue,
		RequestID:       requestID,
		SimulationRunID: cloneUUIDPointer(simulationRunID),
	})
	if err != nil {
		return DecisionResult{}, err
	}
	_, offers, err := client.engine.GetExperimentDetail(ctx, experimentID)
	if err != nil {
		return DecisionResult{}, err
	}
	selectedSlug := ""
	for _, offer := range offers {
		if offer.ID == decision.SelectedOfferID {
			selectedSlug = offer.Slug
			break
		}
	}
	if selectedSlug == "" {
		return DecisionResult{}, fmt.Errorf("selected offer is absent from experiment catalog")
	}
	return DecisionResult{
		DecisionID:        decision.ID,
		SelectedOfferID:   decision.SelectedOfferID,
		SelectedOfferSlug: selectedSlug,
		Propensity:        decision.Propensity,
		PolicyKind:        decision.PolicyKind,
		PolicyVersion:     decision.PolicyVersion,
		CreatedAt:         decision.CreatedAt,
	}, nil
}

func (client *EngineClient) SubmitOutcome(
	ctx context.Context,
	eventID uuid.UUID,
	decisionID uuid.UUID,
	kind domain.OutcomeKind,
	occurredAt time.Time,
	_ string,
) (domain.Outcome, error) {
	result, err := client.engine.RecordOutcome(ctx, service.RecordOutcomeCommand{
		EventID:    eventID,
		DecisionID: decisionID,
		Kind:       kind,
		OccurredAt: occurredAt,
	})
	if err != nil {
		return domain.Outcome{}, err
	}
	return result.Outcome, nil
}

type ManagerConfig struct {
	Workers        int
	MaxErrors      int
	ProgressEvery  int
	PersistTimeout time.Duration
}

type Manager struct {
	store  RunStore
	runner RunExecutor
	clock  SimulationClock
	ids    func() uuid.UUID
	config ManagerConfig

	mu                 sync.Mutex
	runs               map[uuid.UUID]*managedRun
	activeByExperiment map[uuid.UUID]uuid.UUID
	unhealthy          error
	shuttingDown       bool
}

type managedRun struct {
	run             domain.SimulationRun
	cancel          context.CancelFunc
	done            chan struct{}
	stopRequested   bool
	terminalPending bool
}

func NewManager(store RunStore, runner RunExecutor, clock SimulationClock, ids func() uuid.UUID, config ManagerConfig) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("run store is required")
	}
	if runner == nil {
		return nil, fmt.Errorf("runner is required")
	}
	if clock == nil {
		return nil, fmt.Errorf("simulation clock is required")
	}
	if ids == nil {
		return nil, fmt.Errorf("run ID generator is required")
	}
	if config.Workers < 1 || config.Workers > 32 || config.MaxErrors < 1 || config.MaxErrors > 100 || config.ProgressEvery < 1 || config.PersistTimeout <= 0 {
		return nil, fmt.Errorf("manager bounds and persistence timeout are invalid")
	}
	return &Manager{
		store:              store,
		runner:             runner,
		clock:              clock,
		ids:                ids,
		config:             config,
		runs:               make(map[uuid.UUID]*managedRun),
		activeByExperiment: make(map[uuid.UUID]uuid.UUID),
	}, nil
}

func (manager *Manager) RecoverInterrupted(ctx context.Context) (int64, error) {
	count, err := manager.store.ReconcileInterruptedRuns(ctx, manager.clock.Now().UTC())
	if err != nil {
		manager.setUnhealthy(err)
		return 0, fmt.Errorf("reconcile interrupted runs: %w", err)
	}
	return count, nil
}

func (manager *Manager) Start(
	ctx context.Context,
	experimentID uuid.UUID,
	seed int64,
	requestsPerSecond int,
	maxDecisions int,
) (domain.SimulationRun, error) {
	if experimentID == uuid.Nil {
		return domain.SimulationRun{}, fmt.Errorf("experiment id must not be nil")
	}
	manager.mu.Lock()
	if manager.shuttingDown {
		manager.mu.Unlock()
		return domain.SimulationRun{}, fmt.Errorf("simulation manager is shutting down")
	}
	if manager.unhealthy != nil {
		err := manager.unhealthy
		manager.mu.Unlock()
		return domain.SimulationRun{}, fmt.Errorf("%w: %v", ErrManagerUnhealthy, err)
	}
	if _, exists := manager.activeByExperiment[experimentID]; exists {
		manager.mu.Unlock()
		return domain.SimulationRun{}, service.ErrSimulationConflict
	}
	startedAt := manager.clock.Now().UTC()
	run := domain.SimulationRun{
		ID:                manager.ids(),
		ExperimentID:      experimentID,
		Seed:              seed,
		RequestsPerSecond: requestsPerSecond,
		MaxDecisions:      maxDecisions,
		Status:            domain.SimulationRunStatusStarting,
		StartedAt:         startedAt,
		UpdatedAt:         startedAt,
	}
	if err := domain.ValidateSimulationRun(run); err != nil {
		manager.mu.Unlock()
		return domain.SimulationRun{}, err
	}
	managed := &managedRun{run: run, done: make(chan struct{})}
	manager.runs[run.ID] = managed
	manager.activeByExperiment[experimentID] = run.ID
	manager.mu.Unlock()

	if err := manager.store.CreateSimulationRun(ctx, run); err != nil {
		manager.release(run.ID, experimentID)
		return domain.SimulationRun{}, fmt.Errorf("persist starting simulation: %w", err)
	}
	run.Status = domain.SimulationRunStatusRunning
	run.UpdatedAt = manager.clock.Now().UTC()
	if err := manager.store.UpdateSimulationRun(ctx, run); err != nil {
		manager.release(run.ID, experimentID)
		manager.setUnhealthy(err)
		return domain.SimulationRun{}, fmt.Errorf("persist running simulation: %w", err)
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	manager.mu.Lock()
	managed.run = run
	managed.cancel = cancel
	manager.mu.Unlock()
	go manager.execute(managed, RunConfig{
		ExperimentID:      experimentID,
		SimulationRunID:   cloneUUIDPointer(&run.ID),
		Seed:              seed,
		RequestsPerSecond: requestsPerSecond,
		MaxDecisions:      maxDecisions,
		Workers:           manager.config.Workers,
		MaxErrors:         manager.config.MaxErrors,
		ProgressEvery:     min(manager.config.ProgressEvery, maxDecisions),
		OnProgress: func(progressCtx context.Context, progress Progress) error {
			return manager.persistProgress(progressCtx, run.ID, progress)
		},
	}, runCtx)
	return run, nil
}

func (manager *Manager) Get(ctx context.Context, runID uuid.UUID) (domain.SimulationRun, error) {
	manager.mu.Lock()
	managed, exists := manager.runs[runID]
	if exists {
		run := cloneRun(managed.run)
		manager.mu.Unlock()
		return run, nil
	}
	manager.mu.Unlock()
	return manager.store.GetSimulationRun(ctx, runID)
}

func (manager *Manager) Stop(ctx context.Context, runID uuid.UUID) (domain.SimulationRun, error) {
	manager.mu.Lock()
	managed, exists := manager.runs[runID]
	if !exists {
		manager.mu.Unlock()
		return manager.store.GetSimulationRun(ctx, runID)
	}
	if managed.run.Status == domain.SimulationRunStatusCompleted || managed.run.Status == domain.SimulationRunStatusFailed || managed.run.Status == domain.SimulationRunStatusCancelled {
		run := cloneRun(managed.run)
		manager.mu.Unlock()
		return run, nil
	}
	if managed.stopRequested {
		run := cloneRun(managed.run)
		manager.mu.Unlock()
		return run, nil
	}
	managed.stopRequested = true
	managed.run.Status = domain.SimulationRunStatusStopping
	managed.run.UpdatedAt = manager.clock.Now().UTC()
	run := cloneRun(managed.run)
	cancel := managed.cancel
	manager.mu.Unlock()

	if err := manager.store.UpdateSimulationRun(ctx, run); err != nil {
		manager.mu.Lock()
		managed.stopRequested = false
		managed.run.Status = domain.SimulationRunStatusRunning
		manager.mu.Unlock()
		return domain.SimulationRun{}, fmt.Errorf("persist stopping simulation: %w", err)
	}
	if cancel != nil {
		cancel()
	}
	return run, nil
}

func (manager *Manager) Shutdown(ctx context.Context) error {
	manager.mu.Lock()
	manager.shuttingDown = true
	active := make([]*managedRun, 0, len(manager.activeByExperiment))
	for _, runID := range manager.activeByExperiment {
		active = append(active, manager.runs[runID])
	}
	manager.mu.Unlock()

	for _, managed := range active {
		_, _ = manager.Stop(ctx, managed.run.ID)
	}
	for _, managed := range active {
		select {
		case <-managed.done:
		case <-ctx.Done():
			return fmt.Errorf("wait for simulation shutdown: %w", ctx.Err())
		}
		manager.mu.Lock()
		pending := managed.terminalPending
		run := cloneRun(managed.run)
		manager.mu.Unlock()
		if pending {
			if err := manager.store.UpdateSimulationRun(ctx, run); err != nil {
				return fmt.Errorf("retry terminal simulation persistence: %w", err)
			}
			manager.completePersistence(run.ID, run.ExperimentID)
		}
	}
	return nil
}

func (manager *Manager) Ready(context.Context) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.unhealthy != nil {
		return fmt.Errorf("%w: %v", ErrManagerUnhealthy, manager.unhealthy)
	}
	return nil
}

func (manager *Manager) execute(managed *managedRun, config RunConfig, runCtx context.Context) {
	var result RunResult
	var runErr error
	defer func(ctx context.Context) {
		if recovered := recover(); recovered != nil {
			runErr = fmt.Errorf("runner panic type %T", recovered)
		}
		manager.finish(ctx, managed, result, runErr)
		close(managed.done)
	}(runCtx)
	result, runErr = manager.runner.Run(runCtx, config)
}

func (manager *Manager) persistProgress(ctx context.Context, runID uuid.UUID, progress Progress) error {
	manager.mu.Lock()
	managed, exists := manager.runs[runID]
	if !exists {
		manager.mu.Unlock()
		return service.ErrNotFound
	}
	managed.run.DecisionCount = progress.DecisionCount
	managed.run.OutcomeCount = progress.OutcomeCount
	managed.run.ErrorCount = progress.ErrorCount
	managed.run.ObservedRewardSum = progress.ObservedRewardSum
	managed.run.RandomExpectedRewardSum = progress.RandomExpectedRewardSum
	managed.run.OracleExpectedRewardSum = progress.OracleExpectedRewardSum
	managed.run.UpdatedAt = manager.clock.Now().UTC()
	run := cloneRun(managed.run)
	manager.mu.Unlock()
	if err := manager.store.UpdateSimulationRun(ctx, run); err != nil {
		return fmt.Errorf("persist simulation progress: %w", err)
	}
	return nil
}

func (manager *Manager) finish(ctx context.Context, managed *managedRun, result RunResult, runErr error) {
	manager.mu.Lock()
	managed.run.DecisionCount = result.DecisionCount
	managed.run.OutcomeCount = result.OutcomeCount
	managed.run.ErrorCount = result.ErrorCount
	managed.run.ObservedRewardSum = result.ObservedRewardSum
	managed.run.RandomExpectedRewardSum = result.RandomExpectedRewardSum
	managed.run.OracleExpectedRewardSum = result.OracleExpectedRewardSum
	stoppedAt := manager.clock.Now().UTC()
	managed.run.StoppedAt = &stoppedAt
	managed.run.UpdatedAt = stoppedAt
	switch {
	case runErr == nil:
		managed.run.Status = domain.SimulationRunStatusCompleted
	case errors.Is(runErr, context.Canceled) && managed.stopRequested:
		managed.run.Status = domain.SimulationRunStatusCancelled
	default:
		managed.run.Status = domain.SimulationRunStatusFailed
		code := "runner_failed"
		detail := "Simulation runner failed"
		managed.run.ErrorCode = &code
		managed.run.ErrorDetail = &detail
	}
	run := cloneRun(managed.run)
	manager.mu.Unlock()

	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), manager.config.PersistTimeout)
	defer cancel()
	if err := manager.store.UpdateSimulationRun(persistCtx, run); err != nil {
		manager.mu.Lock()
		managed.terminalPending = true
		code := "terminal_persistence_failed"
		detail := "Terminal simulation state could not be persisted"
		managed.run.Status = domain.SimulationRunStatusFailed
		managed.run.ErrorCode = &code
		managed.run.ErrorDetail = &detail
		manager.unhealthy = err
		manager.mu.Unlock()
		return
	}
	manager.completePersistence(run.ID, run.ExperimentID)
}

func (manager *Manager) completePersistence(runID, experimentID uuid.UUID) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if managed, exists := manager.runs[runID]; exists {
		managed.terminalPending = false
	}
	delete(manager.activeByExperiment, experimentID)
	if len(manager.activeByExperiment) == 0 {
		manager.unhealthy = nil
	}
}

func (manager *Manager) release(runID, experimentID uuid.UUID) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	delete(manager.runs, runID)
	delete(manager.activeByExperiment, experimentID)
}

func (manager *Manager) setUnhealthy(err error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.unhealthy = err
}

func cloneRun(run domain.SimulationRun) domain.SimulationRun {
	clone := run
	if run.StoppedAt != nil {
		value := *run.StoppedAt
		clone.StoppedAt = &value
	}
	if run.ErrorCode != nil {
		value := *run.ErrorCode
		clone.ErrorCode = &value
	}
	if run.ErrorDetail != nil {
		value := *run.ErrorDetail
		clone.ErrorDetail = &value
	}
	return clone
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
