package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/config"
	"github.com/onatozmenn/offerpilot/internal/observability"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type Readiness interface {
	Ready(context.Context) error
}

type ReadinessFunc func(context.Context) error

func (function ReadinessFunc) Ready(ctx context.Context) error {
	return function(ctx)
}

func NewRouter(
	handlers *Handlers,
	logger *slog.Logger,
	metrics *observability.Metrics,
	metricsHandler http.Handler,
	readiness Readiness,
	httpConfig config.HTTPConfig,
	corsConfig config.CORSConfig,
) (http.Handler, error) {
	if handlers == nil {
		return nil, fmt.Errorf("handlers are required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if metrics == nil {
		return nil, fmt.Errorf("metrics are required")
	}
	if metricsHandler == nil {
		return nil, fmt.Errorf("metrics handler is required")
	}
	if readiness == nil {
		return nil, fmt.Errorf("readiness dependency is required")
	}
	if httpConfig.MaxBodyBytes < 1 || httpConfig.WriteTimeout <= 0 {
		return nil, fmt.Errorf("HTTP body limit and write timeout must be positive")
	}
	if corsConfig.AllowCredentials {
		return nil, fmt.Errorf("CORS credentials must remain disabled")
	}
	allowedOrigins := make(map[string]struct{}, len(corsConfig.AllowedOrigins))
	for _, origin := range corsConfig.AllowedOrigins {
		if strings.TrimSpace(origin) == "" || origin == "*" {
			return nil, fmt.Errorf("CORS origins must be explicit")
		}
		allowedOrigins[origin] = struct{}{}
	}

	router := chi.NewRouter()
	router.Use(requestIDMiddleware)
	router.Use(metricsMiddleware(metrics))
	router.Use(accessLogMiddleware(logger))
	router.Use(recoveryMiddleware(logger))
	router.Use(corsMiddleware(allowedOrigins))
	router.Use(timeoutMiddleware(httpConfig.WriteTimeout))

	router.Get("/health/live", func(writer http.ResponseWriter, request *http.Request) {
		writeOperationalJSON(writer, request, http.StatusOK, healthDTO{Status: "live"}, logger, "liveness")
	})
	router.Get("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		if err := readiness.Ready(request.Context()); err != nil {
			requestID := requestIDFromContext(request.Context())
			observability.LogInternalError(logger, requestID, "readiness", err)
			writeProblem(writer, newProblem(
				http.StatusServiceUnavailable,
				codeDatabaseUnavailable,
				"Service unavailable",
				"The API is not ready to serve requests.",
				requestID,
			))
			return
		}
		writeOperationalJSON(writer, request, http.StatusOK, healthDTO{Status: "ready"}, logger, "readiness")
	})
	router.Handle("/metrics", metricsHandler)

	router.Route("/v1", func(api chi.Router) {
		api.Get("/experiments", handlers.ListExperiments)
		api.Get("/experiments/{experiment_id}", handlers.GetExperiment)
		api.Get("/experiments/{experiment_id}/summary", handlers.GetExperimentSummary)
		api.Get("/experiments/{experiment_id}/decisions", handlers.ListExperimentDecisions)
		api.Get("/simulation-runs/{run_id}", handlers.GetSimulationRun)
		api.Post("/simulation-runs/{run_id}/stop", handlers.StopSimulationRun)

		api.Group(func(writes chi.Router) {
			writes.Use(jsonWriteMiddleware(logger, httpConfig.MaxBodyBytes))
			writes.Post("/demo/experiments", handlers.CreateDemoExperiment)
			writes.Post("/decisions", handlers.CreateDecision)
			writes.Post("/outcomes", handlers.CreateOutcome)
			writes.Post("/experiments/{experiment_id}/simulation-runs", handlers.CreateSimulationRun)
		})
	})

	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		writeProblem(writer, newProblem(http.StatusNotFound, codeNotFound, "Resource not found", "The requested route does not exist.", requestIDFromContext(request.Context())))
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		writeProblem(writer, newProblem(http.StatusMethodNotAllowed, codeInvalidQuery, "Method not allowed", "The requested method is not supported for this route.", requestIDFromContext(request.Context())))
	})

	return router, nil
}

func NewServer(address string, handler http.Handler, httpConfig config.HTTPConfig) (*http.Server, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("HTTP address is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("HTTP handler is required")
	}
	if httpConfig.ReadHeaderTimeout <= 0 || httpConfig.ReadTimeout <= 0 || httpConfig.WriteTimeout <= 0 || httpConfig.IdleTimeout <= 0 {
		return nil, fmt.Errorf("HTTP server timeouts must be positive")
	}
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: httpConfig.ReadHeaderTimeout,
		ReadTimeout:       httpConfig.ReadTimeout,
		WriteTimeout:      httpConfig.WriteTimeout,
		IdleTimeout:       httpConfig.IdleTimeout,
	}, nil
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(requestID) {
			requestID = uuid.NewString()
		}
		writer.Header().Set("X-Request-ID", requestID)
		ctx := contextWithRequestID(request.Context(), requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func(ctx context.Context) {
				if recovered := recover(); recovered != nil {
					requestID := requestIDFromContext(ctx)
					observability.LogInternalError(logger, requestID, "panic_recovery", fmt.Errorf("panic type %T", recovered))
					writeProblem(writer, newProblem(http.StatusInternalServerError, codeInternalError, "Internal server error", "The server could not complete the request.", requestID))
				}
			}(request.Context())
			next.ServeHTTP(writer, request)
		})
	}
}

func corsMiddleware(allowedOrigins map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			_, allowed := allowedOrigins[origin]
			if origin != "" {
				writer.Header().Add("Vary", "Origin")
			}
			if allowed {
				writer.Header().Set("Access-Control-Allow-Origin", origin)
				writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
				writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Location")
			}
			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			ctx, cancel := context.WithTimeout(request.Context(), timeout)
			defer cancel()
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

func jsonWriteMiddleware(logger *slog.Logger, maxBodyBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				writeMappedError(writer, request, logger, "validate_content_type", newRequestError(http.StatusUnsupportedMediaType, codeUnsupportedMediaType, "Unsupported media type", "JSON write requests require application/json."))
				return
			}
			request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
			next.ServeHTTP(writer, request)
		})
	}
}

func metricsMiddleware(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
			startedAt := time.Now()
			next.ServeHTTP(recorder, request)
			route := chi.RouteContext(request.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			_ = metrics.ObserveHTTPRequest(route, request.Method, recorder.status, time.Since(startedAt))
		})
	}
}

func accessLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
			startedAt := time.Now()
			next.ServeHTTP(recorder, request)
			route := chi.RouteContext(request.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			logger.Info(
				"HTTP request completed",
				slog.String("request_id", requestIDFromContext(request.Context())),
				slog.String("method", request.Method),
				slog.String("route", route),
				slog.Int("status", recorder.status),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (recorder *statusRecorder) WriteHeader(status int) {
	if recorder.wroteHeader {
		return
	}
	recorder.status = status
	recorder.wroteHeader = true
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	if !recorder.wroteHeader {
		recorder.WriteHeader(http.StatusOK)
	}
	return recorder.ResponseWriter.Write(body)
}

func (recorder *statusRecorder) Unwrap() http.ResponseWriter {
	return recorder.ResponseWriter
}

func writeOperationalJSON(writer http.ResponseWriter, request *http.Request, status int, value any, logger *slog.Logger, operation string) {
	if err := writeJSON(writer, status, value); err != nil {
		observability.LogInternalError(logger, requestIDFromContext(request.Context()), operation, err)
	}
}
