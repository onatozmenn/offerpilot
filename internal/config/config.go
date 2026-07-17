package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr                  = "127.0.0.1:8080"
	defaultDatabaseMaxConns          = 10
	defaultDatabaseMinConns          = 1
	defaultDatabaseMaxConnLifetime   = 30 * time.Minute
	defaultDatabaseMaxConnIdleTime   = 5 * time.Minute
	defaultDatabaseHealthCheckPeriod = 30 * time.Second
	defaultLogLevel                  = "info"
	defaultLogFormat                 = "text"
	defaultCORSAllowedOrigins        = "http://localhost:5173"
	defaultHTTPReadHeaderTimeout     = 5 * time.Second
	defaultHTTPReadTimeout           = 10 * time.Second
	defaultHTTPWriteTimeout          = 30 * time.Second
	defaultHTTPIdleTimeout           = 60 * time.Second
	defaultHTTPMaxBodyBytes          = 1 << 20
	defaultShutdownTimeout           = 15 * time.Second
	defaultOutcomeMaxFutureSkew      = 2 * time.Minute
	defaultSimMaxRequestsPerSecond   = 100
	defaultSimMaxDecisions           = 100_000
	defaultSimMaxWorkers             = 32
	defaultSimMaxDuration            = 30 * time.Minute
	defaultSimMaxErrors              = 100
)

// Config contains validated runtime settings grouped by their owning concern.
type Config struct {
	HTTP       HTTPConfig
	Database   DatabaseConfig
	Logging    LoggingConfig
	CORS       CORSConfig
	Shutdown   ShutdownConfig
	Simulation SimulationConfig
}

type HTTPConfig struct {
	Addr                 string
	ReadHeaderTimeout    time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	MaxBodyBytes         int64
	OutcomeMaxFutureSkew time.Duration
}

type DatabaseConfig struct {
	URL               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

type LoggingConfig struct {
	Level  string
	Format string
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowCredentials bool
}

type ShutdownConfig struct {
	Timeout time.Duration
}

type SimulationConfig struct {
	MaxRequestsPerSecond int
	MaxDecisions         int
	MaxWorkers           int
	MaxDuration          time.Duration
	MaxErrors            int
}

// Load reads, defaults, and validates OfferPilot's API-owned environment variables.
func Load() (Config, error) {
	databaseURL, err := loadDatabaseURL()
	if err != nil {
		return Config{}, err
	}

	databaseMaxConns, err := loadInt("OFFERPILOT_DATABASE_MAX_CONNS", defaultDatabaseMaxConns, 1, int(^uint32(0)>>1))
	if err != nil {
		return Config{}, err
	}

	databaseMinConns, err := loadInt("OFFERPILOT_DATABASE_MIN_CONNS", defaultDatabaseMinConns, 0, int(^uint32(0)>>1))
	if err != nil {
		return Config{}, err
	}
	if databaseMinConns > databaseMaxConns {
		return Config{}, fmt.Errorf("OFFERPILOT_DATABASE_MIN_CONNS must not exceed OFFERPILOT_DATABASE_MAX_CONNS")
	}

	databaseMaxConnLifetime, err := loadPositiveDuration("OFFERPILOT_DATABASE_MAX_CONN_LIFETIME", defaultDatabaseMaxConnLifetime)
	if err != nil {
		return Config{}, err
	}
	databaseMaxConnIdleTime, err := loadPositiveDuration("OFFERPILOT_DATABASE_MAX_CONN_IDLE_TIME", defaultDatabaseMaxConnIdleTime)
	if err != nil {
		return Config{}, err
	}
	databaseHealthCheckPeriod, err := loadPositiveDuration("OFFERPILOT_DATABASE_HEALTH_CHECK_PERIOD", defaultDatabaseHealthCheckPeriod)
	if err != nil {
		return Config{}, err
	}

	httpAddr, err := loadHTTPAddr()
	if err != nil {
		return Config{}, err
	}
	httpReadHeaderTimeout, err := loadPositiveDuration("OFFERPILOT_HTTP_READ_HEADER_TIMEOUT", defaultHTTPReadHeaderTimeout)
	if err != nil {
		return Config{}, err
	}
	httpReadTimeout, err := loadPositiveDuration("OFFERPILOT_HTTP_READ_TIMEOUT", defaultHTTPReadTimeout)
	if err != nil {
		return Config{}, err
	}
	httpWriteTimeout, err := loadPositiveDuration("OFFERPILOT_HTTP_WRITE_TIMEOUT", defaultHTTPWriteTimeout)
	if err != nil {
		return Config{}, err
	}
	httpIdleTimeout, err := loadPositiveDuration("OFFERPILOT_HTTP_IDLE_TIMEOUT", defaultHTTPIdleTimeout)
	if err != nil {
		return Config{}, err
	}
	httpMaxBodyBytes, err := loadInt64("OFFERPILOT_HTTP_MAX_BODY_BYTES", defaultHTTPMaxBodyBytes, 1, defaultHTTPMaxBodyBytes)
	if err != nil {
		return Config{}, err
	}
	outcomeMaxFutureSkew, err := loadBoundedDuration("OFFERPILOT_OUTCOME_MAX_FUTURE_SKEW", defaultOutcomeMaxFutureSkew, defaultOutcomeMaxFutureSkew)
	if err != nil {
		return Config{}, err
	}

	logLevel, err := loadEnum("OFFERPILOT_LOG_LEVEL", defaultLogLevel, "debug", "info", "warn", "error")
	if err != nil {
		return Config{}, err
	}
	logFormat, err := loadEnum("OFFERPILOT_LOG_FORMAT", defaultLogFormat, "text", "json")
	if err != nil {
		return Config{}, err
	}

	allowedOrigins, err := loadAllowedOrigins()
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := loadPositiveDuration("OFFERPILOT_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	simMaxRequestsPerSecond, err := loadInt("OFFERPILOT_SIM_MAX_REQUESTS_PER_SECOND", defaultSimMaxRequestsPerSecond, 1, defaultSimMaxRequestsPerSecond)
	if err != nil {
		return Config{}, err
	}
	simMaxDecisions, err := loadInt("OFFERPILOT_SIM_MAX_DECISIONS", defaultSimMaxDecisions, 1, defaultSimMaxDecisions)
	if err != nil {
		return Config{}, err
	}
	simMaxWorkers, err := loadInt("OFFERPILOT_SIM_MAX_WORKERS", defaultSimMaxWorkers, 1, defaultSimMaxWorkers)
	if err != nil {
		return Config{}, err
	}
	simMaxDuration, err := loadBoundedDuration("OFFERPILOT_SIM_MAX_DURATION", defaultSimMaxDuration, defaultSimMaxDuration)
	if err != nil {
		return Config{}, err
	}
	simMaxErrors, err := loadInt("OFFERPILOT_SIM_MAX_ERRORS", defaultSimMaxErrors, 1, defaultSimMaxErrors)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTP: HTTPConfig{
			Addr:                 httpAddr,
			ReadHeaderTimeout:    httpReadHeaderTimeout,
			ReadTimeout:          httpReadTimeout,
			WriteTimeout:         httpWriteTimeout,
			IdleTimeout:          httpIdleTimeout,
			MaxBodyBytes:         httpMaxBodyBytes,
			OutcomeMaxFutureSkew: outcomeMaxFutureSkew,
		},
		Database: DatabaseConfig{
			URL:               databaseURL,
			MaxConns:          int32(databaseMaxConns),
			MinConns:          int32(databaseMinConns),
			MaxConnLifetime:   databaseMaxConnLifetime,
			MaxConnIdleTime:   databaseMaxConnIdleTime,
			HealthCheckPeriod: databaseHealthCheckPeriod,
		},
		Logging: LoggingConfig{
			Level:  logLevel,
			Format: logFormat,
		},
		CORS: CORSConfig{
			AllowedOrigins:   allowedOrigins,
			AllowCredentials: false,
		},
		Shutdown: ShutdownConfig{
			Timeout: shutdownTimeout,
		},
		Simulation: SimulationConfig{
			MaxRequestsPerSecond: simMaxRequestsPerSecond,
			MaxDecisions:         simMaxDecisions,
			MaxWorkers:           simMaxWorkers,
			MaxDuration:          simMaxDuration,
			MaxErrors:            simMaxErrors,
		},
	}, nil
}

func loadDatabaseURL() (string, error) {
	const name = "OFFERPILOT_DATABASE_URL"
	raw, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%s is required", name)
	}

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" || parsed.Path == "" || parsed.Path == "/" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must be a valid PostgreSQL URL", name)
	}

	return raw, nil
}

func loadHTTPAddr() (string, error) {
	const name = "OFFERPILOT_HTTP_ADDR"
	raw := loadString(name, defaultHTTPAddr)
	host, portText, err := net.SplitHostPort(raw)
	if err != nil || strings.TrimSpace(host) != host || strings.ContainsAny(host, "/?#") {
		return "", fmt.Errorf("%s must be a valid host and port", name)
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65_535 {
		return "", fmt.Errorf("%s must contain a port between 1 and 65535", name)
	}

	return raw, nil
}

func loadAllowedOrigins() ([]string, error) {
	const name = "OFFERPILOT_CORS_ALLOWED_ORIGINS"
	raw := loadString(name, defaultCORSAllowedOrigins)
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		origin, err := parseOrigin(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("%s must contain comma-separated HTTP or HTTPS origins", name)
		}

		key := strings.ToLower(origin)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%s must not contain duplicate origins", name)
		}
		seen[key] = struct{}{}
		origins = append(origins, origin)
	}

	return origins, nil
}

func parseOrigin(raw string) (string, error) {
	if raw == "" || raw == "*" {
		return "", fmt.Errorf("invalid origin")
	}

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" || parsed.ForceQuery || (parsed.Path != "" && parsed.Path != "/") || strings.Contains(parsed.Hostname(), "*") {
		return "", fmt.Errorf("invalid origin")
	}

	if strings.HasSuffix(parsed.Host, ":") {
		return "", fmt.Errorf("invalid origin")
	}
	if portText := parsed.Port(); portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65_535 {
			return "", fmt.Errorf("invalid origin")
		}
	}

	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func loadEnum(name, fallback string, allowed ...string) (string, error) {
	value := strings.ToLower(loadString(name, fallback))
	for _, candidate := range allowed {
		if value == candidate {
			return value, nil
		}
	}

	return "", fmt.Errorf("%s has an unsupported value", name)
}

func loadPositiveDuration(name string, fallback time.Duration) (time.Duration, error) {
	value, err := time.ParseDuration(loadString(name, fallback.String()))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}

	return value, nil
}

func loadBoundedDuration(name string, fallback, maximum time.Duration) (time.Duration, error) {
	value, err := loadPositiveDuration(name, fallback)
	if err != nil {
		return 0, err
	}
	if value > maximum {
		return 0, fmt.Errorf("%s exceeds the allowed maximum", name)
	}

	return value, nil
}

func loadInt(name string, fallback, minimum, maximum int) (int, error) {
	value, err := strconv.Atoi(loadString(name, strconv.Itoa(fallback)))
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}

	return value, nil
}

func loadInt64(name string, fallback, minimum, maximum int64) (int64, error) {
	value, err := strconv.ParseInt(loadString(name, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}

	return value, nil
}

func loadString(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}

	return strings.TrimSpace(value)
}
