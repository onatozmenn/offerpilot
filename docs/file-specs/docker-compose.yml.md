# File Spec: `docker-compose.yml`

## Status

`specified`

## Purpose

Provide one-command local PostgreSQL, API, and web startup.

## Depends On

- Root and web Dockerfiles.
- `.env.example`.
- Health endpoints.

## Public Surface

- Services `db`, `api`, and `web`; one named PostgreSQL volume; explicit network.

## Required Behavior

- Pin PostgreSQL major version and define a real health check.
- Start API only after database health, and web only after API readiness where supported.
- Publish only web, API, and optionally local PostgreSQL ports with localhost bindings.
- Use environment substitution without committing secrets.
- Add restart behavior suitable for local development and bounded health retries.

## Failure Cases

- Implicit latest tags, publicly bound database, hard-coded production credentials, or dependency order without health conditions.

## Non-Goals

- Production orchestration or horizontal scaling.

## Validation

- `docker compose config`
- `docker compose up --build --wait`
- End-to-end smoke steps from `docs/09-testing.md`.

## Completion Checklist

- [ ] Compose config validates.
- [ ] All services become healthy from a clean volume.
- [ ] Local ports and credentials are documented.
