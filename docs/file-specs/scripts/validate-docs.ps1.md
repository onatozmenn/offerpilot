# File Spec: `scripts/validate-docs.ps1`

## Status

`specified`

## Purpose

Enforce Markdown links, one instruction file, required spec structure, and one-to-one manifest/spec mapping in local development and CI.

## Depends On

- `docs/13-file-manifest.md`, file-spec template, generated-files list, and syntax shared by Windows PowerShell 5.1 and PowerShell 7.

## Public Surface

- Non-interactive script compatible with Windows PowerShell 5.1 and PowerShell 7, with repository-root discovery and process exit code.

## Required Behavior

- Resolve root from `$PSScriptRoot`, not current directory; avoid APIs unavailable in Windows PowerShell 5.1.
- Check local Markdown links, H1 presence, and exactly one `AGENTS.md`/`copilot-instructions.md` instruction file.
- For every spec except its README, parse the planned path from the exact `# File Spec: <planned path>` H1 and verify mandatory sections and status.
- Parse manifest rows, reject duplicate planned/spec paths, verify links/existence/H1/status, and ensure every spec appears exactly once.
- Verify generated files have no manual specs/manifest rows and report all errors before exiting non-zero.
- Produce concise counts on success without printing file contents or secrets.

## Failure Cases

- Running outside root, malformed manifest, broken encoded path, duplicate mapping, missing section, status drift, or inaccessible file.

## Non-Goals

- Markdown style linting, code tests, or modifying files automatically.

## Validation

- Run from repository root and a nested directory.
- Temporarily introduce one controlled broken fixture in a test copy and confirm non-zero exit.

## Completion Checklist

- [ ] All documented invariants are enforced cross-platform.
- [ ] Errors are aggregated and actionable.
- [ ] CI calls the script directly.
