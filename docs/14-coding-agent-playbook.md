# Coding-Agent Playbook

## Objective

Implement OfferPilot without reopening settled architecture or creating undocumented files. The file manifest is the queue, file specs are acceptance contracts, and focused tests decide completion.

## Required Reading

Before the first implementation task, read:

1. Root `AGENTS.md`.
2. `docs/01-product-brief.md` through `docs/12-definition-of-done.md`.
3. Relevant ADRs under `docs/decisions/`.
4. `docs/13-file-manifest.md`.
5. The selected file's spec and every dependency it names.

Later tasks may reread only the controlling documents and nearby specs, but must not rely on memory when a contract is available.

## Task Loop

1. Choose the first dependency-unblocked `specified` row in the current phase.
2. Change that manifest row and spec status to `in_progress`.
3. Implement only that file's responsibility. Tool-generated lock files may appear as documented side effects.
4. Run the cheapest focused validation listed in the spec immediately.
5. If it fails, fix the same slice and rerun before opening another implementation file.
6. Set status to `implemented` when behavior exists, then run all completion validation.
7. Set both statuses to `validated` only after validation passes.
8. Report files changed, commands/results, assumptions, and the next unblocked manifest ID.

Shared manifests/contracts such as `go.mod`, `web/package.json`, OpenAPI, environment examples, and CI may legitimately evolve when a later primary file introduces their documented dependency. Set the shared file/spec back to `in_progress`, include it in the same focused slice, and revalidate it before restoring `validated`; do not add dependencies in advance merely to avoid reopening it.

## First Implementation Sequence

Start with:

1. `F001` `.gitignore`.
2. `F006` documentation validator, using the already proven terminal checks as behavior references.
3. `F003` license and `F002` environment example.
4. `F004` Go module and `F005` linter configuration.
5. Frontend package/config shell `F007` through `F011` without claiming a production build before Phase 6 source exists.
6. Phase 2 domain and policy core in manifest order.

Do not start Docker, HTTP handlers, or React feature components before their domain/service dependencies exist.

## Change Requests

When implementation reveals a contract defect:

1. Stop the implementation slice.
2. Identify the controlling product, architecture, API, data, or ADR document.
3. Propose the smallest contract change and its consequences.
4. Update the contract, affected specs, and manifest before code.
5. Rerun documentation validation.

Never silently make code disagree with a spec.

## Handoff Template

```text
Manifest ID: C008
Status: validated
Implemented: internal/bandit/epsilon_greedy.go
Contract read: docs/05-online-learning.md, ADR-002, matching file spec
Validation: go test -run TestEpsilonGreedy ./internal/bandit; go test -race ./internal/bandit
Result: pass
Assumptions/limitations: none
Next unblocked ID: C009
```

## Starter Prompt

```text
Read AGENTS.md and docs/14-coding-agent-playbook.md. Validate docs first. Then implement only the first dependency-unblocked `specified` item in docs/13-file-manifest.md. Read its per-file spec and dependencies, update statuses together, run the focused validation immediately after the edit, and stop after reporting the validated slice and next manifest ID.
```

## Guardrails

- Never introduce real merchant/customer data.
- Never add a protected context feature, credit decision, payment term, or price optimization.
- Never claim uplift from synthetic runs as real business impact.
- Never add a service, queue, framework, algorithm, or file because it is fashionable; require a measured need and contract update.
- Never mark validation complete when a command was unavailable or skipped; record the blocker instead.
