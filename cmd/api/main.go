package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/bandit"
	"github.com/onatozmenn/offerpilot/internal/bootstrap"
	"github.com/onatozmenn/offerpilot/internal/config"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/httpapi"
	"github.com/onatozmenn/offerpilot/internal/observability"
	"github.com/onatozmenn/offerpilot/internal/service"
	"github.com/onatozmenn/offerpilot/internal/simulation"
	postgresstore "github.com/onatozmenn/offerpilot/internal/storage/postgres"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "OfferPilot API exited with an error.")
		os.Exit(1)
	}
}

func run(ctx context.Context) (returnErr error) {
	applicationConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger, err := observability.NewLogger(os.Stdout, applicationConfig.Logging.Level, applicationConfig.Logging.Format)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	registry := prometheus.NewRegistry()
	metrics, err := observability.NewMetrics(registry)
	if err != nil {
		return fmt.Errorf("create metrics: %w", err)
	}

	store, err := postgresstore.Open(ctx, applicationConfig.Database)
	if err != nil {
		return fmt.Errorf("open PostgreSQL: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, store.Close())
	}()
	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate PostgreSQL: %w", err)
	}

	engine, err := service.NewEngine(
		store,
		service.PolicyFactoryFunc(newPolicy),
		service.ClockFunc(time.Now),
		service.IDGeneratorFunc(uuid.New),
		applicationConfig.HTTP.OutcomeMaxFutureSkew,
	)
	if err != nil {
		return fmt.Errorf("create service engine: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, engine.Close())
	}()
	recovery, err := engine.Recover(ctx)
	if err != nil {
		return fmt.Errorf("recover policy state: %w", err)
	}
	logger.Info(
		"policy recovery completed",
		slog.Int("experiments", recovery.ExperimentsRecovered),
		slog.Int("outcomes_replayed", recovery.OutcomesReplayed),
		slog.Int64("interrupted_runs", recovery.InterruptedRuns),
	)

	engineClient, err := simulation.NewEngineClient(engine)
	if err != nil {
		return fmt.Errorf("create in-process simulation client: %w", err)
	}
	runner, err := simulation.NewDefaultRunner(simulation.DefaultProfile(), engineClient)
	if err != nil {
		return fmt.Errorf("create simulation runner: %w", err)
	}
	manager, err := simulation.NewManager(
		store,
		runner,
		simulation.SimulationClockFunc(time.Now),
		uuid.New,
		simulation.ManagerConfig{
			Workers:        applicationConfig.Simulation.MaxWorkers,
			MaxErrors:      applicationConfig.Simulation.MaxErrors,
			ProgressEvery:  min(100, applicationConfig.Simulation.MaxDecisions),
			PersistTimeout: applicationConfig.Shutdown.Timeout,
		},
	)
	if err != nil {
		return fmt.Errorf("create simulation manager: %w", err)
	}
	if _, err := manager.RecoverInterrupted(ctx); err != nil {
		return fmt.Errorf("reconcile simulation manager: %w", err)
	}

	demo, err := bootstrap.NewDefaultDemo(engine)
	if err != nil {
		return fmt.Errorf("create demo bootstrap: %w", err)
	}
	if _, _, err := demo.EnsureDemo(ctx); err != nil {
		return fmt.Errorf("ensure demo experiment: %w", err)
	}

	handlers, err := httpapi.NewHandlers(
		engine,
		demo,
		manager,
		logger,
		applicationConfig.HTTP.OutcomeMaxFutureSkew,
	)
	if err != nil {
		return fmt.Errorf("create HTTP handlers: %w", err)
	}
	readiness := apiReadiness{store: store, engine: engine, manager: manager}
	router, err := httpapi.NewRouter(
		handlers,
		logger,
		metrics,
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		readiness,
		applicationConfig.HTTP,
		applicationConfig.CORS,
	)
	if err != nil {
		return fmt.Errorf("create HTTP router: %w", err)
	}
	server, err := httpapi.NewServer(applicationConfig.HTTP.Addr, router, applicationConfig.HTTP)
	if err != nil {
		return fmt.Errorf("create HTTP server: %w", err)
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server starting", slog.String("address", applicationConfig.HTTP.Addr))
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}
		serverErrors <- nil
	}()

	select {
	case err := <-serverErrors:
		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), applicationConfig.Shutdown.Timeout)
	defer cancel()
	if err := manager.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown simulations: %w", err)
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	if err := <-serverErrors; err != nil {
		return fmt.Errorf("serve HTTP during shutdown: %w", err)
	}
	logger.Info("HTTP server stopped")
	return nil
}

func newPolicy(experiment domain.Experiment) (bandit.Policy, error) {
	seed := int64(binary.BigEndian.Uint64(experiment.ID[:8]))
	random := bandit.NewLockedRandom(seed)
	switch experiment.PolicyKind {
	case domain.PolicyKindRandom:
		return bandit.NewRandomPolicy(experiment.ID, 1, random)
	case domain.PolicyKindSegmentedEpsilonGreedy:
		if experiment.Epsilon == nil {
			return nil, fmt.Errorf("adaptive experiment epsilon is required")
		}
		return bandit.NewEpsilonGreedyPolicy(
			experiment.ID,
			*experiment.Epsilon,
			bandit.DefaultPriorCount,
			bandit.DefaultPriorRewardSum,
			1,
			random,
		)
	default:
		return nil, fmt.Errorf("unsupported policy kind %q", experiment.PolicyKind)
	}
}

type apiReadiness struct {
	store   *postgresstore.Store
	engine  *service.Engine
	manager *simulation.Manager
}

func (readiness apiReadiness) Ready(ctx context.Context) error {
	if err := readiness.store.Ping(ctx); err != nil {
		return fmt.Errorf("PostgreSQL readiness: %w", err)
	}
	if err := readiness.engine.Ready(ctx); err != nil {
		return fmt.Errorf("policy readiness: %w", err)
	}
	if err := readiness.manager.Ready(ctx); err != nil {
		return fmt.Errorf("simulation readiness: %w", err)
	}
	return nil
}
