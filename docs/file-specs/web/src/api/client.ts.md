# File Spec: `web/src/api/client.ts`

## Status

`specified`

## Purpose

Provide a typed, abortable browser client for every dashboard API operation.

## Depends On

- API types and OpenAPI contract.

## Public Surface

- `ApiClient`, typed `ApiProblemError`, and methods for health, experiments, demo creation, summary/feed, and simulation lifecycle.

## Required Behavior

- Resolve known paths against an explicitly configured absolute `VITE_API_BASE_URL`; when empty or absent, use browser same-origin. Local development reaches port 8080 through the Vite proxy, never a production localhost fallback.
- Set/accept JSON content types, pass `AbortSignal`, bound error-body reads, and reject unexpected statuses/content types.
- Parse problem JSON into a stable error; retain request ID.
- Encode query parameters through `URLSearchParams` and never interpolate raw input into paths without UUID validation at call sites.
- Do not automatically retry writes.

## Failure Cases

- Network/abort, malformed JSON, problem response, unexpected status/content type, or oversized error response.

## Non-Goals

- Polling, caching, toast/UI behavior, or policy calculations.

## Validation

- Mock-fetch tests through `App.test.tsx` or dedicated cases in that file for success/problem/abort.

## Completion Checklist

- [ ] Every dashboard operation is typed and abortable.
- [ ] Problems retain safe diagnostics.
- [ ] No hidden write retries occur.
