package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/config"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/observability"
	"github.com/onatozmenn/offerpilot/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestHandlers_SuccessResponsesMatchOpenAPI(t *testing.T) {
	fixture := newHTTPFixture(t, 100*time.Millisecond)
	document := loadOpenAPI(t)
	experimentID := fixture.engine.experiment.ID.String()
	runID := fixture.simulations.run.ID.String()
	decisionRequest := fmt.Sprintf(`{
        "experiment_id": %q,
        "context": {
            "device_class": "mobile",
            "daypart": "evening",
            "category_affinity": "travel",
            "visitor_type": "returning"
        }
    }`, experimentID)
	outcomeRequest := fmt.Sprintf(`{
        "event_id": %q,
        "decision_id": %q,
        "outcome": "clicked",
        "occurred_at": "2026-07-17T12:00:30Z"
    }`, fixture.engine.outcome.Outcome.EventID, fixture.engine.decision.ID)

	tests := []struct {
		name         string
		method       string
		path         string
		template     string
		body         string
		wantStatus   int
		wantMedia    string
		wantLocation bool
	}{
		{name: "liveness", method: http.MethodGet, path: "/health/live", template: "/health/live", wantStatus: http.StatusOK, wantMedia: "application/json"},
		{name: "readiness", method: http.MethodGet, path: "/health/ready", template: "/health/ready", wantStatus: http.StatusOK, wantMedia: "application/json"},
		{name: "metrics", method: http.MethodGet, path: "/metrics", template: "/metrics", wantStatus: http.StatusOK, wantMedia: "text/plain"},
		{name: "create demo", method: http.MethodPost, path: "/v1/demo/experiments", template: "/v1/demo/experiments", body: `{"name":"Demo","policy_kind":"random"}`, wantStatus: http.StatusCreated, wantMedia: "application/json", wantLocation: true},
		{name: "list experiments", method: http.MethodGet, path: "/v1/experiments?limit=1", template: "/v1/experiments", wantStatus: http.StatusOK, wantMedia: "application/json"},
		{name: "get experiment", method: http.MethodGet, path: "/v1/experiments/" + experimentID, template: "/v1/experiments/{experiment_id}", wantStatus: http.StatusOK, wantMedia: "application/json"},
		{name: "get summary", method: http.MethodGet, path: "/v1/experiments/" + experimentID + "/summary?max_learning_points=2", template: "/v1/experiments/{experiment_id}/summary", wantStatus: http.StatusOK, wantMedia: "application/json"},
		{name: "list decisions", method: http.MethodGet, path: "/v1/experiments/" + experimentID + "/decisions?limit=1", template: "/v1/experiments/{experiment_id}/decisions", wantStatus: http.StatusOK, wantMedia: "application/json"},
		{name: "create decision", method: http.MethodPost, path: "/v1/decisions", template: "/v1/decisions", body: decisionRequest, wantStatus: http.StatusCreated, wantMedia: "application/json", wantLocation: true},
		{name: "create outcome", method: http.MethodPost, path: "/v1/outcomes", template: "/v1/outcomes", body: outcomeRequest, wantStatus: http.StatusCreated, wantMedia: "application/json", wantLocation: true},
		{name: "start simulation", method: http.MethodPost, path: "/v1/experiments/" + experimentID + "/simulation-runs", template: "/v1/experiments/{experiment_id}/simulation-runs", body: `{"seed":20260717,"requests_per_second":20,"max_decisions":100}`, wantStatus: http.StatusCreated, wantMedia: "application/json", wantLocation: true},
		{name: "get simulation", method: http.MethodGet, path: "/v1/simulation-runs/" + runID, template: "/v1/simulation-runs/{run_id}", wantStatus: http.StatusOK, wantMedia: "application/json"},
		{name: "stop simulation", method: http.MethodPost, path: "/v1/simulation-runs/" + runID + "/stop", template: "/v1/simulation-runs/{run_id}/stop", wantStatus: http.StatusAccepted, wantMedia: "application/json"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("X-Request-ID", "client-request-1")
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			fixture.router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			if response.Header().Get("X-Request-ID") != "client-request-1" {
				t.Fatalf("X-Request-ID = %q", response.Header().Get("X-Request-ID"))
			}
			mediaType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
			if err != nil || mediaType != test.wantMedia {
				t.Fatalf("Content-Type = %q, parsed = %q, error = %v", response.Header().Get("Content-Type"), mediaType, err)
			}
			if test.wantLocation && response.Header().Get("Location") == "" {
				t.Fatal("Location header is empty")
			}
			validateOpenAPIResponse(t, document, test.template, test.method, response.Code, mediaType, response.Body.Bytes())
		})
	}
}

func TestHandlers_DecoderAndInputFailures(t *testing.T) {
	fixture := newHTTPFixture(t, 100*time.Millisecond)
	experimentID := fixture.engine.experiment.ID.String()
	validContext := `"context":{"device_class":"mobile","daypart":"evening","category_affinity":"travel","visitor_type":"returning"}`
	tests := []struct {
		name        string
		path        string
		contentType string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{name: "empty", path: "/v1/decisions", contentType: "application/json", wantStatus: http.StatusBadRequest, wantCode: codeMalformedJSON},
		{name: "malformed", path: "/v1/decisions", contentType: "application/json", body: `{`, wantStatus: http.StatusBadRequest, wantCode: codeMalformedJSON},
		{name: "multiple", path: "/v1/decisions", contentType: "application/json", body: fmt.Sprintf(`{"experiment_id":%q,%s}{}`, experimentID, validContext), wantStatus: http.StatusBadRequest, wantCode: codeMultipleJSONValues},
		{name: "unknown top-level", path: "/v1/decisions", contentType: "application/json", body: fmt.Sprintf(`{"experiment_id":%q,%s,"simulation_run_id":%q}`, experimentID, validContext, testUUID(999)), wantStatus: http.StatusBadRequest, wantCode: codeUnknownField},
		{name: "protected nested field", path: "/v1/decisions", contentType: "application/json", body: fmt.Sprintf(`{"experiment_id":%q,"context":{"device_class":"mobile","daypart":"evening","category_affinity":"travel","visitor_type":"returning","age":42}}`, experimentID), wantStatus: http.StatusBadRequest, wantCode: codeUnknownField},
		{name: "wrong content type", path: "/v1/decisions", contentType: "text/plain", body: `{}`, wantStatus: http.StatusUnsupportedMediaType, wantCode: codeUnsupportedMediaType},
		{name: "invalid body UUID", path: "/v1/decisions", contentType: "application/json", body: fmt.Sprintf(`{"experiment_id":"invalid",%s}`, validContext), wantStatus: http.StatusUnprocessableEntity, wantCode: codeInvalidUUID},
		{name: "invalid context", path: "/v1/decisions", contentType: "application/json", body: fmt.Sprintf(`{"experiment_id":%q,"context":{"device_class":"watch","daypart":"evening","category_affinity":"travel","visitor_type":"returning"}}`, experimentID), wantStatus: http.StatusUnprocessableEntity, wantCode: codeInvalidContext},
		{name: "invalid simulation bounds", path: "/v1/experiments/" + experimentID + "/simulation-runs", contentType: "application/json", body: `{"seed":1,"requests_per_second":101,"max_decisions":1}`, wantStatus: http.StatusUnprocessableEntity, wantCode: codeInvalidQuery},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			fixture.router.ServeHTTP(response, request)
			assertProblem(t, response, test.wantStatus, test.wantCode)
		})
	}

	t.Run("oversized", func(t *testing.T) {
		body := `{"name":"` + strings.Repeat("x", (1<<20)+1) + `","policy_kind":"random"}`
		request := httptest.NewRequest(http.MethodPost, "/v1/demo/experiments", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		assertProblem(t, response, http.StatusRequestEntityTooLarge, codeRequestTooLarge)
	})
}

func TestHandlers_PathQueryTimestampAndRetry(t *testing.T) {
	fixture := newHTTPFixture(t, 100*time.Millisecond)
	experimentID := fixture.engine.experiment.ID.String()
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "invalid path UUID", method: http.MethodGet, path: "/v1/experiments/not-a-uuid", wantStatus: http.StatusBadRequest, wantCode: codeInvalidUUID},
		{name: "invalid limit", method: http.MethodGet, path: "/v1/experiments?limit=0", wantStatus: http.StatusBadRequest, wantCode: codeInvalidQuery},
		{name: "invalid cursor", method: http.MethodGet, path: "/v1/experiments?cursor=not-base64!", wantStatus: http.StatusBadRequest, wantCode: codeInvalidQuery},
		{name: "non UTC timestamp", method: http.MethodPost, path: "/v1/outcomes", body: fmt.Sprintf(`{"event_id":%q,"decision_id":%q,"outcome":"clicked","occurred_at":"2026-07-17T15:00:00+03:00"}`, testUUID(700), fixture.engine.decision.ID), wantStatus: http.StatusUnprocessableEntity, wantCode: codeInvalidTimestamp},
		{name: "future timestamp", method: http.MethodPost, path: "/v1/outcomes", body: fmt.Sprintf(`{"event_id":%q,"decision_id":%q,"outcome":"clicked","occurred_at":"2026-07-17T12:03:00Z"}`, testUUID(701), fixture.engine.decision.ID), wantStatus: http.StatusUnprocessableEntity, wantCode: codeInvalidTimestamp},
		{name: "missing route", method: http.MethodGet, path: "/v1/missing", wantStatus: http.StatusNotFound, wantCode: codeNotFound},
		{name: "method not allowed", method: http.MethodDelete, path: "/v1/experiments/" + experimentID, wantStatus: http.StatusMethodNotAllowed, wantCode: codeInvalidQuery},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			fixture.router.ServeHTTP(response, request)
			assertProblem(t, response, test.wantStatus, test.wantCode)
		})
	}

	t.Run("exact retry", func(t *testing.T) {
		fixture.engine.outcome.ExactRetry = true
		body := fmt.Sprintf(`{"event_id":%q,"decision_id":%q,"outcome":"clicked","occurred_at":"2026-07-17T12:00:30Z"}`, fixture.engine.outcome.Outcome.EventID, fixture.engine.decision.ID)
		request := httptest.NewRequest(http.MethodPost, "/v1/outcomes", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Location") != "" {
			t.Fatalf("retry status = %d, location = %q, body = %s", response.Code, response.Header().Get("Location"), response.Body.String())
		}
	})
}

func TestRouter_RequestIDCORSHealthPanicAndRedaction(t *testing.T) {
	fixture := newHTTPFixture(t, 50*time.Millisecond)
	experimentPath := "/v1/experiments/" + fixture.engine.experiment.ID.String()

	t.Run("request ID reuse and generation", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		request.Header.Set("X-Request-ID", "valid-client-id")
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		if response.Header().Get("X-Request-ID") != "valid-client-id" {
			t.Fatalf("reused request ID = %q", response.Header().Get("X-Request-ID"))
		}
		request = httptest.NewRequest(http.MethodGet, "/health/live", nil)
		request.Header.Set("X-Request-ID", "invalid id with spaces")
		response = httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		if _, err := uuid.Parse(response.Header().Get("X-Request-ID")); err != nil {
			t.Fatalf("generated request ID = %q, error = %v", response.Header().Get("X-Request-ID"), err)
		}
	})

	t.Run("CORS exact origin", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, experimentPath, nil)
		request.Header.Set("Origin", "http://localhost:5173")
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" || response.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("allowed CORS headers = %#v", response.Header())
		}
		request = httptest.NewRequest(http.MethodGet, experimentPath, nil)
		request.Header.Set("Origin", "https://forbidden.example")
		response = httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		if response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("forbidden origin was allowed: %#v", response.Header())
		}
		request = httptest.NewRequest(http.MethodOptions, "/v1/decisions", nil)
		request.Header.Set("Origin", "http://localhost:5173")
		response = httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Fatalf("preflight status = %d, headers = %#v", response.Code, response.Header())
		}
	})

	t.Run("readiness failure is redacted", func(t *testing.T) {
		const canary = "CANARY-DATABASE-PASSWORD"
		fixture.readiness.err = errors.New("postgres://user:" + canary + "@localhost/database")
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
		assertProblem(t, response, http.StatusServiceUnavailable, codeDatabaseUnavailable)
		if strings.Contains(response.Body.String(), canary) || strings.Contains(fixture.logs.String(), canary) {
			t.Fatalf("readiness canary leaked; response=%q logs=%q", response.Body.String(), fixture.logs.String())
		}
	})

	t.Run("panic recovery is redacted", func(t *testing.T) {
		const canary = "CANARY-PANIC-BODY"
		fixture.engine.panicList = canary
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/experiments", nil))
		assertProblem(t, response, http.StatusInternalServerError, codeInternalError)
		if strings.Contains(response.Body.String(), canary) || strings.Contains(fixture.logs.String(), canary) {
			t.Fatalf("panic canary leaked; response=%q logs=%q", response.Body.String(), fixture.logs.String())
		}
		fixture.engine.panicList = ""
	})

	t.Run("internal error body and logs are redacted", func(t *testing.T) {
		const canary = "CANARY-INTERNAL-SECRET"
		fixture.engine.listErr = errors.New("postgres://user:" + canary + "@localhost/database")
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/experiments", nil))
		assertProblem(t, response, http.StatusInternalServerError, codeInternalError)
		if strings.Contains(response.Body.String(), canary) || strings.Contains(fixture.logs.String(), canary) || strings.Contains(fixture.logs.String(), "postgres://") {
			t.Fatalf("internal canary leaked; response=%q logs=%q", response.Body.String(), fixture.logs.String())
		}
		fixture.engine.listErr = nil
	})
}

func TestRouter_TimeoutCancellationAndStableMetricRoute(t *testing.T) {
	fixture := newHTTPFixture(t, 20*time.Millisecond)
	fixture.engine.waitForCancellation = true
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/experiments", nil))
	assertProblem(t, response, http.StatusGatewayTimeout, codeRequestTimeout)
	select {
	case <-fixture.engine.cancelled:
	default:
		t.Fatal("request cancellation did not reach engine")
	}

	fixture.engine.waitForCancellation = false
	actualID := fixture.engine.experiment.ID.String()
	response = httptest.NewRecorder()
	fixture.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/experiments/"+actualID, nil))
	families, err := fixture.registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	foundTemplate := false
	for _, family := range families {
		if family.GetName() != "offerpilot_http_requests_total" {
			continue
		}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "route" {
					if strings.Contains(label.GetValue(), actualID) {
						t.Fatalf("metric route contains resource ID: %q", label.GetValue())
					}
					if label.GetValue() == "/v1/experiments/{experiment_id}" {
						foundTemplate = true
					}
				}
			}
		}
	}
	if !foundTemplate {
		t.Fatal("stable experiment route metric was not gathered")
	}
}

func TestProblems_Mapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		internal   bool
	}{
		{name: "request", err: newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid query", "invalid"), wantStatus: http.StatusBadRequest, wantCode: codeInvalidQuery},
		{name: "not found", err: service.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: codeNotFound},
		{name: "outcome conflict", err: service.ErrOutcomeConflict, wantStatus: http.StatusConflict, wantCode: codeOutcomeAlreadyRecorded},
		{name: "not running", err: service.ErrExperimentNotRunning, wantStatus: http.StatusConflict, wantCode: codeExperimentNotRunning},
		{name: "offers", err: service.ErrInsufficientOffers, wantStatus: http.StatusConflict, wantCode: codeInsufficientOffers},
		{name: "simulation", err: service.ErrSimulationConflict, wantStatus: http.StatusConflict, wantCode: codeSimulationAlreadyRunning},
		{name: "unhealthy", err: service.ErrPolicyUnhealthy, wantStatus: http.StatusServiceUnavailable, wantCode: codePolicyUnhealthy, internal: true},
		{name: "deadline", err: context.DeadlineExceeded, wantStatus: http.StatusGatewayTimeout, wantCode: codeRequestTimeout},
		{name: "cancelled", err: context.Canceled, wantStatus: http.StatusRequestTimeout, wantCode: codeRequestTimeout},
		{name: "internal", err: errors.New("database detail"), wantStatus: http.StatusInternalServerError, wantCode: codeInternalError, internal: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			problem, internal := mapProblem(fmt.Errorf("wrapped: %w", test.err), "request-id")
			if problem.Status != test.wantStatus || problem.Code != test.wantCode || problem.RequestID != "request-id" || internal != test.internal {
				t.Fatalf("mapProblem() = %#v, internal=%v", problem, internal)
			}
		})
	}
}

func TestHandlers_DTOCopiesAndServerSettings(t *testing.T) {
	average := 0.5
	reason := "not_simulated"
	summary := service.Summary{
		ExperimentID:     testUUID(800),
		PolicyKind:       domain.PolicyKindRandom,
		PolicyVersion:    1,
		AverageReward:    &average,
		OfferPerformance: []service.OfferPerformance{{Offer: testOffer(testUUID(801), testUUID(800), "offer-copy")}},
		LearningSeries:   []domain.LearningSeriesPoint{{Timestamp: testNow(), SampleCount: 1, CumulativeAverageReward: 0.5}},
		RandomBenchmark:  domain.BenchmarkReference{Kind: domain.BenchmarkKindRandom, Reason: reason, SimulationOnly: true},
		OracleBenchmark:  domain.BenchmarkReference{Kind: domain.BenchmarkKindOracle, Reason: reason, SimulationOnly: true},
		OPE:              service.OPEEstimate{Reason: "no_samples"},
		Reasons:          map[string]string{"average_reward": "no_outcomes"},
		GeneratedAt:      testNow(),
	}
	dto := newSummaryDTO(summary)
	*dto.AverageReward = 0.9
	dto.OfferPerformance[0].Offer.Slug = "changed"
	dto.LearningSeries[0].SampleCount = 99
	dto.Reasons["average_reward"] = "changed"
	if *summary.AverageReward != 0.5 || summary.OfferPerformance[0].Offer.Slug != "offer-copy" || summary.LearningSeries[0].SampleCount != 1 || summary.Reasons["average_reward"] != "no_outcomes" {
		t.Fatalf("DTO mutation changed service projection: %#v", summary)
	}

	httpConfig := testHTTPConfig(100 * time.Millisecond)
	server, err := NewServer("127.0.0.1:8080", http.NotFoundHandler(), httpConfig)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if server.ReadHeaderTimeout != httpConfig.ReadHeaderTimeout || server.ReadTimeout != httpConfig.ReadTimeout || server.WriteTimeout != httpConfig.WriteTimeout || server.IdleTimeout != httpConfig.IdleTimeout {
		t.Fatalf("server timeouts = %#v", server)
	}
}

type httpFixture struct {
	router      http.Handler
	engine      *fakeHTTPEngine
	demos       *fakeDemoCreator
	simulations *fakeSimulationManager
	readiness   *fakeReadiness
	registry    *prometheus.Registry
	logs        *bytes.Buffer
}

func newHTTPFixture(t *testing.T, timeout time.Duration) *httpFixture {
	t.Helper()
	experimentID := testUUID(1)
	offers := []domain.Offer{
		testOffer(testUUID(2), experimentID, "offer-a"),
		testOffer(testUUID(3), experimentID, "offer-b"),
	}
	experiment := domain.Experiment{ID: experimentID, Slug: "experiment-a", Name: "Experiment A", Status: domain.ExperimentStatusRunning, PolicyKind: domain.PolicyKindRandom, PolicyVersion: 1, CreatedAt: testNow(), UpdatedAt: testNow()}
	decision := domain.Decision{
		ID: testUUID(4), ExperimentID: experimentID, SelectedOfferID: offers[0].ID,
		Context: testContext(), SegmentKey: "mobile|evening|travel|returning",
		EligibleOfferIDs: []uuid.UUID{offers[0].ID, offers[1].ID},
		Distribution:     []domain.ActionProbability{{OfferID: offers[0].ID, Probability: 0.5}, {OfferID: offers[1].ID, Probability: 0.5}},
		Propensity:       0.5, PolicyKind: domain.PolicyKindRandom, PolicyVersion: 1,
		PolicyLatencyMicros: 42, RequestID: "request-1", CreatedAt: testNow().Add(time.Minute),
	}
	outcome := domain.Outcome{EventID: testUUID(5), DecisionID: decision.ID, Kind: domain.OutcomeKindClicked, Reward: 0.25, OccurredAt: testNow().Add(2 * time.Minute), ReceivedAt: testNow().Add(2*time.Minute + time.Second), AppliedPolicyVersion: 2}
	benchmarkReason := "not_simulated"
	summary := service.Summary{
		ExperimentID: experimentID, PolicyKind: domain.PolicyKindRandom, PolicyVersion: 1,
		DecisionCount: 1, OutcomeCount: 1, RewardSum: 0.25,
		AverageReward: floatPointer(0.25), ClickedCount: 1,
		P50PolicyLatencyMicros: int64Pointer(42), P95PolicyLatencyMicros: int64Pointer(42),
		OfferPerformance: []service.OfferPerformance{{Offer: offers[0], SelectionCount: 1, OutcomeCount: 1, ClickedCount: 1, RewardSum: 0.25, EmpiricalMean: floatPointer(0.25), CurrentProbability: floatPointer(0.5)}},
		LearningSeries:   []domain.LearningSeriesPoint{{Timestamp: outcome.ReceivedAt, SampleCount: 1, CumulativeAverageReward: 0.25}},
		RandomBenchmark:  domain.BenchmarkReference{Kind: domain.BenchmarkKindRandom, Reason: benchmarkReason, SimulationOnly: true},
		OracleBenchmark:  domain.BenchmarkReference{Kind: domain.BenchmarkKindOracle, Reason: benchmarkReason, SimulationOnly: true},
		OPE:              service.OPEEstimate{SampleCount: 0, Reason: "no_samples"},
		Reasons:          map[string]string{"random_benchmark": benchmarkReason, "oracle_benchmark": benchmarkReason, "ope": "no_samples"},
		GeneratedAt:      testNow().Add(3 * time.Minute),
	}
	run := domain.SimulationRun{ID: testUUID(6), ExperimentID: experimentID, Seed: 20260717, RequestsPerSecond: 20, MaxDecisions: 100, Status: domain.SimulationRunStatusRunning, StartedAt: testNow(), UpdatedAt: testNow()}
	engine := &fakeHTTPEngine{
		experiment: experiment,
		offers:     offers,
		decision:   decision,
		feed:       []service.DecisionFeedRecord{{Decision: decision, SelectedOffer: offers[0], Outcome: &outcome}},
		summary:    summary,
		outcome:    service.RecordOutcomeResult{Outcome: outcome, Created: true, PolicyUpdated: true, SnapshotSaved: true},
		cancelled:  make(chan struct{}),
	}
	demos := &fakeDemoCreator{experiment: experiment, offers: offers}
	simulations := &fakeSimulationManager{run: run}
	readiness := &fakeReadiness{}
	logs := &bytes.Buffer{}
	logger, err := observability.NewLogger(logs, "debug", "json")
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	registry := prometheus.NewRegistry()
	metrics, err := observability.NewMetrics(registry)
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	handlers, err := NewHandlers(engine, demos, simulations, logger, 2*time.Minute)
	if err != nil {
		t.Fatalf("NewHandlers() error = %v", err)
	}
	handlers.now = testNow
	router, err := NewRouter(
		handlers,
		logger,
		metrics,
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		readiness,
		testHTTPConfig(timeout),
		config.CORSConfig{AllowedOrigins: []string{"http://localhost:5173"}},
	)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return &httpFixture{router: router, engine: engine, demos: demos, simulations: simulations, readiness: readiness, registry: registry, logs: logs}
}

func testHTTPConfig(timeout time.Duration) config.HTTPConfig {
	return config.HTTPConfig{ReadHeaderTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: timeout, IdleTimeout: time.Second, MaxBodyBytes: 1 << 20, OutcomeMaxFutureSkew: 2 * time.Minute}
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	mediaType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
	if err != nil || mediaType != "application/problem+json" {
		t.Fatalf("problem Content-Type = %q, error = %v", response.Header().Get("Content-Type"), err)
	}
	var problem problemDTO
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v; body = %s", err, response.Body.String())
	}
	if problem.Status != status || problem.Code != code || problem.RequestID == "" || response.Header().Get("X-Request-ID") != problem.RequestID {
		t.Fatalf("problem = %#v, header request ID = %q", problem, response.Header().Get("X-Request-ID"))
	}
}

func loadOpenAPI(t *testing.T) *openapi3.T {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(filename), "..", "..", "openapi", "openapi.yaml")
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile(%q): %v", path, err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("OpenAPI Validate(): %v", err)
	}
	return document
}

func validateOpenAPIResponse(t *testing.T, document *openapi3.T, path, method string, status int, mediaType string, body []byte) {
	t.Helper()
	pathItem := document.Paths.Find(path)
	if pathItem == nil {
		t.Fatalf("OpenAPI path %q not found", path)
	}
	operation := pathItem.GetOperation(method)
	if operation == nil {
		t.Fatalf("OpenAPI operation %s %s not found", method, path)
	}
	response := operation.Responses.Value(strconv.Itoa(status))
	if response == nil || response.Value == nil {
		t.Fatalf("OpenAPI response %d for %s %s not found", status, method, path)
	}
	media := response.Value.Content.Get(mediaType)
	if media == nil || media.Schema == nil || media.Schema.Value == nil {
		t.Fatalf("OpenAPI media %q for %s %s %d not found", mediaType, method, path, status)
	}
	var value any
	if strings.HasSuffix(mediaType, "/json") || strings.Contains(mediaType, "+json") {
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatalf("decode JSON response: %v; body = %s", err, string(body))
		}
	} else {
		value = string(body)
	}
	if err := media.Schema.Value.VisitJSON(value); err != nil {
		t.Fatalf("response does not match OpenAPI schema for %s %s %d: %v; body = %s", method, path, status, err, string(body))
	}
}

type fakeHTTPEngine struct {
	mu                  sync.Mutex
	experiment          domain.Experiment
	offers              []domain.Offer
	decision            domain.Decision
	feed                []service.DecisionFeedRecord
	summary             service.Summary
	outcome             service.RecordOutcomeResult
	listErr             error
	panicList           string
	waitForCancellation bool
	cancelled           chan struct{}
	cancelOnce          sync.Once
}

func (engine *fakeHTTPEngine) ListExperiments(ctx context.Context, _ *service.ExperimentCursor, _ int) ([]domain.Experiment, error) {
	if engine.panicList != "" {
		panic(engine.panicList)
	}
	if engine.waitForCancellation {
		<-ctx.Done()
		engine.cancelOnce.Do(func() { close(engine.cancelled) })
		return nil, ctx.Err()
	}
	if engine.listErr != nil {
		return nil, engine.listErr
	}
	return []domain.Experiment{engine.experiment}, nil
}

func (engine *fakeHTTPEngine) GetExperimentDetail(context.Context, uuid.UUID) (domain.Experiment, []domain.Offer, error) {
	return engine.experiment, append([]domain.Offer(nil), engine.offers...), nil
}

func (engine *fakeHTTPEngine) Summary(context.Context, uuid.UUID, int) (service.Summary, error) {
	return engine.summary, nil
}

func (engine *fakeHTTPEngine) ListDecisionFeed(context.Context, uuid.UUID, *service.DecisionCursor, int) ([]service.DecisionFeedRecord, error) {
	return append([]service.DecisionFeedRecord(nil), engine.feed...), nil
}

func (engine *fakeHTTPEngine) Decide(context.Context, service.DecideCommand) (domain.Decision, error) {
	return engine.decision, nil
}

func (engine *fakeHTTPEngine) RecordOutcome(context.Context, service.RecordOutcomeCommand) (service.RecordOutcomeResult, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return engine.outcome, nil
}

type fakeDemoCreator struct {
	experiment domain.Experiment
	offers     []domain.Offer
}

func (creator *fakeDemoCreator) CreateFreshDemo(context.Context, string, domain.PolicyKind, *float64) (domain.Experiment, []domain.Offer, error) {
	return creator.experiment, append([]domain.Offer(nil), creator.offers...), nil
}

type fakeSimulationManager struct {
	run domain.SimulationRun
}

func (manager *fakeSimulationManager) Start(context.Context, uuid.UUID, int64, int, int) (domain.SimulationRun, error) {
	return manager.run, nil
}

func (manager *fakeSimulationManager) Get(context.Context, uuid.UUID) (domain.SimulationRun, error) {
	return manager.run, nil
}

func (manager *fakeSimulationManager) Stop(context.Context, uuid.UUID) (domain.SimulationRun, error) {
	run := manager.run
	run.Status = domain.SimulationRunStatusStopping
	return run, nil
}

type fakeReadiness struct {
	err error
}

func (readiness *fakeReadiness) Ready(context.Context) error {
	return readiness.err
}

func testOffer(id, experimentID uuid.UUID, slug string) domain.Offer {
	return domain.Offer{ID: id, ExperimentID: experimentID, Slug: slug, MerchantName: "Fictional " + slug, Title: "Synthetic offer", Description: "Synthetic fixture.", Category: domain.OfferCategoryTravel, Active: true}
}

func testContext() domain.SessionContext {
	return domain.SessionContext{DeviceClass: domain.DeviceClassMobile, Daypart: domain.DaypartEvening, CategoryAffinity: domain.OfferCategoryTravel, VisitorType: domain.VisitorTypeReturning}
}

func testUUID(value int) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("httpapi-test-%08d", value)))
}

func testNow() time.Time {
	return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
}

func floatPointer(value float64) *float64 { return &value }
func int64Pointer(value int64) *int64     { return &value }

var _ = io.Discard
