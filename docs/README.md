# OfferPilot Documentation

This directory is the implementation contract for OfferPilot. Documents are ordered by the sequence in which a new contributor or coding agent should read them.

## Onboarding Order

1. [Product brief](01-product-brief.md)
2. [Architecture](02-architecture.md)
3. [Domain model](03-domain-model.md)
4. [API contract](04-api-contract.md)
5. [Online-learning design](05-online-learning.md)
6. [Data model](06-data-model.md)
7. [Security and privacy](07-security-privacy.md)
8. [Observability](08-observability.md)
9. [Testing strategy](09-testing.md)
10. [Frontend design](10-frontend.md)
11. [Delivery plan](11-delivery-plan.md)
12. [Definition of done](12-definition-of-done.md)
13. [File manifest](13-file-manifest.md) and [per-file specs](file-specs/README.md)
14. [Coding-agent playbook](14-coding-agent-playbook.md)

## Change Discipline

- Architectural changes require an Architecture Decision Record in `docs/decisions/`.
- A source or configuration file may be created only when a matching document exists under `docs/file-specs/`.
- Generated files such as dependency lock files are documented separately and are never edited manually.
- File specs describe responsibility, public surface, dependencies, behavior, failure cases, tests, and completion criteria.

See [the file-spec template](file-spec-template.md) for the mandatory structure.

## Vocabulary

- **Context:** Privacy-safe attributes describing the current anonymous session.
- **Offer:** A fictional marketplace promotion eligible for selection.
- **Decision:** A policy's selection of one offer from an eligible set.
- **Outcome:** The terminal feedback observed for a decision.
- **Reward:** A bounded numeric value derived from the outcome.
- **Policy:** An algorithm that returns an offer and a probability distribution.
- **Propensity:** The probability with which the selected offer was chosen.
- **OPE:** Off-policy evaluation using logged decisions and propensities.
