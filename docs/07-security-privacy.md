# Security And Privacy

## Threat Model

The MVP is a public portfolio application that accepts untrusted HTTP input and stores synthetic behavioral events. It does not process credentials, payments, lending data, or real user identities, but it must still resist injection, resource exhaustion, accidental secret exposure, and misleading AI behavior.

## Data Minimization

- Accept only the four enumerated context features in the product brief.
- Do not accept names, email addresses, IP-derived location, stable device IDs, free-form profile text, credit attributes, or demographic data.
- Do not place raw request headers or bodies in logs.
- Use fictional merchant names and generated UUIDs in demo fixtures.

## HTTP Controls

- Bind to loopback by default outside containers.
- Configure explicit CORS origins; never combine credentials with wildcard origins.
- Limit request bodies to 1 MiB and simulation rates to documented bounds.
- Set read-header, read, write, and idle timeouts.
- Reject unknown JSON fields and trailing JSON values.
- Validate content type for write requests.
- Return generic database/internal errors with request IDs; log redacted details server-side.

## Persistence Controls

- Use parameterized SQL through `pgx`; never concatenate input into SQL.
- Run PostgreSQL with a dedicated least-privilege application user.
- Keep database passwords in environment variables and out of logs.
- Do not expose PostgreSQL publicly in hosted environments.

## Dependency And Supply Chain

- Pin Go module and npm lockfile versions.
- Run Go vulnerability scanning and npm audit in CI with a documented severity policy.
- Use minimal, non-root runtime images.
- Keep generated frontend assets free of secrets; only `VITE_` public configuration may enter the browser build.

## Responsible Personalization

- The system ranks fictional offers only; it never changes price, eligibility, credit, or payment terms.
- Protected and sensitive attributes are prohibited even in synthetic examples.
- Exploration rate, action probabilities, and policy version are visible for audit.
- Frequency caps and diversity constraints are roadmap safety features and must precede any claim of real-world suitability.
- Dashboard copy must label benchmarks and uplift as simulated.

## Secrets

The MVP needs only database credentials. `.env.example` contains placeholders. `.env` is ignored. CI uses repository secrets only if a hosted deployment is later added.

## Security Validation

- Tests cover malformed JSON, oversized bodies, unknown fields, invalid enums, invalid UUIDs, duplicate outcomes, and forbidden CORS origins.
- CI checks committed files for obvious credential patterns without scanning ignored local secret files into model-visible output.
- Container runs as non-root with a read-only filesystem where practical.
