# OfferPilot

OfferPilot is a Go-first online-learning platform that selects marketplace offers from privacy-safe session context, records feedback, and compares an adaptive policy with a random baseline.

The repository is currently in the **architecture and onboarding stage**. No application code should be generated until the corresponding file specifications are complete.

## Product Boundary

OfferPilot optimizes the ordering of fictional merchant offers. It does not determine credit eligibility, personalize loan terms, process payments, or use protected attributes.

## Planned Stack

- Go API and traffic simulator
- PostgreSQL
- React, TypeScript, and Vite dashboard
- Docker Compose for local development
- Prometheus-compatible metrics
- OpenAPI contract

## Local Prerequisites

- Go 1.26.x, validated locally with 1.26.5
- Node.js 22.x and npm 10.x, validated locally with 22.16.0 and 10.9.2
- Docker 29.x with Docker Compose, validated locally with 29.5.3

## Start Here

1. Read [AGENTS.md](AGENTS.md) for coding-agent rules.
2. Read the [documentation index](docs/README.md).
3. Confirm the architecture and product boundaries.
4. Follow the delivery plan and file manifest once those documents are marked complete.

## Status

| Area | Status |
| --- | --- |
| Product brief | Complete |
| Architecture | Complete |
| Domain model | Complete |
| API and data contracts | Complete |
| Per-file implementation specs | Complete: 70 planned files |
| Application implementation | Not started |
