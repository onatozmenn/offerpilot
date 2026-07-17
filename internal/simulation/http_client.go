package simulation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

const defaultMaxResponseBytes int64 = 1 << 20

type ClientProblemError struct {
	Status    int
	Code      string
	Detail    string
	RequestID string
}

func (problem *ClientProblemError) Error() string {
	return fmt.Sprintf("OfferPilot API problem: status=%d code=%s", problem.Status, problem.Code)
}

type HTTPClient struct {
	baseURL          *url.URL
	client           *http.Client
	maxResponseBytes int64
}

func NewHTTPClient(baseURL string, client *http.Client, maxResponseBytes int64) (*HTTPClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse API base URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("API base URL must be an absolute HTTP or HTTPS origin without credentials or a path")
	}
	if client == nil {
		return nil, fmt.Errorf("HTTP client is required")
	}
	if maxResponseBytes < 1 {
		return nil, fmt.Errorf("max response bytes must be positive")
	}
	parsed.Path = ""
	return &HTTPClient{baseURL: parsed, client: client, maxResponseBytes: maxResponseBytes}, nil
}

func NewDefaultHTTPClient(baseURL string, timeout time.Duration) (*HTTPClient, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("HTTP timeout must be positive")
	}
	return NewHTTPClient(baseURL, &http.Client{Timeout: timeout}, defaultMaxResponseBytes)
}

func (client *HTTPClient) Decide(
	ctx context.Context,
	experimentID uuid.UUID,
	simulationRunID *uuid.UUID,
	contextValue domain.SessionContext,
	requestID string,
) (DecisionResult, error) {
	if simulationRunID != nil {
		return DecisionResult{}, fmt.Errorf("public HTTP decisions cannot carry a simulation run id")
	}
	if experimentID == uuid.Nil {
		return DecisionResult{}, fmt.Errorf("experiment id must not be nil")
	}
	if err := domain.ValidateSessionContext(contextValue); err != nil {
		return DecisionResult{}, err
	}
	if strings.TrimSpace(requestID) == "" {
		return DecisionResult{}, fmt.Errorf("request id is required")
	}
	payload := decisionRequest{
		ExperimentID: experimentID.String(),
		Context: contextPayload{
			DeviceClass:      contextValue.DeviceClass,
			Daypart:          contextValue.Daypart,
			CategoryAffinity: contextValue.CategoryAffinity,
			VisitorType:      contextValue.VisitorType,
		},
	}
	var response decisionResponse
	if err := client.postJSON(ctx, "/v1/decisions", requestID, payload, map[int]bool{http.StatusCreated: true}, &response); err != nil {
		return DecisionResult{}, err
	}
	decisionID, err := uuid.Parse(response.DecisionID)
	if err != nil || decisionID == uuid.Nil {
		return DecisionResult{}, fmt.Errorf("decision response contains invalid decision_id")
	}
	selectedOfferID, err := uuid.Parse(response.SelectedOffer.ID)
	if err != nil || selectedOfferID == uuid.Nil || response.SelectedOffer.Slug == "" {
		return DecisionResult{}, fmt.Errorf("decision response contains invalid selected_offer")
	}
	if math.IsNaN(response.Propensity) || math.IsInf(response.Propensity, 0) || response.Propensity <= 0 || response.Propensity > 1 {
		return DecisionResult{}, fmt.Errorf("decision response contains invalid propensity")
	}
	responseExperimentID, err := uuid.Parse(response.ExperimentID)
	if err != nil || responseExperimentID != experimentID {
		return DecisionResult{}, fmt.Errorf("decision response contains mismatched experiment_id")
	}
	responseContext := domain.SessionContext{
		DeviceClass:      response.Context.DeviceClass,
		Daypart:          response.Context.Daypart,
		CategoryAffinity: response.Context.CategoryAffinity,
		VisitorType:      response.Context.VisitorType,
	}
	if responseContext != contextValue {
		return DecisionResult{}, fmt.Errorf("decision response contains mismatched context")
	}
	eligibleOfferIDs := make([]uuid.UUID, len(response.EligibleOfferIDs))
	for index, rawOfferID := range response.EligibleOfferIDs {
		offerID, err := uuid.Parse(rawOfferID)
		if err != nil || offerID == uuid.Nil {
			return DecisionResult{}, fmt.Errorf("decision response contains invalid eligible_offer_ids")
		}
		eligibleOfferIDs[index] = offerID
	}
	distribution := make([]domain.ActionProbability, len(response.Distribution))
	for index, entry := range response.Distribution {
		offerID, err := uuid.Parse(entry.OfferID)
		if err != nil || offerID == uuid.Nil {
			return DecisionResult{}, fmt.Errorf("decision response contains invalid distribution")
		}
		distribution[index] = domain.ActionProbability{OfferID: offerID, Probability: entry.Probability}
	}
	selectedProbability, err := domain.ValidateDistribution(eligibleOfferIDs, distribution, selectedOfferID)
	if err != nil || selectedProbability != response.Propensity {
		return DecisionResult{}, fmt.Errorf("decision response contains invalid distribution or propensity")
	}
	if response.PolicyKind != domain.PolicyKindRandom && response.PolicyKind != domain.PolicyKindSegmentedEpsilonGreedy {
		return DecisionResult{}, fmt.Errorf("decision response contains invalid policy_kind")
	}
	if response.PolicyVersion < 1 {
		return DecisionResult{}, fmt.Errorf("decision response contains invalid policy_version")
	}
	createdAt, err := parseUTCTimestamp(response.CreatedAt)
	if err != nil {
		return DecisionResult{}, fmt.Errorf("decision response contains invalid created_at")
	}
	return DecisionResult{
		DecisionID:        decisionID,
		SelectedOfferID:   selectedOfferID,
		SelectedOfferSlug: response.SelectedOffer.Slug,
		Propensity:        response.Propensity,
		PolicyKind:        response.PolicyKind,
		PolicyVersion:     response.PolicyVersion,
		CreatedAt:         createdAt,
	}, nil
}

func (client *HTTPClient) SubmitOutcome(
	ctx context.Context,
	eventID uuid.UUID,
	decisionID uuid.UUID,
	kind domain.OutcomeKind,
	occurredAt time.Time,
	requestID string,
) (domain.Outcome, error) {
	if eventID == uuid.Nil || decisionID == uuid.Nil {
		return domain.Outcome{}, fmt.Errorf("event and decision ids must not be nil")
	}
	if _, err := domain.RewardForOutcome(kind); err != nil {
		return domain.Outcome{}, err
	}
	if occurredAt.IsZero() {
		return domain.Outcome{}, fmt.Errorf("occurred_at is required")
	}
	if strings.TrimSpace(requestID) == "" {
		return domain.Outcome{}, fmt.Errorf("request id is required")
	}
	payload := outcomeRequest{
		EventID:    eventID.String(),
		DecisionID: decisionID.String(),
		Outcome:    kind,
		OccurredAt: occurredAt.UTC().Format(time.RFC3339Nano),
	}
	var response outcomeResponse
	if err := client.postJSON(ctx, "/v1/outcomes", requestID, payload, map[int]bool{http.StatusCreated: true, http.StatusOK: true}, &response); err != nil {
		return domain.Outcome{}, err
	}
	responseEventID, err := uuid.Parse(response.EventID)
	if err != nil || responseEventID == uuid.Nil {
		return domain.Outcome{}, fmt.Errorf("outcome response contains invalid event_id")
	}
	responseDecisionID, err := uuid.Parse(response.DecisionID)
	if err != nil || responseDecisionID == uuid.Nil {
		return domain.Outcome{}, fmt.Errorf("outcome response contains invalid decision_id")
	}
	occurred, err := parseUTCTimestamp(response.OccurredAt)
	if err != nil {
		return domain.Outcome{}, fmt.Errorf("outcome response contains invalid occurred_at")
	}
	received, err := parseUTCTimestamp(response.ReceivedAt)
	if err != nil {
		return domain.Outcome{}, fmt.Errorf("outcome response contains invalid received_at")
	}
	expectedReward, err := domain.RewardForOutcome(response.Outcome)
	if err != nil || expectedReward != response.Reward || response.AppliedPolicyVersion < 1 {
		return domain.Outcome{}, fmt.Errorf("outcome response contains invalid reward or applied version")
	}
	return domain.Outcome{
		EventID:              responseEventID,
		DecisionID:           responseDecisionID,
		Kind:                 response.Outcome,
		Reward:               response.Reward,
		OccurredAt:           occurred,
		ReceivedAt:           received,
		AppliedPolicyVersion: response.AppliedPolicyVersion,
	}, nil
}

func (client *HTTPClient) postJSON(
	ctx context.Context,
	path string,
	requestID string,
	payload any,
	allowedStatuses map[int]bool,
	destination any,
) error {
	if path != "/v1/decisions" && path != "/v1/outcomes" {
		return fmt.Errorf("unsupported API path")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	endpoint := client.baseURL.ResolveReference(&url.URL{Path: path})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, application/problem+json")
	request.Header.Set("X-Request-ID", requestID)

	response, err := client.client.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	body, err := readBounded(response.Body, client.maxResponseBytes)
	if err != nil {
		return err
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if !allowedStatuses[response.StatusCode] {
		if mediaErr == nil && mediaType == "application/problem+json" {
			problem, err := decodeProblem(body)
			if err != nil {
				return fmt.Errorf("decode API problem: %w", err)
			}
			problem.Status = response.StatusCode
			return problem
		}
		return fmt.Errorf("unexpected API status %d", response.StatusCode)
	}
	if mediaErr != nil || mediaType != "application/json" {
		return fmt.Errorf("success response must use application/json")
	}
	if err := decodeExactJSON(body, destination); err != nil {
		return fmt.Errorf("decode success response: %w", err)
	}
	return nil
}

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(body)) > maximum {
		return nil, fmt.Errorf("response body exceeds %d bytes", maximum)
	}
	return body, nil
}

func decodeExactJSON(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("response contains multiple JSON values")
	}
	return nil
}

func decodeProblem(body []byte) (*ClientProblemError, error) {
	var payload struct {
		Status    int    `json:"status"`
		Code      string `json:"code"`
		Detail    string `json:"detail"`
		RequestID string `json:"request_id"`
		Type      string `json:"type"`
		Title     string `json:"title"`
	}
	if err := decodeExactJSON(body, &payload); err != nil {
		return nil, err
	}
	if payload.Code == "" || payload.Detail == "" || payload.RequestID == "" {
		return nil, fmt.Errorf("problem response is incomplete")
	}
	return &ClientProblemError{Status: payload.Status, Code: payload.Code, Detail: payload.Detail, RequestID: payload.RequestID}, nil
}

func parseUTCTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	_, offset := parsed.Zone()
	if offset != 0 {
		return time.Time{}, fmt.Errorf("timestamp must be UTC")
	}
	return parsed.UTC(), nil
}

type contextPayload struct {
	DeviceClass      domain.DeviceClass   `json:"device_class"`
	Daypart          domain.Daypart       `json:"daypart"`
	CategoryAffinity domain.OfferCategory `json:"category_affinity"`
	VisitorType      domain.VisitorType   `json:"visitor_type"`
}

type decisionRequest struct {
	ExperimentID string         `json:"experiment_id"`
	Context      contextPayload `json:"context"`
}

type decisionResponse struct {
	DecisionID    string         `json:"decision_id"`
	ExperimentID  string         `json:"experiment_id"`
	Context       contextPayload `json:"context"`
	SelectedOffer struct {
		ID           string               `json:"id"`
		Slug         string               `json:"slug"`
		MerchantName string               `json:"merchant_name"`
		Title        string               `json:"title"`
		Category     domain.OfferCategory `json:"category"`
	} `json:"selected_offer"`
	EligibleOfferIDs []string `json:"eligible_offer_ids"`
	Propensity       float64  `json:"propensity"`
	Distribution     []struct {
		OfferID     string  `json:"offer_id"`
		Probability float64 `json:"probability"`
	} `json:"distribution"`
	PolicyKind          domain.PolicyKind `json:"policy_kind"`
	PolicyVersion       int64             `json:"policy_version"`
	PolicyLatencyMicros int64             `json:"policy_latency_micros"`
	Outcome             *struct {
		EventID              string             `json:"event_id"`
		Outcome              domain.OutcomeKind `json:"outcome"`
		Reward               float64            `json:"reward"`
		OccurredAt           string             `json:"occurred_at"`
		ReceivedAt           string             `json:"received_at"`
		AppliedPolicyVersion int64              `json:"applied_policy_version"`
	} `json:"outcome"`
	CreatedAt string `json:"created_at"`
}

type outcomeRequest struct {
	EventID    string             `json:"event_id"`
	DecisionID string             `json:"decision_id"`
	Outcome    domain.OutcomeKind `json:"outcome"`
	OccurredAt string             `json:"occurred_at"`
}

type outcomeResponse struct {
	EventID              string             `json:"event_id"`
	DecisionID           string             `json:"decision_id"`
	Outcome              domain.OutcomeKind `json:"outcome"`
	Reward               float64            `json:"reward"`
	OccurredAt           string             `json:"occurred_at"`
	ReceivedAt           string             `json:"received_at"`
	AppliedPolicyVersion int64              `json:"applied_policy_version"`
}
