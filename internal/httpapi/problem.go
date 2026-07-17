package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/observability"
	"github.com/onatozmenn/offerpilot/internal/service"
)

const (
	codeMalformedJSON            = "malformed_json"
	codeUnknownField             = "unknown_field"
	codeMultipleJSONValues       = "multiple_json_values"
	codeMissingField             = "missing_field"
	codeInvalidEnum              = "invalid_enum"
	codeInvalidUUID              = "invalid_uuid"
	codeInvalidTimestamp         = "invalid_timestamp"
	codeInvalidContext           = "invalid_context"
	codeInvalidQuery             = "invalid_query"
	codeUnsupportedMediaType     = "unsupported_media_type"
	codeRequestTooLarge          = "request_too_large"
	codeNotFound                 = "not_found"
	codeExperimentNotRunning     = "experiment_not_running"
	codeInsufficientOffers       = "insufficient_offers"
	codeOutcomeAlreadyRecorded   = "outcome_already_recorded"
	codeSimulationAlreadyRunning = "simulation_already_running"
	codePolicyUnhealthy          = "policy_unhealthy"
	codeDatabaseUnavailable      = "database_unavailable"
	codeRequestTimeout           = "request_timeout"
	codeInternalError            = "internal_error"
)

type problemDTO struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	RequestID string `json:"request_id"`
}

type requestError struct {
	status int
	code   string
	title  string
	detail string
}

func (requestError requestError) Error() string {
	return requestError.detail
}

func newRequestError(status int, code, title, detail string) error {
	return requestError{status: status, code: code, title: title, detail: detail}
}

func writeMappedError(writer http.ResponseWriter, request *http.Request, logger *slog.Logger, operation string, err error) {
	problem, internal := mapProblem(err, requestIDFromContext(request.Context()))
	if internal {
		observability.LogInternalError(logger, problem.RequestID, operation, err)
	}
	writeProblem(writer, problem)
}

func mapProblem(err error, requestID string) (problemDTO, bool) {
	var invalidRequest requestError
	if errors.As(err, &invalidRequest) {
		return newProblem(invalidRequest.status, invalidRequest.code, invalidRequest.title, invalidRequest.detail, requestID), false
	}

	switch {
	case errors.Is(err, service.ErrNotFound):
		return newProblem(http.StatusNotFound, codeNotFound, "Resource not found", "The requested resource does not exist.", requestID), false
	case errors.Is(err, service.ErrOutcomeConflict):
		return newProblem(http.StatusConflict, codeOutcomeAlreadyRecorded, "Outcome already recorded", "The decision already has a different terminal outcome.", requestID), false
	case errors.Is(err, service.ErrExperimentNotRunning):
		return newProblem(http.StatusConflict, codeExperimentNotRunning, "Experiment is not running", "Only running experiments accept this operation.", requestID), false
	case errors.Is(err, service.ErrInsufficientOffers):
		return newProblem(http.StatusConflict, codeInsufficientOffers, "Insufficient active offers", "At least two active offers are required.", requestID), false
	case errors.Is(err, service.ErrSimulationConflict):
		return newProblem(http.StatusConflict, codeSimulationAlreadyRunning, "Simulation already running", "Only one active simulation is allowed per experiment.", requestID), false
	case errors.Is(err, service.ErrPolicyUnhealthy):
		return newProblem(http.StatusServiceUnavailable, codePolicyUnhealthy, "Policy unavailable", "The experiment policy is not ready.", requestID), true
	case errors.Is(err, context.DeadlineExceeded):
		return newProblem(http.StatusGatewayTimeout, codeRequestTimeout, "Request timed out", "The operation did not complete before its deadline.", requestID), false
	case errors.Is(err, context.Canceled):
		return newProblem(http.StatusRequestTimeout, codeRequestTimeout, "Request cancelled", "The request was cancelled before completion.", requestID), false
	default:
		return newProblem(http.StatusInternalServerError, codeInternalError, "Internal server error", "The server could not complete the request.", requestID), true
	}
}

func newProblem(status int, code, title, detail, requestID string) problemDTO {
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return problemDTO{
		Type:      "https://offerpilot.local/problems/" + code,
		Title:     title,
		Status:    status,
		Code:      code,
		Detail:    detail,
		RequestID: requestID,
	}
}

func writeProblem(writer http.ResponseWriter, problem problemDTO) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.Header().Set("X-Request-ID", problem.RequestID)
	writer.WriteHeader(problem.Status)
	_ = json.NewEncoder(writer).Encode(problem)
}

func writeJSON(writer http.ResponseWriter, status int, value any) error {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return fmt.Errorf("encode JSON response: %w", err)
	}
	return nil
}

type requestIDContextKey struct{}

func contextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}
