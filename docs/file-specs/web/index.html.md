# File Spec: `web/index.html`

## Status

`specified`

## Purpose

Provide the minimal accessible HTML shell for the React dashboard.

## Depends On

- `web/src/main.tsx`.

## Public Surface

- Document metadata, root mount element, and module entry.

## Required Behavior

- Set language to English, UTF-8, responsive viewport, concise title/description, theme color, and root element.
- Load only the Vite module entry; fonts come from npm imports.
- Include a useful no-script message.

## Failure Cases

- Inline application scripts/styles, remote trackers/fonts, misleading SEO claims, or missing viewport/language.

## Non-Goals

- Marketing content, loading splash, or server-rendered data.

## Validation

- Static HTML parse/check confirming metadata, root, no remote assets, and the exact module entry.
- Production build and browser accessibility smoke check after Phase 6 source exists.

## Completion Checklist

- [ ] Metadata and mount point are valid.
- [ ] No remote dependencies or trackers exist.
- [ ] App boots from the module entry.
