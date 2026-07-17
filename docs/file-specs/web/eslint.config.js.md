# File Spec: `web/eslint.config.js`

## Status

`specified`

## Purpose

Define flat ESLint rules for strict React/TypeScript correctness.

## Depends On

- Frontend package and TypeScript config.

## Public Surface

- ESLint flat configuration for source, tests, and config files.

## Required Behavior

- Use recommended TypeScript, React Hooks, and React refresh rules with type-aware rules where practical.
- Ignore `dist`, coverage, and dependencies only.
- Configure browser globals for source and test globals for tests.
- Fail on floating promises, unsafe type escape, hooks violations, and accidental console use outside approved error reporting.

## Failure Cases

- Legacy config format, blanket rule disables, linting generated output, or conflict with TypeScript compiler settings.

## Non-Goals

- Formatting source or linting Go/Markdown.

## Validation

- Import the ESM flat config with Node and print the effective config for a representative `src/main.tsx` path.
- Run the full lint script after Phase 6 source exists.

## Completion Checklist

- [ ] Flat config loads without warnings.
- [ ] Correct environments apply by glob.
- [ ] No broad suppressions exist.
