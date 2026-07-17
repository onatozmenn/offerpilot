package service

import (
	"context"
	"errors"
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

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	New() uuid.UUID
}

type Engine struct {
	store         Store
	policyFactory PolicyFactory
	clock         Clock
	ids           IDGenerator

	mu        sync.RWMutex
	policies  map[uuid.UUID]bandit.Policy
	unhealthy map[uuid.UUID]error
}

func NewEngine(store Store, policyFactory PolicyFactory, clock Clock, ids IDGenerator) (*Engine, error) {
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

	return &Engine{
		store:         store,
		policyFactory: policyFactory,
		clock:         clock,
		ids:           ids,
		policies:      make(map[uuid.UUID]bandit.Policy),
		unhealthy:     make(map[uuid.UUID]error),
	}, nil
}
