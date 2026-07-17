package simulation

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

var ErrMaxErrors = errors.New("simulation maximum error count reached")

type DecisionResult struct {
	DecisionID        uuid.UUID
	SelectedOfferID   uuid.UUID
	SelectedOfferSlug string
	Propensity        float64
	PolicyKind        domain.PolicyKind
	PolicyVersion     int64
	CreatedAt         time.Time
}

type DecisionClient interface {
	Decide(context.Context, uuid.UUID, *uuid.UUID, domain.SessionContext, string) (DecisionResult, error)
	SubmitOutcome(context.Context, uuid.UUID, uuid.UUID, domain.OutcomeKind, time.Time, string) (domain.Outcome, error)
}

type Limiter interface {
	Wait(context.Context) error
	Stop()
}

type LimiterFactory interface {
	New(int) (Limiter, error)
}

type LimiterFactoryFunc func(int) (Limiter, error)

func (function LimiterFactoryFunc) New(rate int) (Limiter, error) {
	return function(rate)
}

type SimulationClock interface {
	Now() time.Time
}

type SimulationClockFunc func() time.Time

func (function SimulationClockFunc) Now() time.Time {
	return function()
}

type Progress struct {
	AttemptCount            int64
	DecisionCount           int64
	OutcomeCount            int64
	ErrorCount              int64
	ObservedRewardSum       float64
	RandomExpectedRewardSum float64
	OracleExpectedRewardSum float64
}

type RunConfig struct {
	ExperimentID      uuid.UUID
	SimulationRunID   *uuid.UUID
	Seed              int64
	RequestsPerSecond int
	MaxDecisions      int
	Workers           int
	MaxErrors         int
	ProgressEvery     int
	OnProgress        func(context.Context, Progress) error
}

type RunResult struct {
	Progress
	StartedAt   time.Time
	CompletedAt time.Time
}

type RunExecutor interface {
	Run(context.Context, RunConfig) (RunResult, error)
}

type Runner struct {
	profile        *Profile
	client         DecisionClient
	limiterFactory LimiterFactory
	clock          SimulationClock
}

func NewRunner(profile *Profile, client DecisionClient, limiterFactory LimiterFactory, clock SimulationClock) (*Runner, error) {
	if profile == nil || profile.Version() != ProfileVersion {
		return nil, fmt.Errorf("valid profile is required")
	}
	if client == nil {
		return nil, fmt.Errorf("decision client is required")
	}
	if limiterFactory == nil {
		return nil, fmt.Errorf("limiter factory is required")
	}
	if clock == nil {
		return nil, fmt.Errorf("simulation clock is required")
	}
	return &Runner{profile: profile, client: client, limiterFactory: limiterFactory, clock: clock}, nil
}

func NewDefaultRunner(profile *Profile, client DecisionClient) (*Runner, error) {
	return NewRunner(profile, client, realLimiterFactory{}, SimulationClockFunc(time.Now))
}

func (runner *Runner) Run(ctx context.Context, config RunConfig) (RunResult, error) {
	if err := validateRunConfig(config); err != nil {
		return RunResult{}, err
	}
	limiter, err := runner.limiterFactory.New(config.RequestsPerSecond)
	if err != nil {
		return RunResult{}, fmt.Errorf("create rate limiter: %w", err)
	}
	if limiter == nil {
		return RunResult{}, fmt.Errorf("limiter factory returned nil")
	}
	defer limiter.Stop()

	startedAt := runner.clock.Now().UTC()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan simulationTask, config.Workers*2)
	results := make(chan taskResult, config.Workers*2)
	schedulerDone := make(chan struct{})

	source := &runnerRandom{random: rand.New(rand.NewSource(config.Seed))}
	go runner.schedule(runCtx, config, limiter, source, jobs, results, schedulerDone)

	var workers sync.WaitGroup
	for range config.Workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for task := range jobs {
				results <- runner.executeTask(runCtx, config.ExperimentID, config.SimulationRunID, task)
			}
		}()
	}
	go func() {
		<-schedulerDone
		workers.Wait()
		close(results)
	}()

	completionProgress := Progress{}
	successful := make([]taskResult, 0, config.MaxDecisions)
	var runErr error
	for result := range results {
		completionProgress.AttemptCount++
		if result.err != nil {
			if !errors.Is(result.err, context.Canceled) && !errors.Is(result.err, context.DeadlineExceeded) {
				completionProgress.ErrorCount++
			}
		} else {
			completionProgress.DecisionCount++
			completionProgress.OutcomeCount++
			completionProgress.ObservedRewardSum += result.reward
			completionProgress.RandomExpectedRewardSum += result.randomExpected
			completionProgress.OracleExpectedRewardSum += result.oracleExpected
			successful = append(successful, result)
		}
		if completionProgress.ErrorCount >= int64(config.MaxErrors) && runErr == nil {
			runErr = ErrMaxErrors
			cancel()
		}
		if config.OnProgress != nil && (completionProgress.AttemptCount%int64(config.ProgressEvery) == 0) {
			if err := config.OnProgress(runCtx, completionProgress); err != nil && runErr == nil {
				runErr = fmt.Errorf("persist simulation progress: %w", err)
				cancel()
			}
		}
	}

	finalProgress := deterministicProgress(successful, completionProgress.AttemptCount, completionProgress.ErrorCount)
	if config.OnProgress != nil {
		if err := config.OnProgress(ctx, finalProgress); err != nil && runErr == nil {
			runErr = fmt.Errorf("persist final simulation progress: %w", err)
		}
	}
	result := RunResult{Progress: finalProgress, StartedAt: startedAt, CompletedAt: runner.clock.Now().UTC()}
	if result.CompletedAt.Before(result.StartedAt) {
		return result, fmt.Errorf("simulation clock moved backwards")
	}
	if runErr != nil {
		return result, runErr
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func (runner *Runner) schedule(
	ctx context.Context,
	config RunConfig,
	limiter Limiter,
	source *runnerRandom,
	jobs chan<- simulationTask,
	results chan<- taskResult,
	done chan<- struct{},
) {
	defer close(done)
	defer close(jobs)
	for index := 0; index < config.MaxDecisions; index++ {
		if err := limiter.Wait(ctx); err != nil {
			return
		}
		contextValue, err := runner.profile.Context(source)
		if err != nil {
			results <- taskResult{index: index, err: fmt.Errorf("generate context: %w", err)}
			continue
		}
		outcomeDraw, err := source.Float64()
		if err != nil {
			results <- taskResult{index: index, err: fmt.Errorf("generate outcome draw: %w", err)}
			continue
		}
		task := simulationTask{
			index:       index,
			context:     contextValue,
			outcomeDraw: outcomeDraw,
			eventID:     deterministicRunUUID(config.ExperimentID, config.Seed, index, "outcome"),
			requestID:   deterministicRunUUID(config.ExperimentID, config.Seed, index, "request").String(),
		}
		select {
		case jobs <- task:
		case <-ctx.Done():
			return
		}
	}
}

func (runner *Runner) executeTask(ctx context.Context, experimentID uuid.UUID, simulationRunID *uuid.UUID, task simulationTask) (result taskResult) {
	result.index = task.index
	defer func() {
		if recovered := recover(); recovered != nil {
			result = taskResult{index: task.index, err: fmt.Errorf("simulation worker panic type %T", recovered)}
		}
	}()

	decision, err := runner.client.Decide(ctx, experimentID, simulationRunID, task.context, task.requestID)
	if err != nil {
		result.err = fmt.Errorf("request decision: %w", err)
		return result
	}
	if decision.DecisionID == uuid.Nil || decision.SelectedOfferID == uuid.Nil || decision.SelectedOfferSlug == "" || decision.PolicyVersion < 1 {
		result.err = fmt.Errorf("decision response is incomplete")
		return result
	}
	probabilities, err := runner.profile.Probabilities(task.context, decision.SelectedOfferSlug)
	if err != nil {
		result.err = err
		return result
	}
	outcomeKind := outcomeFromDraw(task.outcomeDraw, probabilities)
	reward, err := domain.RewardForOutcome(outcomeKind)
	if err != nil {
		result.err = err
		return result
	}
	occurredAt := runner.clock.Now().UTC()
	outcome, err := runner.client.SubmitOutcome(ctx, task.eventID, decision.DecisionID, outcomeKind, occurredAt, task.requestID)
	if err != nil {
		result.err = fmt.Errorf("submit outcome: %w", err)
		return result
	}
	if outcome.EventID != task.eventID || outcome.DecisionID != decision.DecisionID || outcome.Kind != outcomeKind || outcome.Reward != reward || outcome.AppliedPolicyVersion < 1 {
		result.err = fmt.Errorf("outcome response does not match submitted event")
		return result
	}
	catalog := runner.profile.Catalog()
	offerSlugs := make([]string, len(catalog))
	for index, offer := range catalog {
		offerSlugs[index] = offer.Slug
	}
	randomExpected, err := runner.profile.UniformExpectedReward(task.context, offerSlugs)
	if err != nil {
		result.err = err
		return result
	}
	oracleExpected, err := runner.profile.OracleExpectedReward(task.context, offerSlugs)
	if err != nil {
		result.err = err
		return result
	}
	result.reward = reward
	result.randomExpected = randomExpected
	result.oracleExpected = oracleExpected
	return result
}

func validateRunConfig(config RunConfig) error {
	if config.ExperimentID == uuid.Nil {
		return fmt.Errorf("experiment id must not be nil")
	}
	if config.SimulationRunID != nil && *config.SimulationRunID == uuid.Nil {
		return fmt.Errorf("simulation run id must not be nil UUID")
	}
	if config.RequestsPerSecond < 1 || config.RequestsPerSecond > 100 {
		return fmt.Errorf("requests per second must be between 1 and 100")
	}
	if config.MaxDecisions < 1 || config.MaxDecisions > 100_000 {
		return fmt.Errorf("max decisions must be between 1 and 100000")
	}
	if config.Workers < 1 || config.Workers > 32 {
		return fmt.Errorf("workers must be between 1 and 32")
	}
	if config.MaxErrors < 1 || config.MaxErrors > 100 {
		return fmt.Errorf("max errors must be between 1 and 100")
	}
	if config.ProgressEvery < 1 || config.ProgressEvery > config.MaxDecisions {
		return fmt.Errorf("progress interval must be between 1 and max decisions")
	}
	return nil
}

func deterministicProgress(successful []taskResult, attempts, errorsCount int64) Progress {
	sort.Slice(successful, func(left, right int) bool { return successful[left].index < successful[right].index })
	progress := Progress{AttemptCount: attempts, ErrorCount: errorsCount}
	for _, result := range successful {
		progress.DecisionCount++
		progress.OutcomeCount++
		progress.ObservedRewardSum += result.reward
		progress.RandomExpectedRewardSum += result.randomExpected
		progress.OracleExpectedRewardSum += result.oracleExpected
	}
	return progress
}

func deterministicRunUUID(experimentID uuid.UUID, seed int64, index int, kind string) uuid.UUID {
	return uuid.NewSHA1(experimentID, []byte(fmt.Sprintf("%d:%d:%s", seed, index, kind)))
}

func outcomeFromDraw(draw float64, probabilities OutcomeProbabilities) domain.OutcomeKind {
	if draw < probabilities.Ignored {
		return domain.OutcomeKindIgnored
	}
	if draw < probabilities.Ignored+probabilities.Clicked {
		return domain.OutcomeKindClicked
	}
	return domain.OutcomeKindConverted
}

type simulationTask struct {
	index       int
	context     domain.SessionContext
	outcomeDraw float64
	eventID     uuid.UUID
	requestID   string
}

type taskResult struct {
	index          int
	reward         float64
	randomExpected float64
	oracleExpected float64
	err            error
}

type runnerRandom struct {
	random *rand.Rand
}

func (source *runnerRandom) Float64() (float64, error) {
	if source == nil || source.random == nil {
		return 0, fmt.Errorf("random source is unavailable")
	}
	return source.random.Float64(), nil
}

type realLimiterFactory struct{}

func (realLimiterFactory) New(rate int) (Limiter, error) {
	if rate < 1 {
		return nil, fmt.Errorf("rate must be positive")
	}
	interval := time.Second / time.Duration(rate)
	if interval <= 0 {
		return nil, fmt.Errorf("rate interval is invalid")
	}
	return &tickerLimiter{ticker: time.NewTicker(interval), first: true}, nil
}

type tickerLimiter struct {
	mu     sync.Mutex
	ticker *time.Ticker
	first  bool
}

func (limiter *tickerLimiter) Wait(ctx context.Context) error {
	limiter.mu.Lock()
	if limiter.first {
		limiter.first = false
		limiter.mu.Unlock()
		return nil
	}
	ticker := limiter.ticker
	limiter.mu.Unlock()
	select {
	case <-ticker.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (limiter *tickerLimiter) Stop() {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if limiter.ticker != nil {
		limiter.ticker.Stop()
		limiter.ticker = nil
	}
}
