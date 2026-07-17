# Per-File Specifications

Every manually authored implementation or configuration file planned for OfferPilot has one matching Markdown specification in this directory. The spec path normally mirrors the planned path and appends `.md`.

Example:

```text
Planned file: internal/bandit/policy.go
Specification: docs/file-specs/internal/bandit/policy.go.md
```

The manifest is authoritative for exceptional spec names. In particular, Dockerfile specs are named `root-container.md` and `web-container.md` because VS Code otherwise classifies `Dockerfile.md` as Docker source instead of Markdown. The exact planned path always appears in the spec H1 as `# File Spec: <planned path>`.

## Agent Protocol

1. Locate the target in `docs/13-file-manifest.md`.
2. Read the matching spec and listed dependencies.
3. Implement only the described responsibility.
4. Run focused validation immediately.
5. Change the manifest status only after validation succeeds.

Specs are not implementation suggestions. They are acceptance contracts. If a spec is wrong or incomplete, update the governing architecture document and ADR before changing implementation scope.
