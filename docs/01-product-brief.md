# Product Brief

## Problem

Static marketplace rankings show the same offers to broad groups and learn slowly through manual A/B tests. A portfolio-scale system should demonstrate how an online policy can adapt offer selection from immediate feedback while remaining observable, reproducible, and privacy-safe.

## Product

OfferPilot is a local-first experimentation lab. It accepts an anonymous session context and eligible fictional offers, selects one offer, records the action probability, accepts a terminal outcome, and updates an online policy. A dashboard compares the adaptive policy with a random baseline in a seeded traffic simulation.

## Primary User

An AI or growth engineer evaluating whether an online-learning policy improves marketplace engagement without violating product constraints.

## MVP User Journey

1. The user opens the dashboard and sees a stopped demo experiment.
2. The user starts a seeded simulation and chooses requests per second.
3. The simulator creates anonymous contexts and sends decision requests.
4. The API chooses an eligible offer using either the baseline or adaptive policy.
5. The simulator produces one terminal outcome per decision.
6. The dashboard displays reward, click-through proxy, offer distribution, exploration rate, and latency.
7. The user pauses the run, changes the seed or policy, resets state, and compares results reproducibly.

## MVP Scope

- One built-in experiment with fictional offers and deterministic synthetic traffic.
- Random baseline policy.
- Segmented epsilon-greedy adaptive policy with known action probabilities.
- Decision and terminal-feedback APIs.
- Persisted experiments, offers, decisions, outcomes, policy state, and simulation runs.
- IPS and self-normalized IPS estimates over logged policy data.
- React operations dashboard with simulation controls and live polling.
- Structured logs, Prometheus-compatible metrics, health endpoints, tests, and Docker Compose.

## Explicit Non-Goals

- Credit approval, lending decisions, payment-plan recommendations, or dynamic pricing.
- Production advertising, sponsored-placement billing, or real merchant integrations.
- User accounts, authentication, multi-tenancy, or personally identifiable information.
- Kafka, Kubernetes, microservices, feature stores, or distributed model training in the MVP.
- LLM-generated recommendations. The core AI signal is measurable online learning.
- Scraping Sezzle or any merchant marketplace.

## Context Features

Only coarse, synthetic, session-level features are allowed:

- Device class: `mobile`, `desktop`, or `tablet`.
- Daypart: `morning`, `afternoon`, `evening`, or `night`.
- Category affinity: one of the configured fictional offer categories.
- Visitor type: `new` or `returning`.

No stable user identifier is required. The combination forms a deterministic segment key for the MVP policy.

## Outcomes And Rewards

Each decision receives at most one terminal outcome:

| Outcome | Reward |
| --- | ---: |
| `ignored` | 0.00 |
| `clicked` | 0.25 |
| `converted` | 1.00 |

The server derives reward from the outcome; clients cannot submit arbitrary reward values.

## Success Criteria

- A fresh clone starts through Docker Compose using documented commands.
- The same seed and configuration produce the same synthetic sequence.
- Every decision records policy name, policy version, selected offer, full action distribution, propensity, and context.
- Duplicate outcome submissions are idempotent.
- The adaptive policy can be compared with the random baseline without unsupported causal claims.
- `go test -race ./...` passes.
- The dashboard remains usable at desktop and mobile widths.

## Portfolio Story

The project should demonstrate Go concurrency and API design, online machine learning, exploration versus exploitation, unbiased logging for OPE, production-oriented observability, SQL persistence, testing, and responsible feature selection.
