# FUI v0.1 (MVP) — development plan

Work is split in waves that follow the hexagonal dependency order (domain → app ports → adapters → composition).
Tasks in the same wave run in parallel, each in its own git worktree/branch, and are merged into `main` with
rebase by the coordinator at the end of the wave. Task files for later waves are written when the wave starts,
so they reference the real code merged so far.

| Wave | Task | Branch | Owned paths | Agent / model |
| --- | --- | --- | --- | --- |
| 1 | T01 Domain value types and entities | `feat/domain-core` | `internal/domain/` | Claude Code · sonnet · effort high |
| 1 | T02 fui/1 protocol | `feat/proto-fui1` | `pkg/fuiproto/` | Copilot CLI · gpt-5.3-codex · effort high |
| 1 | T03 Config, clock, logging, backup files, ADRs | `feat/platform` | `internal/config/`, `internal/clock/`, `internal/logging/`, `internal/backup/`, `docs/adr/` | Antigravity · gemini-3.8-flash-high |
| 2 | T04 App ports and core use cases (Plan/Apply) | `feat/app-core` | `internal/app/` (root), `internal/testutil/memstore/` | Claude Code · sonnet · high |
| 2 | T05 Domain services (balance, dedup, transfer match, runway, reports) | `feat/domain-services` | `internal/domain/` (new files), `internal/app/reporting/` | Copilot CLI · gpt-5.3-codex · high |
| 2 | T06 Revolut and Widiba connectors | `feat/connectors-bank` | `cmd/fui-connector-*/`, `tools/anonymize/` | Antigravity · gemini-3.8-flash-high |
| 3 | T07 SQLite store, migrations, sqlc, backup/restore | `feat/store-sqlite` | `internal/store/` | Copilot CLI · gpt-5.3-codex · high |
| 3 | T08 Import/export pipeline, connector runner, CSV connector | `feat/import-pipeline` | `internal/app/importing/`, `internal/connector/` | Claude Code · sonnet · high |
| 3 | T09 CLI (cobra, renderers, errors, agent mode, help --json) | `feat/cli` | `internal/cli/` | Copilot CLI · claude-sonnet-5 · high |
| 3 | T10 TUI (Dashboard, Transactions, Accounts) | `feat/tui` | `internal/tui/` | Antigravity · gemini-3.8-flash-high |
| 4 | T11 Composition root, e2e testscript UC-01..06/10, skill, CI | `feat/integration` | `cmd/fui/`, `internal/skill/`, `testdata/`, `.github/` | Claude Code · sonnet · high |
| 5 | Review by coordinator, fixes sent back to the owning agent | — | — | — |

Model policy: no model above effort `high`, no premium/token-heavy models (Opus/Fable class excluded).

## Out of scope for this iteration

v0.2/v0.3 features, goreleaser/Homebrew/cosign release pipeline (only a CI workflow is added), mutation testing,
nightly jobs.
