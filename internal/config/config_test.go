package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

const testDatabaseURL = "postgres://offerpilot:local-only@127.0.0.1:5432/offerpilot?sslmode=disable"

var environmentKeys = []string{
	"OFFERPILOT_HTTP_ADDR",
	"OFFERPILOT_DATABASE_URL",
	"OFFERPILOT_DATABASE_MAX_CONNS",
	"OFFERPILOT_DATABASE_MIN_CONNS",
	"OFFERPILOT_DATABASE_MAX_CONN_LIFETIME",
	"OFFERPILOT_DATABASE_MAX_CONN_IDLE_TIME",
	"OFFERPILOT_DATABASE_HEALTH_CHECK_PERIOD",
	"OFFERPILOT_LOG_LEVEL",
	"OFFERPILOT_LOG_FORMAT",
	"OFFERPILOT_CORS_ALLOWED_ORIGINS",
	"OFFERPILOT_HTTP_READ_HEADER_TIMEOUT",
	"OFFERPILOT_HTTP_READ_TIMEOUT",
	"OFFERPILOT_HTTP_WRITE_TIMEOUT",
	"OFFERPILOT_HTTP_IDLE_TIMEOUT",
	"OFFERPILOT_HTTP_MAX_BODY_BYTES",
	"OFFERPILOT_SHUTDOWN_TIMEOUT",
	"OFFERPILOT_OUTCOME_MAX_FUTURE_SKEW",
	"OFFERPILOT_SIM_MAX_REQUESTS_PER_SECOND",
	"OFFERPILOT_SIM_MAX_DECISIONS",
	"OFFERPILOT_SIM_MAX_WORKERS",
	"OFFERPILOT_SIM_MAX_DURATION",
	"OFFERPILOT_SIM_MAX_ERRORS",
	"OFFERPILOT_API_PROXY_TARGET",
	"VITE_API_BASE_URL",
}

func TestLoad_Defaults(t *testing.T) {
	isolateEnvironment(t)
	setEnvironment(t, "OFFERPILOT_DATABASE_URL", testDatabaseURL)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		HTTP: HTTPConfig{
			Addr:                 "127.0.0.1:8080",
			ReadHeaderTimeout:    5 * time.Second,
			ReadTimeout:          10 * time.Second,
			WriteTimeout:         30 * time.Second,
			IdleTimeout:          60 * time.Second,
			MaxBodyBytes:         1_048_576,
			OutcomeMaxFutureSkew: 2 * time.Minute,
		},
		Database: DatabaseConfig{
			URL:               testDatabaseURL,
			MaxConns:          10,
			MinConns:          1,
			MaxConnLifetime:   30 * time.Minute,
			MaxConnIdleTime:   5 * time.Minute,
			HealthCheckPeriod: 30 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		CORS: CORSConfig{
			AllowedOrigins:   []string{"http://localhost:5173"},
			AllowCredentials: false,
		},
		Shutdown: ShutdownConfig{
			Timeout: 15 * time.Second,
		},
		Simulation: SimulationConfig{
			MaxRequestsPerSecond: 100,
			MaxDecisions:         100_000,
			MaxWorkers:           32,
			MaxDuration:          30 * time.Minute,
			MaxErrors:            100,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	isolateEnvironment(t)
	overrides := map[string]string{
		"OFFERPILOT_HTTP_ADDR":                    "0.0.0.0:9090",
		"OFFERPILOT_DATABASE_URL":                 "postgresql://app:local@db:5432/custom?sslmode=disable",
		"OFFERPILOT_DATABASE_MAX_CONNS":           "20",
		"OFFERPILOT_DATABASE_MIN_CONNS":           "2",
		"OFFERPILOT_DATABASE_MAX_CONN_LIFETIME":   "20m",
		"OFFERPILOT_DATABASE_MAX_CONN_IDLE_TIME":  "4m",
		"OFFERPILOT_DATABASE_HEALTH_CHECK_PERIOD": "15s",
		"OFFERPILOT_LOG_LEVEL":                    "DEBUG",
		"OFFERPILOT_LOG_FORMAT":                   "json",
		"OFFERPILOT_CORS_ALLOWED_ORIGINS":         "https://dashboard.example.com, http://127.0.0.1:4173/",
		"OFFERPILOT_HTTP_READ_HEADER_TIMEOUT":     "1s",
		"OFFERPILOT_HTTP_READ_TIMEOUT":            "2s",
		"OFFERPILOT_HTTP_WRITE_TIMEOUT":           "3s",
		"OFFERPILOT_HTTP_IDLE_TIMEOUT":            "4s",
		"OFFERPILOT_HTTP_MAX_BODY_BYTES":          "4096",
		"OFFERPILOT_SHUTDOWN_TIMEOUT":             "5s",
		"OFFERPILOT_OUTCOME_MAX_FUTURE_SKEW":      "1m",
		"OFFERPILOT_SIM_MAX_REQUESTS_PER_SECOND":  "25",
		"OFFERPILOT_SIM_MAX_DECISIONS":            "5000",
		"OFFERPILOT_SIM_MAX_WORKERS":              "8",
		"OFFERPILOT_SIM_MAX_DURATION":             "10m",
		"OFFERPILOT_SIM_MAX_ERRORS":               "10",
		"OFFERPILOT_API_PROXY_TARGET":             "not-an-origin",
		"VITE_API_BASE_URL":                       "also-not-an-origin",
	}
	for name, value := range overrides {
		setEnvironment(t, name, value)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.HTTP.Addr != "0.0.0.0:9090" || got.HTTP.ReadHeaderTimeout != time.Second || got.HTTP.ReadTimeout != 2*time.Second || got.HTTP.WriteTimeout != 3*time.Second || got.HTTP.IdleTimeout != 4*time.Second || got.HTTP.MaxBodyBytes != 4096 || got.HTTP.OutcomeMaxFutureSkew != time.Minute {
		t.Fatalf("HTTP overrides not applied: %#v", got.HTTP)
	}
	if got.Database.URL != overrides["OFFERPILOT_DATABASE_URL"] || got.Database.MaxConns != 20 || got.Database.MinConns != 2 || got.Database.MaxConnLifetime != 20*time.Minute || got.Database.MaxConnIdleTime != 4*time.Minute || got.Database.HealthCheckPeriod != 15*time.Second {
		t.Fatalf("database overrides not applied: %#v", got.Database)
	}
	if got.Logging != (LoggingConfig{Level: "debug", Format: "json"}) {
		t.Fatalf("logging overrides not applied: %#v", got.Logging)
	}
	wantOrigins := []string{"https://dashboard.example.com", "http://127.0.0.1:4173"}
	if !reflect.DeepEqual(got.CORS.AllowedOrigins, wantOrigins) || got.CORS.AllowCredentials {
		t.Fatalf("CORS overrides not applied: %#v", got.CORS)
	}
	if got.Shutdown.Timeout != 5*time.Second {
		t.Fatalf("shutdown override not applied: %#v", got.Shutdown)
	}
	if got.Simulation != (SimulationConfig{MaxRequestsPerSecond: 25, MaxDecisions: 5000, MaxWorkers: 8, MaxDuration: 10 * time.Minute, MaxErrors: 10}) {
		t.Fatalf("simulation overrides not applied: %#v", got.Simulation)
	}
}

func TestLoad_InvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		variable  string
		value     string
		configure map[string]string
	}{
		{name: "database URL empty", variable: "OFFERPILOT_DATABASE_URL", value: ""},
		{name: "database URL malformed", variable: "OFFERPILOT_DATABASE_URL", value: "://secret"},
		{name: "database max malformed", variable: "OFFERPILOT_DATABASE_MAX_CONNS", value: "many"},
		{name: "database max zero", variable: "OFFERPILOT_DATABASE_MAX_CONNS", value: "0"},
		{name: "database min negative", variable: "OFFERPILOT_DATABASE_MIN_CONNS", value: "-1"},
		{name: "database min exceeds max", variable: "OFFERPILOT_DATABASE_MIN_CONNS", value: "11"},
		{name: "database lifetime zero", variable: "OFFERPILOT_DATABASE_MAX_CONN_LIFETIME", value: "0s"},
		{name: "database idle malformed", variable: "OFFERPILOT_DATABASE_MAX_CONN_IDLE_TIME", value: "later"},
		{name: "database health negative", variable: "OFFERPILOT_DATABASE_HEALTH_CHECK_PERIOD", value: "-1s"},
		{name: "HTTP address malformed", variable: "OFFERPILOT_HTTP_ADDR", value: "localhost"},
		{name: "HTTP port zero", variable: "OFFERPILOT_HTTP_ADDR", value: "127.0.0.1:0"},
		{name: "HTTP port excessive", variable: "OFFERPILOT_HTTP_ADDR", value: "127.0.0.1:65536"},
		{name: "read header zero", variable: "OFFERPILOT_HTTP_READ_HEADER_TIMEOUT", value: "0s"},
		{name: "read malformed", variable: "OFFERPILOT_HTTP_READ_TIMEOUT", value: "soon"},
		{name: "write negative", variable: "OFFERPILOT_HTTP_WRITE_TIMEOUT", value: "-1s"},
		{name: "idle zero", variable: "OFFERPILOT_HTTP_IDLE_TIMEOUT", value: "0s"},
		{name: "body malformed", variable: "OFFERPILOT_HTTP_MAX_BODY_BYTES", value: "large"},
		{name: "body zero", variable: "OFFERPILOT_HTTP_MAX_BODY_BYTES", value: "0"},
		{name: "body excessive", variable: "OFFERPILOT_HTTP_MAX_BODY_BYTES", value: "1048577"},
		{name: "skew malformed", variable: "OFFERPILOT_OUTCOME_MAX_FUTURE_SKEW", value: "later"},
		{name: "skew zero", variable: "OFFERPILOT_OUTCOME_MAX_FUTURE_SKEW", value: "0s"},
		{name: "skew excessive", variable: "OFFERPILOT_OUTCOME_MAX_FUTURE_SKEW", value: "2m1s"},
		{name: "shutdown zero", variable: "OFFERPILOT_SHUTDOWN_TIMEOUT", value: "0s"},
		{name: "log level", variable: "OFFERPILOT_LOG_LEVEL", value: "verbose"},
		{name: "log format", variable: "OFFERPILOT_LOG_FORMAT", value: "yaml"},
		{name: "rate zero", variable: "OFFERPILOT_SIM_MAX_REQUESTS_PER_SECOND", value: "0"},
		{name: "rate negative", variable: "OFFERPILOT_SIM_MAX_REQUESTS_PER_SECOND", value: "-1"},
		{name: "rate excessive", variable: "OFFERPILOT_SIM_MAX_REQUESTS_PER_SECOND", value: "101"},
		{name: "decisions zero", variable: "OFFERPILOT_SIM_MAX_DECISIONS", value: "0"},
		{name: "decisions negative", variable: "OFFERPILOT_SIM_MAX_DECISIONS", value: "-1"},
		{name: "decisions excessive", variable: "OFFERPILOT_SIM_MAX_DECISIONS", value: "100001"},
		{name: "workers zero", variable: "OFFERPILOT_SIM_MAX_WORKERS", value: "0"},
		{name: "workers negative", variable: "OFFERPILOT_SIM_MAX_WORKERS", value: "-1"},
		{name: "workers excessive", variable: "OFFERPILOT_SIM_MAX_WORKERS", value: "33"},
		{name: "duration zero", variable: "OFFERPILOT_SIM_MAX_DURATION", value: "0s"},
		{name: "duration negative", variable: "OFFERPILOT_SIM_MAX_DURATION", value: "-1s"},
		{name: "duration excessive", variable: "OFFERPILOT_SIM_MAX_DURATION", value: "30m1s"},
		{name: "errors zero", variable: "OFFERPILOT_SIM_MAX_ERRORS", value: "0"},
		{name: "errors negative", variable: "OFFERPILOT_SIM_MAX_ERRORS", value: "-1"},
		{name: "errors excessive", variable: "OFFERPILOT_SIM_MAX_ERRORS", value: "101"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateEnvironment(t)
			setEnvironment(t, "OFFERPILOT_DATABASE_URL", testDatabaseURL)
			for name, value := range test.configure {
				setEnvironment(t, name, value)
			}
			setEnvironment(t, test.variable, test.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want %s failure", test.variable)
			}
			if !strings.Contains(err.Error(), test.variable) {
				t.Fatalf("Load() error = %q, want variable %s", err, test.variable)
			}
			if test.value != "" && strings.Contains(err.Error(), test.value) && test.value != "0" && test.value != "0s" && test.value != "-1" && test.value != "-1s" {
				t.Fatalf("Load() error leaked rejected value %q", test.value)
			}
		})
	}
}

func TestLoad_InvalidCORSOrigins(t *testing.T) {
	tests := []string{
		"",
		"*",
		"ftp://localhost",
		"localhost:5173",
		"http://localhost/path",
		"http://localhost?query=value",
		"http://localhost#fragment",
		"http://user:private@localhost",
		"http://localhost:5173,http://LOCALHOST:5173/",
		"http://localhost:5173,",
		"http://localhost:0",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			isolateEnvironment(t)
			setEnvironment(t, "OFFERPILOT_DATABASE_URL", testDatabaseURL)
			setEnvironment(t, "OFFERPILOT_CORS_ALLOWED_ORIGINS", value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil for origin %q", value)
			}
			if !strings.Contains(err.Error(), "OFFERPILOT_CORS_ALLOWED_ORIGINS") {
				t.Fatalf("Load() error = %q, want CORS variable", err)
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatalf("Load() error leaked credentials: %q", err)
			}
		})
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	isolateEnvironment(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing database URL failure")
	}
	if !strings.Contains(err.Error(), "OFFERPILOT_DATABASE_URL") {
		t.Fatalf("Load() error = %q, want database variable", err)
	}
}

func TestLoad_SecretSafeErrors(t *testing.T) {
	isolateEnvironment(t)
	const secret = "do-not-print-this-password"
	setEnvironment(t, "OFFERPILOT_DATABASE_URL", "postgres://offerpilot:"+secret+"@localhost")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want malformed database URL failure")
	}
	if !strings.Contains(err.Error(), "OFFERPILOT_DATABASE_URL") {
		t.Fatalf("Load() error = %q, want database variable", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("Load() leaked database URL material: %q", err)
	}
}

func isolateEnvironment(t *testing.T) {
	t.Helper()

	type originalValue struct {
		value string
		set   bool
	}
	originals := make(map[string]originalValue, len(environmentKeys))
	for _, name := range environmentKeys {
		value, set := os.LookupEnv(name)
		originals[name] = originalValue{value: value, set: set}
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}

	t.Cleanup(func() {
		for _, name := range environmentKeys {
			original := originals[name]
			var err error
			if original.set {
				err = os.Setenv(name, original.value)
			} else {
				err = os.Unsetenv(name)
			}
			if err != nil {
				t.Errorf("restore %s: %v", name, err)
			}
		}
	})
}

func setEnvironment(t *testing.T, name, value string) {
	t.Helper()
	if err := os.Setenv(name, value); err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}
