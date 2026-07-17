package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

const redactedValue = "[REDACTED]"

var boundedSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,127}$`)

func NewLogger(writer io.Writer, level, format string) (*slog.Logger, error) {
	if writer == nil {
		return nil, fmt.Errorf("log writer is required")
	}
	parsedLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	options := &slog.HandlerOptions{Level: parsedLevel}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "text":
		handler = slog.NewTextHandler(writer, options)
	case "json":
		handler = slog.NewJSONHandler(writer, options)
	default:
		return nil, fmt.Errorf("log format must be text or json")
	}

	return slog.New(redactingHandler{next: handler}), nil
}

func LogInternalError(logger *slog.Logger, requestID, operation string, err error) {
	if logger == nil {
		return
	}
	logger.Error(
		"internal operation failed",
		slog.String("request_id", requestID),
		slog.String("operation", operation),
		slog.Any("error", err),
	)
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log level must be debug, info, warn, or error")
	}
}

type redactingHandler struct {
	next slog.Handler
}

func (handler redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return handler.next.Enabled(ctx, level)
}

func (handler redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	sanitized := slog.NewRecord(record.Time, record.Level, sanitizeMessage(record.Message), record.PC)
	record.Attrs(func(attribute slog.Attr) bool {
		sanitized.AddAttrs(sanitizeAttribute(attribute))
		return true
	})
	return handler.next.Handle(ctx, sanitized)
}

func (handler redactingHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attributes))
	for index, attribute := range attributes {
		sanitized[index] = sanitizeAttribute(attribute)
	}
	return redactingHandler{next: handler.next.WithAttrs(sanitized)}
}

func (handler redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{next: handler.next.WithGroup(name)}
}

func sanitizeAttribute(attribute slog.Attr) slog.Attr {
	attribute.Value = attribute.Value.Resolve()
	if sensitiveKey(attribute.Key) {
		return slog.String(attribute.Key, redactedValue)
	}
	if attribute.Value.Kind() == slog.KindAny {
		if err, ok := attribute.Value.Any().(error); ok {
			return slog.String(attribute.Key, fmt.Sprintf("%T", err))
		}
	}
	if attribute.Value.Kind() == slog.KindGroup {
		group := attribute.Value.Group()
		for index := range group {
			group[index] = sanitizeAttribute(group[index])
		}
		return slog.Group(attribute.Key, attrsToAny(group)...)
	}
	return attribute
}

func attrsToAny(attributes []slog.Attr) []any {
	values := make([]any, len(attributes))
	for index, attribute := range attributes {
		values[index] = attribute
	}
	return values
}

func sensitiveKey(key string) bool {
	normalized := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(key))
	for _, fragment := range []string{
		"authorization",
		"cookie",
		"database_url",
		"password",
		"secret",
		"token",
		"request_body",
		"response_body",
		"context",
		"distribution",
		"environment",
		"snapshot",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func sanitizeMessage(message string) string {
	normalized := strings.ToLower(message)
	for _, marker := range []string{"postgres://", "postgresql://", "password=", "authorization:", "bearer "} {
		if strings.Contains(normalized, marker) {
			return "redacted log message"
		}
	}
	return message
}

type Metrics struct {
	httpRequests             *prometheus.CounterVec
	httpDuration             *prometheus.HistogramVec
	decisions                *prometheus.CounterVec
	outcomes                 *prometheus.CounterVec
	reward                   *prometheus.CounterVec
	policyVersion            *prometheus.GaugeVec
	policyUpdates            *prometheus.CounterVec
	policyUpdateDuration     *prometheus.HistogramVec
	simulationActive         *prometheus.GaugeVec
	simulationEvents         *prometheus.CounterVec
	storageOperations        *prometheus.CounterVec
	storageOperationDuration *prometheus.HistogramVec
	recoveryReplayed         *prometheus.CounterVec
}

func NewMetrics(registerer prometheus.Registerer) (*Metrics, error) {
	if registerer == nil {
		return nil, fmt.Errorf("prometheus registerer is required")
	}

	metrics := &Metrics{
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests by stable route, method, and status.",
		}, []string{"route", "method", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "offerpilot",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration by stable route and method.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"route", "method"}),
		decisions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "decisions_total",
			Help:      "Persisted decisions by bounded experiment, policy, and offer slugs.",
		}, []string{"experiment", "policy", "offer"}),
		outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "outcomes_total",
			Help:      "Accepted outcomes by bounded experiment slug and outcome kind.",
		}, []string{"experiment", "outcome"}),
		reward: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "reward_total",
			Help:      "Server-derived reward by bounded experiment slug and policy kind.",
		}, []string{"experiment", "policy"}),
		policyVersion: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "offerpilot",
			Name:      "policy_version",
			Help:      "Current applied policy version by bounded experiment slug.",
		}, []string{"experiment"}),
		policyUpdates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "policy_updates_total",
			Help:      "Policy update attempts by bounded experiment slug and stable result.",
		}, []string{"experiment", "result"}),
		policyUpdateDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "offerpilot",
			Name:      "policy_update_duration_seconds",
			Help:      "Policy update duration by bounded experiment slug.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"experiment"}),
		simulationActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "offerpilot",
			Name:      "simulation_active",
			Help:      "Whether an in-process simulation is active by bounded experiment slug.",
		}, []string{"experiment"}),
		simulationEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "simulation_events_total",
			Help:      "Simulation events by bounded experiment slug and stable event type.",
		}, []string{"experiment", "type"}),
		storageOperations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "storage_operations_total",
			Help:      "Storage operations by stable operation and result.",
		}, []string{"operation", "result"}),
		storageOperationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "offerpilot",
			Name:      "storage_operation_duration_seconds",
			Help:      "Storage operation duration by stable operation.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"operation"}),
		recoveryReplayed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "offerpilot",
			Name:      "recovery_replayed_outcomes_total",
			Help:      "Outcomes replayed during recovery by bounded experiment slug.",
		}, []string{"experiment"}),
	}

	collectors := []prometheus.Collector{
		metrics.httpRequests,
		metrics.httpDuration,
		metrics.decisions,
		metrics.outcomes,
		metrics.reward,
		metrics.policyVersion,
		metrics.policyUpdates,
		metrics.policyUpdateDuration,
		metrics.simulationActive,
		metrics.simulationEvents,
		metrics.storageOperations,
		metrics.storageOperationDuration,
		metrics.recoveryReplayed,
	}
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("register OfferPilot metric: %w", err)
		}
	}

	return metrics, nil
}

func (metrics *Metrics) ObserveHTTPRequest(route, method string, status int, duration time.Duration) error {
	if metrics == nil {
		return fmt.Errorf("metrics are required")
	}
	if !allowedHTTPRoutes[route] {
		return fmt.Errorf("HTTP route label is not allowed")
	}
	method = strings.ToUpper(method)
	if !allowedHTTPMethods[method] {
		return fmt.Errorf("HTTP method label is not allowed")
	}
	if status < 100 || status > 599 {
		return fmt.Errorf("HTTP status label is invalid")
	}
	if duration < 0 {
		return fmt.Errorf("HTTP duration must not be negative")
	}
	statusLabel := strconv.Itoa(status)
	metrics.httpRequests.WithLabelValues(route, method, statusLabel).Inc()
	metrics.httpDuration.WithLabelValues(route, method).Observe(duration.Seconds())
	return nil
}

func (metrics *Metrics) ObserveDecision(experiment, policy, offer string) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	if err := validateBoundedSlug("offer", offer); err != nil {
		return err
	}
	if !allowedPolicies[policy] {
		return fmt.Errorf("policy label is not allowed")
	}
	metrics.decisions.WithLabelValues(experiment, policy, offer).Inc()
	return nil
}

func (metrics *Metrics) ObserveOutcome(experiment, outcome string) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	if !allowedOutcomes[outcome] {
		return fmt.Errorf("outcome label is not allowed")
	}
	metrics.outcomes.WithLabelValues(experiment, outcome).Inc()
	return nil
}

func (metrics *Metrics) AddReward(experiment, policy string, reward float64) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	if !allowedPolicies[policy] {
		return fmt.Errorf("policy label is not allowed")
	}
	if math.IsNaN(reward) || math.IsInf(reward, 0) || reward < 0 || reward > 1 {
		return fmt.Errorf("reward must be finite and between zero and one")
	}
	metrics.reward.WithLabelValues(experiment, policy).Add(reward)
	return nil
}

func (metrics *Metrics) SetPolicyVersion(experiment string, version int64) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	if version < 1 {
		return fmt.Errorf("policy version must be positive")
	}
	metrics.policyVersion.WithLabelValues(experiment).Set(float64(version))
	return nil
}

func (metrics *Metrics) ObservePolicyUpdate(experiment, result string, duration time.Duration) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	if !allowedPolicyResults[result] {
		return fmt.Errorf("policy update result label is not allowed")
	}
	if duration < 0 {
		return fmt.Errorf("policy update duration must not be negative")
	}
	metrics.policyUpdates.WithLabelValues(experiment, result).Inc()
	metrics.policyUpdateDuration.WithLabelValues(experiment).Observe(duration.Seconds())
	return nil
}

func (metrics *Metrics) SetSimulationActive(experiment string, active bool) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	value := 0.0
	if active {
		value = 1
	}
	metrics.simulationActive.WithLabelValues(experiment).Set(value)
	return nil
}

func (metrics *Metrics) ObserveSimulationEvent(experiment, eventType string) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	if !allowedSimulationEvents[eventType] {
		return fmt.Errorf("simulation event label is not allowed")
	}
	metrics.simulationEvents.WithLabelValues(experiment, eventType).Inc()
	return nil
}

func (metrics *Metrics) ObserveStorageOperation(operation, result string, duration time.Duration) error {
	if !allowedStorageOperations[operation] {
		return fmt.Errorf("storage operation label is not allowed")
	}
	if !allowedStorageResults[result] {
		return fmt.Errorf("storage result label is not allowed")
	}
	if duration < 0 {
		return fmt.Errorf("storage duration must not be negative")
	}
	metrics.storageOperations.WithLabelValues(operation, result).Inc()
	metrics.storageOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
	return nil
}

func (metrics *Metrics) AddRecoveryReplayedOutcomes(experiment string, count int) error {
	if err := validateBoundedSlug("experiment", experiment); err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("replayed outcome count must not be negative")
	}
	metrics.recoveryReplayed.WithLabelValues(experiment).Add(float64(count))
	return nil
}

func validateBoundedSlug(label, value string) error {
	if !boundedSlugPattern.MatchString(value) {
		return fmt.Errorf("%s label must be a bounded lowercase slug", label)
	}
	if _, err := uuid.Parse(value); err == nil {
		return fmt.Errorf("%s label must not be a UUID", label)
	}
	return nil
}

var allowedHTTPRoutes = map[string]bool{
	"/health/live":                                    true,
	"/health/ready":                                   true,
	"/metrics":                                        true,
	"/v1/demo/experiments":                            true,
	"/v1/experiments":                                 true,
	"/v1/experiments/{experiment_id}":                 true,
	"/v1/experiments/{experiment_id}/summary":         true,
	"/v1/experiments/{experiment_id}/decisions":       true,
	"/v1/decisions":                                   true,
	"/v1/outcomes":                                    true,
	"/v1/experiments/{experiment_id}/simulation-runs": true,
	"/v1/simulation-runs/{run_id}":                    true,
	"/v1/simulation-runs/{run_id}/stop":               true,
	"unmatched":                                       true,
}

var allowedHTTPMethods = map[string]bool{"GET": true, "POST": true, "OPTIONS": true}

var allowedPolicies = map[string]bool{
	string(domain.PolicyKindRandom):                 true,
	string(domain.PolicyKindSegmentedEpsilonGreedy): true,
}

var allowedOutcomes = map[string]bool{
	string(domain.OutcomeKindIgnored):   true,
	string(domain.OutcomeKindClicked):   true,
	string(domain.OutcomeKindConverted): true,
}

var allowedPolicyResults = map[string]bool{
	"applied":     true,
	"exact_retry": true,
	"conflict":    true,
	"failed":      true,
}

var allowedSimulationEvents = map[string]bool{
	"decision": true,
	"outcome":  true,
	"error":    true,
}

var allowedStorageOperations = map[string]bool{
	"open":              true,
	"ping":              true,
	"migrate":           true,
	"create_experiment": true,
	"get_experiment":    true,
	"list_experiments":  true,
	"list_offers":       true,
	"insert_decision":   true,
	"get_decision":      true,
	"list_decisions":    true,
	"accept_outcome":    true,
	"save_snapshot":     true,
	"load_snapshot":     true,
	"recovery":          true,
	"summary":           true,
	"simulation_create": true,
	"simulation_get":    true,
	"simulation_update": true,
}

var allowedStorageResults = map[string]bool{
	"success":   true,
	"error":     true,
	"not_found": true,
	"conflict":  true,
}
