package observability

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestLogger_FormatsLevelsAndStableFields(t *testing.T) {
	t.Run("text info", func(t *testing.T) {
		var output bytes.Buffer
		logger, err := NewLogger(&output, "info", "text")
		if err != nil {
			t.Fatalf("NewLogger() error = %v", err)
		}
		logger.Debug("hidden")
		logger.Info("request complete", slog.String("request_id", "request-1"), slog.Int("status", 201))
		text := output.String()
		if strings.Contains(text, "hidden") || !strings.Contains(text, "request complete") || !strings.Contains(text, "request_id=request-1") || !strings.Contains(text, "status=201") {
			t.Fatalf("text log = %q", text)
		}
	})

	t.Run("json debug", func(t *testing.T) {
		var output bytes.Buffer
		logger, err := NewLogger(&output, "debug", "json")
		if err != nil {
			t.Fatalf("NewLogger() error = %v", err)
		}
		logger.Debug("policy selected", slog.String("policy", "random"), slog.Int64("version", 3))
		var record map[string]any
		if err := json.Unmarshal(output.Bytes(), &record); err != nil {
			t.Fatalf("json.Unmarshal() error = %v; output = %q", err, output.String())
		}
		if record["msg"] != "policy selected" || record["level"] != "DEBUG" || record["policy"] != "random" || record["version"] != float64(3) {
			t.Fatalf("JSON record = %#v", record)
		}
		if _, ok := record["time"]; !ok {
			t.Fatalf("JSON record has no timestamp: %#v", record)
		}
	})

	for _, test := range []struct {
		name   string
		level  string
		format string
	}{
		{name: "invalid level", level: "trace", format: "text"},
		{name: "invalid format", level: "info", format: "yaml"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewLogger(&bytes.Buffer{}, test.level, test.format); err == nil {
				t.Fatal("NewLogger() error = nil")
			}
		})
	}
	if _, err := NewLogger(nil, "info", "text"); err == nil {
		t.Fatal("NewLogger(nil) error = nil")
	}
}

func TestLogger_RedactsSensitiveValuesAndErrors(t *testing.T) {
	const canary = "CANARY-DO-NOT-LOG-7f4c"
	var output bytes.Buffer
	logger, err := NewLogger(&output, "debug", "json")
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	logger = logger.With(
		slog.String("database_url", "postgres://user:"+canary+"@localhost/database"),
		slog.String("stable_code", "database_unavailable"),
	)
	logger.Error(
		"request failed",
		slog.String("request_body", canary),
		slog.String("authorization", "Bearer "+canary),
		slog.Any("error", errors.New("database failed: "+canary)),
		slog.Group("details", slog.String("context", canary), slog.String("operation", "insert_decision")),
	)
	logger.Error("postgres://user:" + canary + "@localhost/database")
	LogInternalError(logger, "request-2", "accept_outcome", errors.New(canary))

	text := output.String()
	if strings.Contains(text, canary) || strings.Contains(text, "postgres://") || strings.Contains(text, "Bearer") {
		t.Fatalf("sensitive value leaked: %q", text)
	}
	if !strings.Contains(text, redactedValue) || !strings.Contains(text, "database_unavailable") || !strings.Contains(text, "*errors.errorString") || !strings.Contains(text, "redacted log message") {
		t.Fatalf("redacted output missing stable fields: %q", text)
	}
}

func TestMetrics_RegistrationGatheringAndLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	if err := observeAllMetrics(metrics); err != nil {
		t.Fatalf("observeAllMetrics() error = %v", err)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	wantLabels := map[string][]string{
		"offerpilot_http_requests_total":                {"method", "route", "status"},
		"offerpilot_http_request_duration_seconds":      {"method", "route"},
		"offerpilot_decisions_total":                    {"experiment", "offer", "policy"},
		"offerpilot_outcomes_total":                     {"experiment", "outcome"},
		"offerpilot_reward_total":                       {"experiment", "policy"},
		"offerpilot_policy_version":                     {"experiment"},
		"offerpilot_policy_updates_total":               {"experiment", "result"},
		"offerpilot_policy_update_duration_seconds":     {"experiment"},
		"offerpilot_simulation_active":                  {"experiment"},
		"offerpilot_simulation_events_total":            {"experiment", "type"},
		"offerpilot_storage_operations_total":           {"operation", "result"},
		"offerpilot_storage_operation_duration_seconds": {"operation"},
		"offerpilot_recovery_replayed_outcomes_total":   {"experiment"},
	}
	if len(families) != len(wantLabels) {
		t.Fatalf("metric family count = %d, want %d", len(families), len(wantLabels))
	}
	for _, family := range families {
		allowed, exists := wantLabels[family.GetName()]
		if !exists {
			t.Errorf("unexpected metric family %q", family.GetName())
			continue
		}
		if len(family.Metric) == 0 {
			t.Errorf("metric family %q has no samples", family.GetName())
			continue
		}
		gotLabels := make([]string, len(family.Metric[0].Label))
		for index, pair := range family.Metric[0].Label {
			gotLabels[index] = pair.GetName()
			for _, forbidden := range []string{"request_id", "decision_id", "segment", "error"} {
				if pair.GetName() == forbidden {
					t.Errorf("metric %q contains forbidden label %q", family.GetName(), forbidden)
				}
			}
		}
		if strings.Join(gotLabels, ",") != strings.Join(allowed, ",") {
			t.Errorf("metric %q labels = %v, want %v", family.GetName(), gotLabels, allowed)
		}
	}
}

func TestMetrics_RejectsUnboundedAndInvalidLabels(t *testing.T) {
	metrics, err := NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	uuidLabel := "7923cfb7-0edc-4f84-a624-68457f45bb38"
	tests := []struct {
		name string
		call func() error
	}{
		{name: "raw route", call: func() error {
			return metrics.ObserveHTTPRequest("/v1/decisions/"+uuidLabel, "POST", 201, time.Millisecond)
		}},
		{name: "method", call: func() error { return metrics.ObserveHTTPRequest("/v1/decisions", "PATCH", 200, time.Millisecond) }},
		{name: "status", call: func() error { return metrics.ObserveHTTPRequest("/v1/decisions", "POST", 999, time.Millisecond) }},
		{name: "negative duration", call: func() error { return metrics.ObserveHTTPRequest("/v1/decisions", "POST", 201, -time.Second) }},
		{name: "experiment UUID", call: func() error { return metrics.ObserveDecision(uuidLabel, "random", "offer-a") }},
		{name: "segment offer", call: func() error { return metrics.ObserveDecision("experiment-a", "random", "mobile|evening") }},
		{name: "policy", call: func() error { return metrics.ObserveDecision("experiment-a", "thompson", "offer-a") }},
		{name: "outcome", call: func() error { return metrics.ObserveOutcome("experiment-a", "opened") }},
		{name: "NaN reward", call: func() error { return metrics.AddReward("experiment-a", "random", math.NaN()) }},
		{name: "version", call: func() error { return metrics.SetPolicyVersion("experiment-a", 0) }},
		{name: "policy result", call: func() error {
			return metrics.ObservePolicyUpdate("experiment-a", "secret-error-text", time.Millisecond)
		}},
		{name: "simulation event", call: func() error { return metrics.ObserveSimulationEvent("experiment-a", uuidLabel) }},
		{name: "storage operation", call: func() error {
			return metrics.ObserveStorageOperation("SELECT * FROM secrets", "success", time.Millisecond)
		}},
		{name: "storage result", call: func() error { return metrics.ObserveStorageOperation("ping", "database-password", time.Millisecond) }},
		{name: "negative replay count", call: func() error { return metrics.AddRecoveryReplayedOutcomes("experiment-a", -1) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("metric observation error = nil")
			}
		})
	}
}

func TestMetrics_RegistriesAndConcurrency(t *testing.T) {
	firstRegistry := prometheus.NewRegistry()
	first, err := NewMetrics(firstRegistry)
	if err != nil {
		t.Fatalf("NewMetrics(first) error = %v", err)
	}
	if _, err := NewMetrics(firstRegistry); err == nil {
		t.Fatal("duplicate NewMetrics() error = nil")
	}
	second, err := NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("NewMetrics(separate registry) error = %v", err)
	}
	if _, err := NewMetrics(nil); err == nil {
		t.Fatal("NewMetrics(nil) error = nil")
	}

	const goroutines = 16
	const observations = 100
	errorsChannel := make(chan error, goroutines*observations*2)
	var waitGroup sync.WaitGroup
	for range goroutines {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range observations {
				if err := first.ObserveHTTPRequest("/v1/decisions", "POST", 201, time.Millisecond); err != nil {
					errorsChannel <- err
				}
				if err := second.ObserveOutcome("experiment-a", "clicked"); err != nil {
					errorsChannel <- err
				}
			}
		}()
	}
	waitGroup.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Errorf("concurrent observation error = %v", err)
	}
}

func observeAllMetrics(metrics *Metrics) error {
	operations := []func() error{
		func() error { return metrics.ObserveHTTPRequest("/v1/decisions", "POST", 201, 10*time.Millisecond) },
		func() error { return metrics.ObserveDecision("experiment-a", "random", "offer-a") },
		func() error { return metrics.ObserveOutcome("experiment-a", "clicked") },
		func() error { return metrics.AddReward("experiment-a", "random", 0.25) },
		func() error { return metrics.SetPolicyVersion("experiment-a", 2) },
		func() error { return metrics.ObservePolicyUpdate("experiment-a", "applied", time.Millisecond) },
		func() error { return metrics.SetSimulationActive("experiment-a", true) },
		func() error { return metrics.ObserveSimulationEvent("experiment-a", "decision") },
		func() error { return metrics.ObserveStorageOperation("ping", "success", time.Millisecond) },
		func() error { return metrics.AddRecoveryReplayedOutcomes("experiment-a", 3) },
	}
	for _, operation := range operations {
		if err := operation(); err != nil {
			return err
		}
	}
	return nil
}
