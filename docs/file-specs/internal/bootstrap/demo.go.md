# File Spec: `internal/bootstrap/demo.go`

## Status

`specified`

## Purpose

Define and create the versioned fictional OfferPilot demo experiment and offer catalog.

## Depends On

- Domain models, engine experiment creation, and approved simulation profile categories.

## Public Surface

- Demo template version, fictional offer definitions, `EnsureDemo`, and `CreateFreshDemo` operations.

## Required Behavior

- Define six clearly fictional merchants, one in each approved category, with no copied logos, claims, or trademarks.
- Ensure one initial stopped/running-ready demo exists idempotently at startup.
- Create fresh experiments with unique IDs/slugs while retaining template version metadata.
- Validate the complete template through domain validation before persistence.

## Failure Cases

- Partial catalog creation, duplicate template race, invalid policy config, real brand use, or mismatch with simulation profiles.

## Non-Goals

- SQL, HTTP responses, production merchant imports, or hidden reward parameters.

## Validation

- `go test -run TestDemo ./internal/bootstrap`

## Completion Checklist

- [ ] Catalog is fictional, stable, and valid.
- [ ] Ensure operation is idempotent under concurrency.
- [ ] Fresh demos preserve history.
