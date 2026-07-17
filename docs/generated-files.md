# Generated Files

Generated artifacts do not receive per-file implementation specs because tools own their exact content. They must never be edited manually.

| File | Generator | Validation |
| --- | --- | --- |
| `go.sum` | `go mod tidy` | `go mod verify` |
| `web/package-lock.json` | `npm install` | `npm ci --prefix web` |
| `web/dist/**` | `npm --prefix web run build` | Frontend production build |
| Coverage files | Go/Vitest test commands | Ignored by Git |

If a new generated artifact is required, add it here and to `.gitignore` where appropriate before generation.
