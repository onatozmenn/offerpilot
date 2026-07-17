package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/observability"
	"github.com/onatozmenn/offerpilot/internal/service"
)

type Engine interface {
	ListExperiments(context.Context, *service.ExperimentCursor, int) ([]domain.Experiment, error)
	GetExperimentDetail(context.Context, uuid.UUID) (domain.Experiment, []domain.Offer, error)
	Summary(context.Context, uuid.UUID, int) (service.Summary, error)
	ListDecisionFeed(context.Context, uuid.UUID, *service.DecisionCursor, int) ([]service.DecisionFeedRecord, error)
	Decide(context.Context, service.DecideCommand) (domain.Decision, error)
	RecordOutcome(context.Context, service.RecordOutcomeCommand) (service.RecordOutcomeResult, error)
}

type DemoCreator interface {
	CreateFreshDemo(context.Context, string, domain.PolicyKind, *float64) (domain.Experiment, []domain.Offer, error)
}

type SimulationManager interface {
	Start(context.Context, uuid.UUID, int64, int, int) (domain.SimulationRun, error)
	Get(context.Context, uuid.UUID) (domain.SimulationRun, error)
	Stop(context.Context, uuid.UUID) (domain.SimulationRun, error)
}

type Handlers struct {
	engine        Engine
	demos         DemoCreator
	simulations   SimulationManager
	logger        *slog.Logger
	maxFutureSkew time.Duration
	now           func() time.Time
}

func NewHandlers(
	engine Engine,
	demos DemoCreator,
	simulations SimulationManager,
	logger *slog.Logger,
	maxFutureSkew time.Duration,
) (*Handlers, error) {
	if engine == nil {
		return nil, fmt.Errorf("engine is required")
	}
	if demos == nil {
		return nil, fmt.Errorf("demo creator is required")
	}
	if simulations == nil {
		return nil, fmt.Errorf("simulation manager is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if maxFutureSkew < 0 {
		return nil, fmt.Errorf("max future skew must not be negative")
	}
	return &Handlers{
		engine:        engine,
		demos:         demos,
		simulations:   simulations,
		logger:        logger,
		maxFutureSkew: maxFutureSkew,
		now:           time.Now,
	}, nil
}

func (handlers *Handlers) CreateDemoExperiment(writer http.ResponseWriter, request *http.Request) {
	var body createDemoExperimentRequest
	if err := decodeJSON(request, &body); err != nil {
		handlers.writeError(writer, request, "create_demo_experiment", err)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		handlers.writeError(writer, request, "create_demo_experiment", newRequestError(http.StatusBadRequest, codeMissingField, "Missing field", "name is required."))
		return
	}
	if err := domain.ValidateEpsilon(body.PolicyKind, body.Epsilon); err != nil {
		handlers.writeError(writer, request, "create_demo_experiment", newRequestError(http.StatusUnprocessableEntity, codeInvalidEnum, "Invalid policy configuration", "policy_kind and epsilon are inconsistent."))
		return
	}
	experiment, offers, err := handlers.demos.CreateFreshDemo(request.Context(), body.Name, body.PolicyKind, cloneFloat(body.Epsilon))
	if err != nil {
		handlers.writeError(writer, request, "create_demo_experiment", err)
		return
	}
	writer.Header().Set("Location", "/v1/experiments/"+experiment.ID.String())
	handlers.writeResponse(writer, request, http.StatusCreated, newExperimentDetailDTO(experiment, offers), "create_demo_experiment")
}

func (handlers *Handlers) ListExperiments(writer http.ResponseWriter, request *http.Request) {
	limit, err := parseLimit(request.URL.Query().Get("limit"), 50, 100)
	if err != nil {
		handlers.writeError(writer, request, "list_experiments", err)
		return
	}
	cursor, err := parseExperimentCursor(request.URL.Query().Get("cursor"))
	if err != nil {
		handlers.writeError(writer, request, "list_experiments", err)
		return
	}
	experiments, err := handlers.engine.ListExperiments(request.Context(), cursor, limit)
	if err != nil {
		handlers.writeError(writer, request, "list_experiments", err)
		return
	}
	page := experimentPageDTO{Items: make([]experimentDTO, len(experiments))}
	for index, experiment := range experiments {
		page.Items[index] = newExperimentDTO(experiment)
	}
	if len(experiments) == limit && len(experiments) > 0 {
		encoded, err := encodeCursor(experiments[len(experiments)-1].CreatedAt, experiments[len(experiments)-1].ID)
		if err != nil {
			handlers.writeError(writer, request, "list_experiments", err)
			return
		}
		page.NextCursor = &encoded
	}
	handlers.writeResponse(writer, request, http.StatusOK, page, "list_experiments")
}

func (handlers *Handlers) GetExperiment(writer http.ResponseWriter, request *http.Request) {
	experimentID, err := parseUUID(chi.URLParam(request, "experiment_id"), "experiment_id", http.StatusBadRequest)
	if err != nil {
		handlers.writeError(writer, request, "get_experiment", err)
		return
	}
	experiment, offers, err := handlers.engine.GetExperimentDetail(request.Context(), experimentID)
	if err != nil {
		handlers.writeError(writer, request, "get_experiment", err)
		return
	}
	handlers.writeResponse(writer, request, http.StatusOK, newExperimentDetailDTO(experiment, offers), "get_experiment")
}

func (handlers *Handlers) GetExperimentSummary(writer http.ResponseWriter, request *http.Request) {
	experimentID, err := parseUUID(chi.URLParam(request, "experiment_id"), "experiment_id", http.StatusBadRequest)
	if err != nil {
		handlers.writeError(writer, request, "get_experiment_summary", err)
		return
	}
	maxPoints, err := parseLimit(request.URL.Query().Get("max_learning_points"), service.DefaultMaxLearningSeriesPoints, service.DefaultMaxLearningSeriesPoints)
	if err != nil {
		handlers.writeError(writer, request, "get_experiment_summary", err)
		return
	}
	summary, err := handlers.engine.Summary(request.Context(), experimentID, maxPoints)
	if err != nil {
		handlers.writeError(writer, request, "get_experiment_summary", err)
		return
	}
	handlers.writeResponse(writer, request, http.StatusOK, newSummaryDTO(summary), "get_experiment_summary")
}

func (handlers *Handlers) ListExperimentDecisions(writer http.ResponseWriter, request *http.Request) {
	experimentID, err := parseUUID(chi.URLParam(request, "experiment_id"), "experiment_id", http.StatusBadRequest)
	if err != nil {
		handlers.writeError(writer, request, "list_experiment_decisions", err)
		return
	}
	limit, err := parseLimit(request.URL.Query().Get("limit"), 50, 200)
	if err != nil {
		handlers.writeError(writer, request, "list_experiment_decisions", err)
		return
	}
	cursor, err := parseDecisionCursor(request.URL.Query().Get("cursor"))
	if err != nil {
		handlers.writeError(writer, request, "list_experiment_decisions", err)
		return
	}
	records, err := handlers.engine.ListDecisionFeed(request.Context(), experimentID, cursor, limit)
	if err != nil {
		handlers.writeError(writer, request, "list_experiment_decisions", err)
		return
	}
	page := decisionPageDTO{Items: make([]decisionDTO, len(records))}
	for index, record := range records {
		page.Items[index] = newDecisionDTO(record.Decision, record.SelectedOffer, record.Outcome)
	}
	if len(records) == limit && len(records) > 0 {
		last := records[len(records)-1].Decision
		encoded, err := encodeCursor(last.CreatedAt, last.ID)
		if err != nil {
			handlers.writeError(writer, request, "list_experiment_decisions", err)
			return
		}
		page.NextCursor = &encoded
	}
	handlers.writeResponse(writer, request, http.StatusOK, page, "list_experiment_decisions")
}

func (handlers *Handlers) CreateDecision(writer http.ResponseWriter, request *http.Request) {
	var body createDecisionRequest
	if err := decodeJSON(request, &body); err != nil {
		handlers.writeError(writer, request, "create_decision", err)
		return
	}
	experimentID, err := parseUUID(body.ExperimentID, "experiment_id", http.StatusUnprocessableEntity)
	if err != nil {
		handlers.writeError(writer, request, "create_decision", err)
		return
	}
	contextValue := body.Context.domain()
	if err := domain.ValidateSessionContext(contextValue); err != nil {
		handlers.writeError(writer, request, "create_decision", newRequestError(http.StatusUnprocessableEntity, codeInvalidContext, "Invalid session context", "context contains an unsupported or missing value."))
		return
	}
	decision, err := handlers.engine.Decide(request.Context(), service.DecideCommand{
		ExperimentID: experimentID,
		Context:      contextValue,
		RequestID:    requestIDFromContext(request.Context()),
	})
	if err != nil {
		handlers.writeError(writer, request, "create_decision", err)
		return
	}
	_, offers, err := handlers.engine.GetExperimentDetail(request.Context(), experimentID)
	if err != nil {
		handlers.writeError(writer, request, "create_decision", err)
		return
	}
	selectedOffer, found := findOffer(offers, decision.SelectedOfferID)
	if !found {
		handlers.writeError(writer, request, "create_decision", fmt.Errorf("persisted selected offer projection is unavailable"))
		return
	}
	writer.Header().Set("Location", "/v1/experiments/"+experimentID.String()+"/decisions")
	handlers.writeResponse(writer, request, http.StatusCreated, newDecisionDTO(decision, selectedOffer, nil), "create_decision")
}

func (handlers *Handlers) CreateOutcome(writer http.ResponseWriter, request *http.Request) {
	var body createOutcomeRequest
	if err := decodeJSON(request, &body); err != nil {
		handlers.writeError(writer, request, "create_outcome", err)
		return
	}
	eventID, err := parseUUID(body.EventID, "event_id", http.StatusUnprocessableEntity)
	if err != nil {
		handlers.writeError(writer, request, "create_outcome", err)
		return
	}
	decisionID, err := parseUUID(body.DecisionID, "decision_id", http.StatusUnprocessableEntity)
	if err != nil {
		handlers.writeError(writer, request, "create_outcome", err)
		return
	}
	if _, err := domain.RewardForOutcome(body.Outcome); err != nil {
		handlers.writeError(writer, request, "create_outcome", newRequestError(http.StatusUnprocessableEntity, codeInvalidEnum, "Invalid outcome", "outcome must be ignored, clicked, or converted."))
		return
	}
	occurredAt, err := parseTimestamp(body.OccurredAt, "occurred_at")
	if err != nil {
		handlers.writeError(writer, request, "create_outcome", err)
		return
	}
	if occurredAt.After(handlers.now().UTC().Add(handlers.maxFutureSkew)) {
		handlers.writeError(writer, request, "create_outcome", newRequestError(http.StatusUnprocessableEntity, codeInvalidTimestamp, "Invalid timestamp", "occurred_at exceeds the allowed future clock skew."))
		return
	}
	result, err := handlers.engine.RecordOutcome(request.Context(), service.RecordOutcomeCommand{
		EventID:    eventID,
		DecisionID: decisionID,
		Kind:       body.Outcome,
		OccurredAt: occurredAt,
	})
	if err != nil {
		handlers.writeError(writer, request, "create_outcome", err)
		return
	}
	status := http.StatusCreated
	if result.ExactRetry {
		status = http.StatusOK
	} else {
		writer.Header().Set("Location", "/v1/outcomes")
	}
	handlers.writeResponse(writer, request, status, newOutcomeDTO(result.Outcome), "create_outcome")
}

func (handlers *Handlers) CreateSimulationRun(writer http.ResponseWriter, request *http.Request) {
	experimentID, err := parseUUID(chi.URLParam(request, "experiment_id"), "experiment_id", http.StatusBadRequest)
	if err != nil {
		handlers.writeError(writer, request, "create_simulation_run", err)
		return
	}
	var body createSimulationRunRequest
	if err := decodeJSON(request, &body); err != nil {
		handlers.writeError(writer, request, "create_simulation_run", err)
		return
	}
	if body.RequestsPerSecond < 1 || body.RequestsPerSecond > 100 || body.MaxDecisions < 1 || body.MaxDecisions > 100_000 {
		handlers.writeError(writer, request, "create_simulation_run", newRequestError(http.StatusUnprocessableEntity, codeInvalidQuery, "Invalid simulation configuration", "simulation rate and decision count must be within documented limits."))
		return
	}
	run, err := handlers.simulations.Start(request.Context(), experimentID, body.Seed, body.RequestsPerSecond, body.MaxDecisions)
	if err != nil {
		handlers.writeError(writer, request, "create_simulation_run", err)
		return
	}
	writer.Header().Set("Location", "/v1/simulation-runs/"+run.ID.String())
	handlers.writeResponse(writer, request, http.StatusCreated, newSimulationRunDTO(run), "create_simulation_run")
}

func (handlers *Handlers) GetSimulationRun(writer http.ResponseWriter, request *http.Request) {
	runID, err := parseUUID(chi.URLParam(request, "run_id"), "run_id", http.StatusBadRequest)
	if err != nil {
		handlers.writeError(writer, request, "get_simulation_run", err)
		return
	}
	run, err := handlers.simulations.Get(request.Context(), runID)
	if err != nil {
		handlers.writeError(writer, request, "get_simulation_run", err)
		return
	}
	handlers.writeResponse(writer, request, http.StatusOK, newSimulationRunDTO(run), "get_simulation_run")
}

func (handlers *Handlers) StopSimulationRun(writer http.ResponseWriter, request *http.Request) {
	runID, err := parseUUID(chi.URLParam(request, "run_id"), "run_id", http.StatusBadRequest)
	if err != nil {
		handlers.writeError(writer, request, "stop_simulation_run", err)
		return
	}
	run, err := handlers.simulations.Stop(request.Context(), runID)
	if err != nil {
		handlers.writeError(writer, request, "stop_simulation_run", err)
		return
	}
	handlers.writeResponse(writer, request, http.StatusAccepted, newSimulationRunDTO(run), "stop_simulation_run")
}

func (handlers *Handlers) writeError(writer http.ResponseWriter, request *http.Request, operation string, err error) {
	writeMappedError(writer, request, handlers.logger, operation, err)
}

func (handlers *Handlers) writeResponse(writer http.ResponseWriter, request *http.Request, status int, value any, operation string) {
	if err := writeJSON(writer, status, value); err != nil {
		observability.LogInternalError(handlers.logger, requestIDFromContext(request.Context()), operation, err)
	}
}

func decodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return newRequestError(http.StatusRequestEntityTooLarge, codeRequestTooLarge, "Request too large", "The request body exceeds the configured limit.")
		}
		if errors.Is(err, io.EOF) {
			return newRequestError(http.StatusBadRequest, codeMalformedJSON, "Malformed JSON", "The request body must contain one JSON object.")
		}
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			return newRequestError(http.StatusBadRequest, codeUnknownField, "Unknown field", "The request contains an unknown field.")
		}
		return newRequestError(http.StatusBadRequest, codeMalformedJSON, "Malformed JSON", "The request body is not a valid JSON object.")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return newRequestError(http.StatusBadRequest, codeMultipleJSONValues, "Multiple JSON values", "The request body must contain exactly one JSON object.")
	}
	return nil
}

func parseUUID(value, field string, status int) (uuid.UUID, error) {
	if strings.TrimSpace(value) == "" {
		return uuid.Nil, newRequestError(http.StatusBadRequest, codeMissingField, "Missing field", field+" is required.")
	}
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, newRequestError(status, codeInvalidUUID, "Invalid UUID", field+" must be a non-nil UUID.")
	}
	return parsed, nil
}

func parseTimestamp(value, field string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, newRequestError(http.StatusBadRequest, codeMissingField, "Missing field", field+" is required.")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, newRequestError(http.StatusUnprocessableEntity, codeInvalidTimestamp, "Invalid timestamp", field+" must be an RFC 3339 timestamp.")
	}
	_, offset := parsed.Zone()
	if offset != 0 {
		return time.Time{}, newRequestError(http.StatusUnprocessableEntity, codeInvalidTimestamp, "Invalid timestamp", field+" must use UTC.")
	}
	return parsed.UTC(), nil
}

func parseLimit(raw string, fallback, maximum int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximum {
		return 0, newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid query", fmt.Sprintf("limit must be between 1 and %d.", maximum))
	}
	return value, nil
}

type cursorPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func encodeCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	payload, err := json.Marshal(cursorPayload{CreatedAt: formatTimestamp(createdAt), ID: id.String()})
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(raw string) (time.Time, uuid.UUID, error) {
	if raw == "" {
		return time.Time{}, uuid.Nil, nil
	}
	if len(raw) > 512 {
		return time.Time{}, uuid.Nil, newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid cursor", "cursor is invalid.")
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid cursor", "cursor is invalid.")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var decoded cursorPayload
	if err := decoder.Decode(&decoded); err != nil {
		return time.Time{}, uuid.Nil, newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid cursor", "cursor is invalid.")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return time.Time{}, uuid.Nil, newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid cursor", "cursor is invalid.")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, decoded.CreatedAt)
	if err != nil {
		return time.Time{}, uuid.Nil, newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid cursor", "cursor is invalid.")
	}
	id, err := uuid.Parse(decoded.ID)
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, newRequestError(http.StatusBadRequest, codeInvalidQuery, "Invalid cursor", "cursor is invalid.")
	}
	return createdAt.UTC(), id, nil
}

func parseExperimentCursor(raw string) (*service.ExperimentCursor, error) {
	createdAt, id, err := decodeCursor(raw)
	if err != nil || raw == "" {
		return nil, err
	}
	return &service.ExperimentCursor{CreatedAt: createdAt, ID: id}, nil
}

func parseDecisionCursor(raw string) (*service.DecisionCursor, error) {
	createdAt, id, err := decodeCursor(raw)
	if err != nil || raw == "" {
		return nil, err
	}
	return &service.DecisionCursor{CreatedAt: createdAt, ID: id}, nil
}

func findOffer(offers []domain.Offer, offerID uuid.UUID) (domain.Offer, bool) {
	for _, offer := range offers {
		if offer.ID == offerID {
			return offer, true
		}
	}
	return domain.Offer{}, false
}
