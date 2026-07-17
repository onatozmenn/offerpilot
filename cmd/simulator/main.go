package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/simulation"
)

type cliConfig struct {
	BaseURL           string        `json:"base_url"`
	ExperimentID      uuid.UUID     `json:"experiment_id"`
	Seed              int64         `json:"seed"`
	RequestsPerSecond int           `json:"requests_per_second"`
	MaxDecisions      int           `json:"max_decisions"`
	Workers           int           `json:"workers"`
	MaxErrors         int           `json:"max_errors"`
	Timeout           time.Duration `json:"timeout"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	Output            string        `json:"output"`
}

type outputRecord struct {
	Type   string                `json:"type"`
	Config *cliConfig            `json:"config,omitempty"`
	Result *simulation.RunResult `json:"result,omitempty"`
	Error  string                `json:"error,omitempty"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	config, err := parseFlags(arguments, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "simulator configuration: %v\n", err)
		return 2
	}
	if err := writeConfig(stdout, config); err != nil {
		_, _ = fmt.Fprintf(stderr, "write simulator configuration: %v\n", err)
		return 1
	}

	client, err := simulation.NewDefaultHTTPClient(config.BaseURL, config.RequestTimeout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create simulator HTTP client: %v\n", err)
		return 2
	}
	runner, err := simulation.NewDefaultRunner(simulation.DefaultProfile(), client)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create simulator runner: %v\n", err)
		return 1
	}

	runCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	progressEvery := min(100, config.MaxDecisions)
	result, runErr := runner.Run(runCtx, simulation.RunConfig{
		ExperimentID:      config.ExperimentID,
		Seed:              config.Seed,
		RequestsPerSecond: config.RequestsPerSecond,
		MaxDecisions:      config.MaxDecisions,
		Workers:           config.Workers,
		MaxErrors:         config.MaxErrors,
		ProgressEvery:     progressEvery,
	})
	if err := writeResult(stdout, config.Output, result, runErr); err != nil {
		_, _ = fmt.Fprintf(stderr, "write simulator result: %v\n", err)
		return 1
	}
	if runErr == nil {
		return 0
	}
	if errors.Is(runErr, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
		return 0
	}
	_, _ = fmt.Fprintf(stderr, "simulation failed: %v\n", runErr)
	return 1
}

func parseFlags(arguments []string, output io.Writer) (cliConfig, error) {
	flags := flag.NewFlagSet("offerpilot-simulator", flag.ContinueOnError)
	flags.SetOutput(output)
	var config cliConfig
	var experimentID string
	flags.StringVar(&config.BaseURL, "base-url", "http://127.0.0.1:8080", "OfferPilot API origin")
	flags.StringVar(&experimentID, "experiment-id", "", "experiment UUID (required)")
	flags.Int64Var(&config.Seed, "seed", 20260717, "deterministic traffic seed")
	flags.IntVar(&config.RequestsPerSecond, "rate", 20, "requests per second (1-100)")
	flags.IntVar(&config.MaxDecisions, "max-decisions", 5000, "maximum decisions (1-100000)")
	flags.IntVar(&config.Workers, "workers", 8, "bounded worker count (1-32)")
	flags.IntVar(&config.MaxErrors, "max-errors", 100, "terminal error threshold (1-100)")
	flags.DurationVar(&config.Timeout, "timeout", 30*time.Minute, "overall simulation timeout")
	flags.DurationVar(&config.RequestTimeout, "request-timeout", 10*time.Second, "per-request HTTP timeout")
	flags.StringVar(&config.Output, "output", "human", "output format: human or json")
	if err := flags.Parse(arguments); err != nil {
		return cliConfig{}, err
	}
	if flags.NArg() != 0 {
		return cliConfig{}, fmt.Errorf("unexpected positional arguments")
	}
	parsedExperimentID, err := uuid.Parse(experimentID)
	if err != nil || parsedExperimentID == uuid.Nil {
		return cliConfig{}, fmt.Errorf("experiment-id must be a non-nil UUID")
	}
	config.ExperimentID = parsedExperimentID
	if config.RequestsPerSecond < 1 || config.RequestsPerSecond > 100 {
		return cliConfig{}, fmt.Errorf("rate must be between 1 and 100")
	}
	if config.MaxDecisions < 1 || config.MaxDecisions > 100_000 {
		return cliConfig{}, fmt.Errorf("max-decisions must be between 1 and 100000")
	}
	if config.Workers < 1 || config.Workers > 32 {
		return cliConfig{}, fmt.Errorf("workers must be between 1 and 32")
	}
	if config.MaxErrors < 1 || config.MaxErrors > 100 {
		return cliConfig{}, fmt.Errorf("max-errors must be between 1 and 100")
	}
	if config.Timeout <= 0 || config.Timeout > 24*time.Hour {
		return cliConfig{}, fmt.Errorf("timeout must be positive and at most 24h")
	}
	if config.RequestTimeout <= 0 || config.RequestTimeout > config.Timeout {
		return cliConfig{}, fmt.Errorf("request-timeout must be positive and no greater than timeout")
	}
	config.Output = strings.ToLower(strings.TrimSpace(config.Output))
	if config.Output != "human" && config.Output != "json" {
		return cliConfig{}, fmt.Errorf("output must be human or json")
	}
	return config, nil
}

func writeConfig(writer io.Writer, config cliConfig) error {
	if config.Output == "json" {
		return json.NewEncoder(writer).Encode(outputRecord{Type: "config", Config: &config})
	}
	_, err := fmt.Fprintf(
		writer,
		"OfferPilot simulation: experiment=%s seed=%d rate=%d max_decisions=%d workers=%d max_errors=%d\n",
		config.ExperimentID,
		config.Seed,
		config.RequestsPerSecond,
		config.MaxDecisions,
		config.Workers,
		config.MaxErrors,
	)
	return err
}

func writeResult(writer io.Writer, output string, result simulation.RunResult, runErr error) error {
	if output == "json" {
		record := outputRecord{Type: "result", Result: &result}
		if runErr != nil {
			record.Error = runErr.Error()
		}
		return json.NewEncoder(writer).Encode(record)
	}
	status := "completed"
	if runErr != nil {
		status = "partial"
	}
	_, err := fmt.Fprintf(
		writer,
		"Simulation %s: attempts=%d decisions=%d outcomes=%d errors=%d observed_reward=%.4f random_expected=%.4f oracle_expected=%.4f\n",
		status,
		result.AttemptCount,
		result.DecisionCount,
		result.OutcomeCount,
		result.ErrorCount,
		result.ObservedRewardSum,
		result.RandomExpectedRewardSum,
		result.OracleExpectedRewardSum,
	)
	return err
}
