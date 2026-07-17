# File Spec: `web/tsconfig.json`

## Status

`validated`

## Purpose

Configure strict TypeScript for browser source, tests, and Vite configuration.

## Depends On

- Frontend package dependencies and source layout.

## Public Surface

- Compiler options consumed by editor, `tsc`, Vite, Vitest, and ESLint.

## Required Behavior

- Enable strict mode, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noFallthroughCasesInSwitch`, unused checks, and modern DOM/ES libraries.
- Use bundler module resolution, JSX transform, `noEmit`, and Vite client/Vitest types without a generated env declaration file.
- Include source, test setup, Vite config, and ESLint config with appropriate environment types.

## Failure Cases

- `skipLibCheck` hiding local errors through overly broad includes, CommonJS mismatch, or disabling strictness for convenience.

## Non-Goals

- Emitting JavaScript or configuring backend TypeScript.

## Validation

- Parse the strict JSON file with Node immediately.
- Run `npm --prefix web run typecheck` once Phase 6 source exists; reopening this validated shared config requires rerunning both checks.

## Completion Checklist

- [ ] Strict options are active.
- [ ] App, tests, and config typecheck.
- [ ] No generated declaration file is required.
