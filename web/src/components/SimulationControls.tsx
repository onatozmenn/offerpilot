import { Play, Square } from "lucide-react";
import { useRef } from "react";
import type { FormEvent } from "react";

import type { SimulationFormField } from "../hooks/useDashboard";
import type { SimulationRun } from "../types/api";

interface SimulationControlsProps {
  seed: number | "";
  requestsPerSecond: number | "";
  maxDecisions: number | "";
  seedMin?: number;
  seedMax?: number;
  requestsPerSecondMin?: number;
  requestsPerSecondMax?: number;
  maxDecisionsMin?: number;
  maxDecisionsMax?: number;
  run: SimulationRun | null;
  isStartPending?: boolean;
  isStopPending?: boolean;
  error?: string | null;
  onChange: (field: SimulationFormField, value: string) => void;
  onStart: () => Promise<void>;
  onStop: () => Promise<void>;
}

const defaultSeedMin = Number.MIN_SAFE_INTEGER;
const defaultSeedMax = Number.MAX_SAFE_INTEGER;

export function SimulationControls({
  seed,
  requestsPerSecond,
  maxDecisions,
  seedMin = defaultSeedMin,
  seedMax = defaultSeedMax,
  requestsPerSecondMin = 1,
  requestsPerSecondMax = 100,
  maxDecisionsMin = 1,
  maxDecisionsMax = 100_000,
  run,
  isStartPending = false,
  isStopPending = false,
  error = null,
  onChange,
  onStart,
  onStop,
}: SimulationControlsProps) {
  const inFlightCommandRef = useRef<"start" | "stop" | null>(null);
  const seedValid = validInteger(seed, seedMin, seedMax);
  const rateValid = validInteger(requestsPerSecond, requestsPerSecondMin, requestsPerSecondMax);
  const decisionsValid = validInteger(maxDecisions, maxDecisionsMin, maxDecisionsMax);
  const runActive = run?.status === "starting" || run?.status === "running" || run?.status === "stopping";
  const startDisabled = !seedValid || !rateValid || !decisionsValid || runActive || isStartPending || isStopPending;
  const stopDisabled = run === null || run.status === "stopping" || !runActive || isStartPending || isStopPending;

  const handleSubmit = (event: FormEvent<HTMLFormElement>): void => {
    event.preventDefault();
    if (!startDisabled) {
      void invokeCommand("start", onStart);
    }
  };

  const invokeCommand = async (command: "start" | "stop", callback: () => Promise<void>): Promise<void> => {
    if (inFlightCommandRef.current !== null) {
      return;
    }
    inFlightCommandRef.current = command;
    try {
      await callback();
    } finally {
      if (inFlightCommandRef.current === command) {
        inFlightCommandRef.current = null;
      }
    }
  };

  return (
    <form className="simulation-controls" onSubmit={handleSubmit} noValidate>
      <div className="simulation-controls__fields">
        <NumericField
          id="simulation-seed"
          label="Seed"
          value={seed}
          minimum={seedMin}
          maximum={seedMax}
          valid={seedValid}
          error="Enter a whole-number seed."
          onChange={(value) => onChange("seed", value)}
        />
        <NumericField
          id="simulation-rate"
          label="Requests per second"
          value={requestsPerSecond}
          minimum={requestsPerSecondMin}
          maximum={requestsPerSecondMax}
          valid={rateValid}
          error={`Enter ${requestsPerSecondMin}-${requestsPerSecondMax}.`}
          onChange={(value) => onChange("requestsPerSecond", value)}
        />
        <NumericField
          id="simulation-max-decisions"
          label="Maximum decisions"
          value={maxDecisions}
          minimum={maxDecisionsMin}
          maximum={maxDecisionsMax}
          valid={decisionsValid}
          error={`Enter ${maxDecisionsMin}-${maxDecisionsMax.toLocaleString("en-US")}.`}
          onChange={(value) => onChange("maxDecisions", value)}
        />
      </div>

      <div className="simulation-controls__commands">
        <button className="button button--primary" type="submit" disabled={startDisabled}>
          <Play size={17} aria-hidden="true" />
          {isStartPending ? "Starting..." : "Start run"}
        </button>
        <button
          className="button button--stop"
          type="button"
          disabled={stopDisabled}
          onClick={() => void invokeCommand("stop", onStop)}
        >
          <Square size={16} aria-hidden="true" />
          {isStopPending || run?.status === "stopping" ? "Stopping..." : "Stop run"}
        </button>
      </div>

      <div className="simulation-controls__feedback">
        <p className="simulation-controls__run-status" aria-live="polite">
          {runStatusText(run)}
        </p>
        {error === null ? null : <p className="simulation-controls__error" role="alert">{error}</p>}
      </div>
    </form>
  );
}

interface NumericFieldProps {
  id: string;
  label: string;
  value: number | "";
  minimum: number;
  maximum: number;
  valid: boolean;
  error: string;
  onChange: (value: string) => void;
}

function NumericField({ id, label, value, minimum, maximum, valid, error, onChange }: NumericFieldProps) {
  const errorId = `${id}-error`;
  return (
    <div className="simulation-controls__field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="number"
        inputMode="numeric"
        min={minimum}
        max={maximum}
        step={1}
        required
        value={value}
        aria-invalid={!valid}
        aria-describedby={!valid ? errorId : undefined}
        onChange={(event) => onChange(event.currentTarget.value)}
      />
      <span className="simulation-controls__field-error" id={errorId} aria-live="polite">
        {valid ? "" : error}
      </span>
    </div>
  );
}

function validInteger(value: number | "", minimum: number, maximum: number): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= minimum && value <= maximum;
}

function runStatusText(run: SimulationRun | null): string {
  if (run === null) {
    return "No simulation run selected.";
  }
  const status = run.status[0]?.toUpperCase() + run.status.slice(1);
  return `${status}: ${run.decision_count.toLocaleString("en-US")} of ${run.max_decisions.toLocaleString("en-US")} decisions. Seed ${run.seed.toLocaleString("en-US")}.`;
}