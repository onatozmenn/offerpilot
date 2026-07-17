package simulation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/service"
)

func TestProfile_CatalogProbabilitiesAndBenchmarks(t *testing.T) {
	profile := DefaultProfile()
	if profile.Version() != ProfileVersion {
		t.Fatalf("Version() = %d", profile.Version())
	}
	catalog := profile.Catalog()
	if len(catalog) != 6 {
		t.Fatalf("catalog size = %d, want 6", len(catalog))
	}
	slugs := make(map[string]struct{}, len(catalog))
	categories := make(map[domain.OfferCategory]struct{}, len(catalog))
	for _, offer := range catalog {
		if offer.Slug == "" || offer.MerchantName == "" || offer.Title == "" {
			t.Fatalf("incomplete catalog offer: %#v", offer)
		}
		if _, exists := slugs[offer.Slug]; exists {
			t.Fatalf("duplicate offer slug %q", offer.Slug)
		}
		slugs[offer.Slug] = struct{}{}
		categories[offer.Category] = struct{}{}
	}
	if len(categories) != 6 {
		t.Fatalf("catalog categories = %v", categories)
	}

	offerSlugs := make([]string, len(catalog))
	for index, offer := range catalog {
		offerSlugs[index] = offer.Slug
	}
	contexts := allProfileContexts()
	for _, contextValue := range contexts {
		uniform, err := profile.UniformExpectedReward(contextValue, offerSlugs)
		if err != nil {
			t.Fatalf("UniformExpectedReward(%#v): %v", contextValue, err)
		}
		oracle, err := profile.OracleExpectedReward(contextValue, offerSlugs)
		if err != nil {
			t.Fatalf("OracleExpectedReward(%#v): %v", contextValue, err)
		}
		if uniform < 0 || uniform > 1 || oracle < uniform || oracle > 1 {
			t.Fatalf("benchmark bounds for %#v: uniform=%v oracle=%v", contextValue, uniform, oracle)
		}
		for _, offerSlug := range offerSlugs {
			probabilities, err := profile.Probabilities(contextValue, offerSlug)
			if err != nil {
				t.Fatalf("Probabilities(%q): %v", offerSlug, err)
			}
			if err := validateOutcomeProbabilities(probabilities); err != nil {
				t.Fatalf("invalid probabilities for %q / %#v: %v", offerSlug, contextValue, err)
			}
			expected, err := profile.ExpectedReward(contextValue, offerSlug)
			if err != nil || expected < 0 || expected > 1 || math.IsNaN(expected) || math.IsInf(expected, 0) {
				t.Fatalf("ExpectedReward(%q) = %v, error=%v", offerSlug, expected, err)
			}
		}
	}
	if _, err := profile.Probabilities(testSimulationContext(), "unknown-offer"); err == nil {
		t.Fatal("Probabilities(unknown) error = nil")
	}
	if _, err := profile.UniformExpectedReward(testSimulationContext(), []string{offerSlugs[0], offerSlugs[0]}); err == nil {
		t.Fatal("UniformExpectedReward(duplicate) error = nil")
	}
}

func TestProfile_DeterministicGenerationAndDraws(t *testing.T) {
	profile := DefaultProfile()
	first := &runnerRandom{random: rand.New(rand.NewSource(20260717))}
	second := &runnerRandom{random: rand.New(rand.NewSource(20260717))}
	different := &runnerRandom{random: rand.New(rand.NewSource(20260718))}
	firstContexts := make([]domain.SessionContext, 100)
	secondContexts := make([]domain.SessionContext, 100)
	differentContexts := make([]domain.SessionContext, 100)
	for index := range firstContexts {
		var err error
		firstContexts[index], err = profile.Context(first)
		if err != nil {
			t.Fatalf("first Context(%d): %v", index, err)
		}
		secondContexts[index], err = profile.Context(second)
		if err != nil {
			t.Fatalf("second Context(%d): %v", index, err)
		}
		differentContexts[index], err = profile.Context(different)
		if err != nil {
			t.Fatalf("different Context(%d): %v", index, err)
		}
	}
	if !reflect.DeepEqual(firstContexts, secondContexts) {
		t.Fatal("identically seeded profile contexts diverged")
	}
	if reflect.DeepEqual(firstContexts, differentContexts) {
		t.Fatal("different profile seeds produced identical context sequences")
	}

	contextValue := testSimulationContext()
	offerSlug := profile.Catalog()[0].Slug
	probabilities, err := profile.Probabilities(contextValue, offerSlug)
	if err != nil {
		t.Fatalf("Probabilities: %v", err)
	}
	for _, test := range []struct {
		draw float64
		want domain.OutcomeKind
	}{
		{draw: 0, want: domain.OutcomeKindIgnored},
		{draw: probabilities.Ignored, want: domain.OutcomeKindClicked},
		{draw: probabilities.Ignored + probabilities.Clicked, want: domain.OutcomeKindConverted},
	} {
		got, err := profile.DrawOutcome(&fixedFloatSource{value: test.draw}, contextValue, offerSlug)
		if err != nil || got != test.want {
			t.Fatalf("DrawOutcome(%v) = %q, error=%v, want %q", test.draw, got, err, test.want)
		}
	}
	if _, err := profile.DrawOutcome(&fixedFloatSource{value: 1}, contextValue, offerSlug); err == nil {
		t.Fatal("DrawOutcome(1) error = nil")
	}
}

func TestRunner_DeterminismTotalsAndExactIDs(t *testing.T) {
	firstRunner, firstClient := newTestRunner(t, 4)
	secondRunner, secondClient := newTestRunner(t, 4)
	config := testRunConfig()
	config.OnProgress = func(_ context.Context, progress Progress) error {
		firstClient.mu.Lock()
		firstClient.progress = append(firstClient.progress, progress)
		firstClient.mu.Unlock()
		return nil
	}
	firstResult, err := firstRunner.Run(context.Background(), config)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	secondConfig := config
	secondConfig.OnProgress = nil
	secondResult, err := secondRunner.Run(context.Background(), secondConfig)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if firstResult.Progress != secondResult.Progress {
		t.Fatalf("same-seed progress differs:\n%#v\n%#v", firstResult.Progress, secondResult.Progress)
	}
	if firstResult.DecisionCount != int64(config.MaxDecisions) || firstResult.OutcomeCount != int64(config.MaxDecisions) || firstResult.ErrorCount != 0 || firstResult.ObservedRewardSum < 0 || firstResult.RandomExpectedRewardSum <= 0 || firstResult.OracleExpectedRewardSum < firstResult.RandomExpectedRewardSum {
		t.Fatalf("Run result = %#v", firstResult)
	}
	firstEvents := firstClient.eventIDs()
	secondEvents := secondClient.eventIDs()
	if !reflect.DeepEqual(firstEvents, secondEvents) {
		t.Fatal("same-seed event IDs differ")
	}
	wantEvents := make([]uuid.UUID, config.MaxDecisions)
	for index := 0; index < config.MaxDecisions; index++ {
		wantEvents[index] = deterministicRunUUID(config.ExperimentID, config.Seed, index, "outcome")
	}
	sort.Slice(wantEvents, func(left, right int) bool { return wantEvents[left].String() < wantEvents[right].String() })
	for index, want := range wantEvents {
		if firstEvents[index] != want {
			t.Fatalf("event[%d] = %s, want %s", index, firstEvents[index], want)
		}
	}
	if firstClient.maxConcurrent() > config.Workers || firstClient.maxConcurrent() < 1 {
		t.Fatalf("max concurrent client calls = %d, workers = %d", firstClient.maxConcurrent(), config.Workers)
	}
	if len(firstClient.progress) == 0 {
		t.Fatal("runner did not report progress")
	}

	differentRunner, differentClient := newTestRunner(t, 4)
	differentConfig := config
	differentConfig.Seed++
	if _, err := differentRunner.Run(context.Background(), differentConfig); err != nil {
		t.Fatalf("different-seed Run: %v", err)
	}
	if reflect.DeepEqual(firstClient.contextMultiset(), differentClient.contextMultiset()) {
		t.Fatal("different seeds produced identical context multiset")
	}
}

func TestRunner_BoundsCancellationErrorsAndPanic(t *testing.T) {
	runner, _ := newTestRunner(t, 2)
	base := testRunConfig()
	tests := []struct {
		name   string
		mutate func(*RunConfig)
	}{
		{name: "nil experiment", mutate: func(value *RunConfig) { value.ExperimentID = uuid.Nil }},
		{name: "rate zero", mutate: func(value *RunConfig) { value.RequestsPerSecond = 0 }},
		{name: "rate excessive", mutate: func(value *RunConfig) { value.RequestsPerSecond = 101 }},
		{name: "decisions zero", mutate: func(value *RunConfig) { value.MaxDecisions = 0 }},
		{name: "decisions excessive", mutate: func(value *RunConfig) { value.MaxDecisions = 100_001 }},
		{name: "workers zero", mutate: func(value *RunConfig) { value.Workers = 0 }},
		{name: "workers excessive", mutate: func(value *RunConfig) { value.Workers = 33 }},
		{name: "errors zero", mutate: func(value *RunConfig) { value.MaxErrors = 0 }},
		{name: "progress zero", mutate: func(value *RunConfig) { value.ProgressEvery = 0 }},
		{name: "progress excessive", mutate: func(value *RunConfig) { value.ProgressEvery = value.MaxDecisions + 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			test.mutate(&config)
			if _, err := runner.Run(context.Background(), config); err == nil {
				t.Fatal("Run() error = nil")
			}
		})
	}

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := runner.Run(ctx, base)
		if !errors.Is(err, context.Canceled) || result.DecisionCount != 0 {
			t.Fatalf("Run(cancelled) result=%#v error=%v", result, err)
		}
	})

	t.Run("max errors", func(t *testing.T) {
		errorRunner, client := newTestRunner(t, 4)
		client.decideErr = errors.New("decision unavailable")
		config := base
		config.MaxErrors = 3
		result, err := errorRunner.Run(context.Background(), config)
		if !errors.Is(err, ErrMaxErrors) || result.ErrorCount < 3 || result.OutcomeCount != 0 {
			t.Fatalf("Run(max errors) result=%#v error=%v", result, err)
		}
	})

	t.Run("worker panic", func(t *testing.T) {
		panicRunner, client := newTestRunner(t, 2)
		client.panicDecide = true
		config := base
		config.MaxErrors = 1
		result, err := panicRunner.Run(context.Background(), config)
		if !errors.Is(err, ErrMaxErrors) || result.ErrorCount < 1 {
			t.Fatalf("Run(panic) result=%#v error=%v", result, err)
		}
	})

	t.Run("progress failure", func(t *testing.T) {
		progressRunner, _ := newTestRunner(t, 2)
		config := base
		config.ProgressEvery = 1
		config.OnProgress = func(context.Context, Progress) error { return errors.New("progress persistence failed") }
		if _, err := progressRunner.Run(context.Background(), config); err == nil || !strings.Contains(err.Error(), "progress") {
			t.Fatalf("Run(progress failure) error=%v", err)
		}
	})
}

func TestManager_LifecycleConflictStopAndShutdown(t *testing.T) {
	store := newMemoryRunStore()
	executor := newBlockingExecutor()
	manager := newTestManager(t, store, executor)
	experimentID := testSimulationUUID(10)
	run, err := manager.Start(context.Background(), experimentID, 1, 10, 10)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	awaitSignal(t, executor.started, "runner start")
	runConfig := <-executor.config
	if runConfig.SimulationRunID == nil || *runConfig.SimulationRunID != run.ID {
		t.Fatalf("runner simulation ID = %v, want %s", runConfig.SimulationRunID, run.ID)
	}
	if _, err := manager.Start(context.Background(), experimentID, 2, 10, 10); !errors.Is(err, service.ErrSimulationConflict) {
		t.Fatalf("duplicate Start error=%v", err)
	}
	loaded, err := manager.Get(context.Background(), run.ID)
	if err != nil || loaded.Status != domain.SimulationRunStatusRunning {
		t.Fatalf("Get running=%#v error=%v", loaded, err)
	}
	stopping, err := manager.Stop(context.Background(), run.ID)
	if err != nil || stopping.Status != domain.SimulationRunStatusStopping {
		t.Fatalf("Stop=%#v error=%v", stopping, err)
	}
	if _, err := manager.Stop(context.Background(), run.ID); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	terminal := awaitRunStatus(t, store, domain.SimulationRunStatusCancelled)
	if terminal.ID != run.ID {
		t.Fatalf("terminal run ID=%s, want %s", terminal.ID, run.ID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := manager.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
}

func TestManager_CompletionPanicTerminalFailureAndReconciliation(t *testing.T) {
	t.Run("completed", func(t *testing.T) {
		store := newMemoryRunStore()
		executor := &immediateExecutor{result: RunResult{Progress: Progress{AttemptCount: 2, DecisionCount: 2, OutcomeCount: 2, ObservedRewardSum: 1, RandomExpectedRewardSum: 0.8, OracleExpectedRewardSum: 1.2}}}
		manager := newTestManager(t, store, executor)
		run, err := manager.Start(context.Background(), testSimulationUUID(20), 1, 10, 2)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		terminal := awaitRunStatus(t, store, domain.SimulationRunStatusCompleted)
		if terminal.ID != run.ID || terminal.DecisionCount != 2 || terminal.OutcomeCount != 2 {
			t.Fatalf("completed run=%#v", terminal)
		}
	})

	t.Run("panic", func(t *testing.T) {
		store := newMemoryRunStore()
		manager := newTestManager(t, store, panicExecutor{})
		if _, err := manager.Start(context.Background(), testSimulationUUID(21), 1, 10, 2); err != nil {
			t.Fatalf("Start: %v", err)
		}
		terminal := awaitRunStatus(t, store, domain.SimulationRunStatusFailed)
		if terminal.ErrorCode == nil || *terminal.ErrorCode != "runner_failed" {
			t.Fatalf("panic terminal=%#v", terminal)
		}
	})

	t.Run("terminal persistence failure", func(t *testing.T) {
		store := newMemoryRunStore()
		store.failTerminal.Store(true)
		manager := newTestManager(t, store, &immediateExecutor{result: RunResult{Progress: Progress{DecisionCount: 1, OutcomeCount: 1}}})
		run, err := manager.Start(context.Background(), testSimulationUUID(22), 1, 10, 1)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		awaitSignal(t, store.terminalAttempt, "terminal persistence attempt")
		if err := manager.Ready(context.Background()); !errors.Is(err, ErrManagerUnhealthy) {
			t.Fatalf("Ready error=%v", err)
		}
		loaded, err := manager.Get(context.Background(), run.ID)
		if err != nil || loaded.Status != domain.SimulationRunStatusFailed || loaded.ErrorCode == nil || *loaded.ErrorCode != "terminal_persistence_failed" {
			t.Fatalf("in-memory failed run=%#v error=%v", loaded, err)
		}
		store.failTerminal.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown retry: %v", err)
		}
		if err := manager.Ready(context.Background()); err != nil {
			t.Fatalf("Ready after retry: %v", err)
		}
	})

	t.Run("restart reconciliation", func(t *testing.T) {
		store := newMemoryRunStore()
		active := domain.SimulationRun{ID: testSimulationUUID(30), ExperimentID: testSimulationUUID(31), Seed: 1, RequestsPerSecond: 10, MaxDecisions: 100, Status: domain.SimulationRunStatusRunning, DecisionCount: 7, OutcomeCount: 6, ErrorCount: 1, ObservedRewardSum: 2, RandomExpectedRewardSum: 1.5, OracleExpectedRewardSum: 3, StartedAt: testSimulationTime(), UpdatedAt: testSimulationTime()}
		store.runs[active.ID] = active
		manager := newTestManager(t, store, newBlockingExecutor())
		count, err := manager.RecoverInterrupted(context.Background())
		if err != nil || count != 1 {
			t.Fatalf("RecoverInterrupted count=%d error=%v", count, err)
		}
		reconciled := store.get(active.ID)
		if reconciled.Status != domain.SimulationRunStatusFailed || reconciled.DecisionCount != 7 || reconciled.ErrorCode == nil || *reconciled.ErrorCode != "process_restarted" {
			t.Fatalf("reconciled=%#v", reconciled)
		}
		newRun, err := manager.Start(context.Background(), active.ExperimentID, 2, 10, 10)
		if err != nil {
			t.Fatalf("Start after reconciliation: %v", err)
		}
		if _, err := manager.Stop(context.Background(), newRun.ID); err != nil {
			t.Fatalf("Stop new run: %v", err)
		}
	})
}

func TestManager_EngineClientCarriesInternalRunAttribution(t *testing.T) {
	experimentID := testSimulationUUID(90)
	runID := testSimulationUUID(91)
	offer := testSimulationUUID(92)
	engine := &engineClientFake{
		decision: domain.Decision{ID: testSimulationUUID(93), ExperimentID: experimentID, SelectedOfferID: offer, Propensity: 1, PolicyKind: domain.PolicyKindRandom, PolicyVersion: 1, CreatedAt: testSimulationTime()},
		offers:   []domain.Offer{{ID: offer, ExperimentID: experimentID, Slug: OfferSlugOrbitMeadow, MerchantName: "Orbit Meadow", Title: "Offer", Category: domain.OfferCategoryTravel, Active: true}},
	}
	client, err := NewEngineClient(engine)
	if err != nil {
		t.Fatalf("NewEngineClient: %v", err)
	}
	if _, err := client.Decide(context.Background(), experimentID, &runID, testSimulationContext(), "request-1"); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if engine.command.SimulationRunID == nil || *engine.command.SimulationRunID != runID {
		t.Fatalf("service simulation ID = %v, want %s", engine.command.SimulationRunID, runID)
	}
}

func TestHTTPClient_SuccessNoRetryAndRequestShape(t *testing.T) {
	experimentID := testSimulationUUID(100)
	decisionID := testSimulationUUID(101)
	offerIDs := []uuid.UUID{testSimulationUUID(102), testSimulationUUID(103)}
	sort.Slice(offerIDs, func(left, right int) bool { return offerIDs[left].String() < offerIDs[right].String() })
	eventID := testSimulationUUID(104)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" || request.Header.Get("X-Request-ID") != "request-1" {
			t.Errorf("request method/headers = %s %#v", request.Method, request.Header)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/decisions":
			writer.WriteHeader(http.StatusCreated)
			body, _ := io.ReadAll(request.Body)
			if bytes.Contains(body, []byte("simulation_run_id")) {
				t.Error("public decision request contains simulation_run_id")
			}
			_, _ = fmt.Fprintf(writer, `{
                    "decision_id":%q,
                    "experiment_id":%q,
                    "context":{"device_class":"mobile","daypart":"evening","category_affinity":"travel","visitor_type":"returning"},
                    "selected_offer":{"id":%q,"slug":%q,"merchant_name":"Orbit Meadow","title":"Offer","category":"travel"},
                    "eligible_offer_ids":[%q,%q],
                    "propensity":0.5,
                    "distribution":[{"offer_id":%q,"probability":0.5},{"offer_id":%q,"probability":0.5}],
                    "policy_kind":"random","policy_version":1,"policy_latency_micros":10,"outcome":null,
                    "created_at":"2026-07-17T12:00:00Z"
                }`, decisionID, experimentID, offerIDs[0], OfferSlugOrbitMeadow, offerIDs[0], offerIDs[1], offerIDs[0], offerIDs[1])
		case "/v1/outcomes":
			writer.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(writer, `{"event_id":%q,"decision_id":%q,"outcome":"clicked","reward":0.25,"occurred_at":"2026-07-17T12:00:01Z","received_at":"2026-07-17T12:00:02Z","applied_policy_version":2}`, eventID, decisionID)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewHTTPClient(server.URL, server.Client(), 1<<20)
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	decision, err := client.Decide(context.Background(), experimentID, nil, testSimulationContext(), "request-1")
	if err != nil || decision.DecisionID != decisionID || decision.SelectedOfferSlug != OfferSlugOrbitMeadow || decision.Propensity != 0.5 {
		t.Fatalf("Decide=%#v error=%v", decision, err)
	}
	outcome, err := client.SubmitOutcome(context.Background(), eventID, decisionID, domain.OutcomeKindClicked, testSimulationTime().Add(time.Second), "request-1")
	if err != nil || outcome.EventID != eventID || outcome.AppliedPolicyVersion != 2 {
		t.Fatalf("SubmitOutcome=%#v error=%v", outcome, err)
	}
	if requests.Load() != 2 {
		t.Fatalf("request count=%d, want 2", requests.Load())
	}
}

func TestHTTPClient_ProblemsMalformedOversizedAndCancellation(t *testing.T) {
	t.Run("problem no retry", func(t *testing.T) {
		var requests atomic.Int32
		const canary = "CANARY-RAW-PROBLEM-BODY"
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			writer.Header().Set("Content-Type", "application/problem+json")
			writer.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(writer, `{"type":"https://offerpilot.local/problems/outcome_already_recorded","title":"Conflict","status":409,"code":"outcome_already_recorded","detail":"`+canary+`","request_id":"request-1"}`)
		}))
		t.Cleanup(server.Close)
		client, err := NewHTTPClient(server.URL, server.Client(), 1<<20)
		if err != nil {
			t.Fatalf("NewHTTPClient: %v", err)
		}
		_, err = client.Decide(context.Background(), testSimulationUUID(200), nil, testSimulationContext(), "request-1")
		var problem *ClientProblemError
		if !errors.As(err, &problem) || problem.Status != 409 || problem.Code != "outcome_already_recorded" || strings.Contains(err.Error(), canary) || requests.Load() != 1 {
			t.Fatalf("problem=%#v error=%v requests=%d", problem, err, requests.Load())
		}
	})

	for _, test := range []struct {
		name        string
		contentType string
		body        string
		maxBytes    int64
	}{
		{name: "malformed", contentType: "application/json", body: `{`, maxBytes: 1024},
		{name: "wrong media", contentType: "text/plain", body: `{}`, maxBytes: 1024},
		{name: "oversized", contentType: "application/json", body: strings.Repeat("x", 65), maxBytes: 64},
		{name: "unexpected status", contentType: "text/plain", body: `failure`, maxBytes: 1024},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", test.contentType)
				if test.name == "unexpected status" {
					writer.WriteHeader(http.StatusBadGateway)
				}
				_, _ = io.WriteString(writer, test.body)
			}))
			t.Cleanup(server.Close)
			client, err := NewHTTPClient(server.URL, server.Client(), test.maxBytes)
			if err != nil {
				t.Fatalf("NewHTTPClient: %v", err)
			}
			if _, err := client.Decide(context.Background(), testSimulationUUID(201), nil, testSimulationContext(), "request-1"); err == nil {
				t.Fatal("Decide() error=nil")
			}
		})
	}

	t.Run("cancellation", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			close(started)
			select {
			case <-request.Context().Done():
			case <-release:
			}
		}))
		t.Cleanup(func() {
			close(release)
			server.CloseClientConnections()
			server.Close()
		})
		client, err := NewHTTPClient(server.URL, server.Client(), 1024)
		if err != nil {
			t.Fatalf("NewHTTPClient: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := client.Decide(ctx, testSimulationUUID(202), nil, testSimulationContext(), "request-1")
			done <- err
		}()
		awaitSignal(t, started, "HTTP request start")
		cancel()
		err = awaitError(t, done, "HTTP cancellation")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Decide cancellation error=%v", err)
		}
	})
}

func TestHTTPClient_ValidationAndBodyClose(t *testing.T) {
	for _, baseURL := range []string{"", "localhost:8080", "ftp://localhost", "http://user:secret@localhost", "http://localhost/path"} {
		if _, err := NewHTTPClient(baseURL, http.DefaultClient, 1024); err == nil {
			t.Fatalf("NewHTTPClient(%q) error=nil", baseURL)
		}
	}
	closed := atomic.Bool{}
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusCreated, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: &trackedBody{Reader: strings.NewReader(`{`), closed: &closed}}, nil
	})
	client, err := NewHTTPClient("http://localhost", &http.Client{Transport: transport}, 1024)
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	runID := testSimulationUUID(299)
	if _, err := client.Decide(context.Background(), testSimulationUUID(300), &runID, testSimulationContext(), "request-1"); err == nil {
		t.Fatal("public Decide(internal run ID) error=nil")
	}
	if _, err := client.Decide(context.Background(), testSimulationUUID(300), nil, testSimulationContext(), "request-1"); err == nil {
		t.Fatal("Decide(malformed) error=nil")
	}
	if !closed.Load() {
		t.Fatal("response body was not closed")
	}
}

func newTestRunner(t *testing.T, workers int) (*Runner, *runnerTestClient) {
	t.Helper()
	client := &runnerTestClient{profile: DefaultProfile()}
	clock := &monotonicSimulationClock{current: testSimulationTime(), step: time.Millisecond}
	runner, err := NewRunner(DefaultProfile(), client, LimiterFactoryFunc(func(int) (Limiter, error) { return &immediateLimiter{}, nil }), clock)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	client.workers = workers
	return runner, client
}

func testRunConfig() RunConfig {
	progress := make([]Progress, 0)
	_ = progress
	return RunConfig{ExperimentID: testSimulationUUID(1), Seed: 20260717, RequestsPerSecond: 20, MaxDecisions: 40, Workers: 4, MaxErrors: 10, ProgressEvery: 5}
}

type runnerTestClient struct {
	profile       *Profile
	workers       int
	mu            sync.Mutex
	active        int
	maxActiveSeen int
	decideErr     error
	outcomeErr    error
	panicDecide   bool
	events        []uuid.UUID
	contexts      []string
	progress      []Progress
}

func (client *runnerTestClient) Decide(_ context.Context, experimentID uuid.UUID, _ *uuid.UUID, contextValue domain.SessionContext, requestID string) (DecisionResult, error) {
	client.mu.Lock()
	client.active++
	if client.active > client.maxActiveSeen {
		client.maxActiveSeen = client.active
	}
	encoded, _ := json.Marshal(contextValue)
	client.contexts = append(client.contexts, string(encoded))
	client.mu.Unlock()
	defer func() {
		client.mu.Lock()
		client.active--
		client.mu.Unlock()
	}()
	if client.panicDecide {
		panic("runner client panic")
	}
	if client.decideErr != nil {
		return DecisionResult{}, client.decideErr
	}
	offer := client.profile.Catalog()[0]
	return DecisionResult{DecisionID: uuid.NewSHA1(experimentID, []byte(requestID)), SelectedOfferID: uuid.NewSHA1(experimentID, []byte(offer.Slug)), SelectedOfferSlug: offer.Slug, Propensity: 1.0 / 6, PolicyKind: domain.PolicyKindRandom, PolicyVersion: 1, CreatedAt: testSimulationTime()}, nil
}

func (client *runnerTestClient) SubmitOutcome(_ context.Context, eventID, decisionID uuid.UUID, kind domain.OutcomeKind, occurredAt time.Time, _ string) (domain.Outcome, error) {
	if client.outcomeErr != nil {
		return domain.Outcome{}, client.outcomeErr
	}
	reward, _ := domain.RewardForOutcome(kind)
	client.mu.Lock()
	client.events = append(client.events, eventID)
	client.mu.Unlock()
	return domain.Outcome{EventID: eventID, DecisionID: decisionID, Kind: kind, Reward: reward, OccurredAt: occurredAt, ReceivedAt: occurredAt, AppliedPolicyVersion: 1}, nil
}

func (client *runnerTestClient) eventIDs() []uuid.UUID {
	client.mu.Lock()
	defer client.mu.Unlock()
	result := append([]uuid.UUID(nil), client.events...)
	sort.Slice(result, func(left, right int) bool { return result[left].String() < result[right].String() })
	return result
}

func (client *runnerTestClient) contextMultiset() []string {
	client.mu.Lock()
	defer client.mu.Unlock()
	result := append([]string(nil), client.contexts...)
	sort.Strings(result)
	return result
}

func (client *runnerTestClient) maxConcurrent() int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.maxActiveSeen
}

type immediateLimiter struct{}

func (*immediateLimiter) Wait(ctx context.Context) error {
	return ctx.Err()
}

func (*immediateLimiter) Stop() {}

type monotonicSimulationClock struct {
	mu      sync.Mutex
	current time.Time
	step    time.Duration
}

func (clock *monotonicSimulationClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	value := clock.current
	clock.current = clock.current.Add(clock.step)
	return value
}

type fixedFloatSource struct {
	value float64
	err   error
}

func (source *fixedFloatSource) Float64() (float64, error) {
	return source.value, source.err
}

type memoryRunStore struct {
	mu              sync.Mutex
	runs            map[uuid.UUID]domain.SimulationRun
	updates         chan domain.SimulationRun
	terminalAttempt chan struct{}
	failTerminal    atomic.Bool
}

func newMemoryRunStore() *memoryRunStore {
	return &memoryRunStore{runs: make(map[uuid.UUID]domain.SimulationRun), updates: make(chan domain.SimulationRun, 64), terminalAttempt: make(chan struct{}, 8)}
}

func (store *memoryRunStore) CreateSimulationRun(_ context.Context, run domain.SimulationRun) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, existing := range store.runs {
		if existing.ExperimentID == run.ExperimentID && (existing.Status == domain.SimulationRunStatusStarting || existing.Status == domain.SimulationRunStatusRunning || existing.Status == domain.SimulationRunStatusStopping) {
			return service.ErrSimulationConflict
		}
	}
	store.runs[run.ID] = cloneRun(run)
	return nil
}

func (store *memoryRunStore) GetSimulationRun(_ context.Context, runID uuid.UUID) (domain.SimulationRun, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	run, exists := store.runs[runID]
	if !exists {
		return domain.SimulationRun{}, service.ErrNotFound
	}
	return cloneRun(run), nil
}

func (store *memoryRunStore) UpdateSimulationRun(ctx context.Context, run domain.SimulationRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	terminal := run.Status == domain.SimulationRunStatusCompleted || run.Status == domain.SimulationRunStatusFailed || run.Status == domain.SimulationRunStatusCancelled
	if terminal {
		select {
		case store.terminalAttempt <- struct{}{}:
		default:
		}
		if store.failTerminal.Load() {
			return errors.New("terminal persistence failed")
		}
	}
	store.mu.Lock()
	store.runs[run.ID] = cloneRun(run)
	store.mu.Unlock()
	select {
	case store.updates <- cloneRun(run):
	default:
	}
	return nil
}

func (store *memoryRunStore) ReconcileInterruptedRuns(_ context.Context, now time.Time) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var count int64
	for id, run := range store.runs {
		if run.Status != domain.SimulationRunStatusStarting && run.Status != domain.SimulationRunStatusRunning && run.Status != domain.SimulationRunStatusStopping {
			continue
		}
		code := "process_restarted"
		detail := "API process restarted before the run completed"
		run.Status = domain.SimulationRunStatusFailed
		run.StoppedAt = &now
		run.UpdatedAt = now
		run.ErrorCode = &code
		run.ErrorDetail = &detail
		store.runs[id] = run
		count++
	}
	return count, nil
}

func (store *memoryRunStore) get(id uuid.UUID) domain.SimulationRun {
	store.mu.Lock()
	defer store.mu.Unlock()
	return cloneRun(store.runs[id])
}

type blockingExecutor struct {
	started chan struct{}
	config  chan RunConfig
	release chan struct{}
	once    sync.Once
	result  RunResult
	err     error
}

func newBlockingExecutor() *blockingExecutor {
	return &blockingExecutor{started: make(chan struct{}), config: make(chan RunConfig, 1), release: make(chan struct{})}
}

func (executor *blockingExecutor) Run(ctx context.Context, config RunConfig) (RunResult, error) {
	executor.config <- config
	executor.once.Do(func() { close(executor.started) })
	select {
	case <-executor.release:
		return executor.result, executor.err
	case <-ctx.Done():
		return executor.result, ctx.Err()
	}
}

type immediateExecutor struct {
	result RunResult
	err    error
}

func (executor *immediateExecutor) Run(context.Context, RunConfig) (RunResult, error) {
	return executor.result, executor.err
}

type panicExecutor struct{}

func (panicExecutor) Run(context.Context, RunConfig) (RunResult, error) {
	panic("runner panic")
}

func newTestManager(t *testing.T, store RunStore, executor RunExecutor) *Manager {
	t.Helper()
	clock := &monotonicSimulationClock{current: testSimulationTime(), step: time.Millisecond}
	var next atomic.Int64
	manager, err := NewManager(store, executor, clock, func() uuid.UUID { return testSimulationUUID(int(next.Add(1) + 10_000)) }, ManagerConfig{Workers: 4, MaxErrors: 10, ProgressEvery: 2, PersistTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})
	return manager
}

func awaitRunStatus(t *testing.T, store *memoryRunStore, status domain.SimulationRunStatus) domain.SimulationRun {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case run := <-store.updates:
			if run.Status == status {
				return run
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for run status %q", status)
		}
	}
}

func awaitSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func awaitError(t *testing.T, errorsChannel <-chan error, name string) error {
	t.Helper()
	select {
	case err := <-errorsChannel:
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
		return nil
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type trackedBody struct {
	io.Reader
	closed *atomic.Bool
}

type engineClientFake struct {
	command  service.DecideCommand
	decision domain.Decision
	offers   []domain.Offer
}

func (engine *engineClientFake) Decide(_ context.Context, command service.DecideCommand) (domain.Decision, error) {
	engine.command = command
	return engine.decision, nil
}

func (engine *engineClientFake) RecordOutcome(context.Context, service.RecordOutcomeCommand) (service.RecordOutcomeResult, error) {
	return service.RecordOutcomeResult{}, nil
}

func (engine *engineClientFake) GetExperimentDetail(context.Context, uuid.UUID) (domain.Experiment, []domain.Offer, error) {
	return domain.Experiment{}, append([]domain.Offer(nil), engine.offers...), nil
}

func (body *trackedBody) Close() error {
	body.closed.Store(true)
	return nil
}

func allProfileContexts() []domain.SessionContext {
	var contexts []domain.SessionContext
	for _, device := range []domain.DeviceClass{domain.DeviceClassMobile, domain.DeviceClassDesktop, domain.DeviceClassTablet} {
		for _, daypart := range []domain.Daypart{domain.DaypartMorning, domain.DaypartAfternoon, domain.DaypartEvening, domain.DaypartNight} {
			for _, category := range []domain.OfferCategory{domain.OfferCategoryTravel, domain.OfferCategoryDining, domain.OfferCategoryWellness, domain.OfferCategoryHome, domain.OfferCategoryTechnology, domain.OfferCategoryEntertainment} {
				for _, visitor := range []domain.VisitorType{domain.VisitorTypeNew, domain.VisitorTypeReturning} {
					contexts = append(contexts, domain.SessionContext{DeviceClass: device, Daypart: daypart, CategoryAffinity: category, VisitorType: visitor})
				}
			}
		}
	}
	return contexts
}

func testSimulationContext() domain.SessionContext {
	return domain.SessionContext{DeviceClass: domain.DeviceClassMobile, Daypart: domain.DaypartEvening, CategoryAffinity: domain.OfferCategoryTravel, VisitorType: domain.VisitorTypeReturning}
}

func testSimulationUUID(value int) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("simulation-test-%08d", value)))
}

func testSimulationTime() time.Time {
	return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
}
