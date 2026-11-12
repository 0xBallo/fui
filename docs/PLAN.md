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
| 2 | T06 Revolut and Widiba connectors | `feat/connectors-bank` | `cmd/fui-connector-*/`, `tools/anonymize/` | Copilot CLI · gpt-5.3-codex · high (was Antigravity, quota exhausted) |
| 3 | T07 SQLite store, migrations, sqlc, backup/restore | `feat/store-sqlite` | `internal/store/` | Copilot CLI · gpt-5.3-codex · high |
| 3 | T08 Import/export pipeline, connector runner, CSV connector | `feat/import-pipeline` | `internal/app/importing/`, `internal/connector/` | Claude Code · sonnet · high |
| 3 | T09 CLI (cobra, renderers, errors, agent mode, help --json) | `feat/cli` | `internal/cli/` | Copilot CLI · claude-sonnet-5 · high |
| 3 | T10 TUI (Dashboard, Transactions, Accounts) | `feat/tui` | `internal/tui/` | Copilot CLI · claude-sonnet-5 · high (was Antigravity, quota exhausted) |
| 4 | T11 Composition root, e2e testscript UC-01..06/10, skill, CI | `feat/integration` | `cmd/fui/`, `internal/skill/`, `testdata/`, `.github/` | Claude Code · sonnet · high |
| 5 | Review by coordinator, fixes sent back to the owning agent | — | — | — |

Model policy: no model above effort `high`, no premium/token-heavy models (Opus/Fable class excluded).

## Out of scope for this iteration

v0.2/v0.3 features, goreleaser/Homebrew/cosign release pipeline (only a CI workflow is added), mutation testing,
nightly jobs.

## Follow-up tasks (added during the run)

| Wave | Task | Branch | Agent / model |
| --- | --- | --- | --- |
| 4 | T08b Import/export CLI commands, account IBAN (migration 0002) | `feat/import-cli` | Claude Code · sonnet · high (same agent as T08) |
| 4 | T11 Composition root, e2e testscript, skill, CI | `feat/integration` | Claude Code · sonnet · high |

## MVP completion for specs v1.2 (waves 6–9)

Specs v1.2 add to the MVP: module activation, transaction search with filters, `fui reset`, Catppuccin TUI themes.
The CI on `main` was red (lint action, excelize CVEs, Go 1.26 golden, perf test under `-race`) and had no coverage
gate (spec: ≥ 80% total, ≥ 90% domain). Workers follow `docs/tasks/_PROTOCOL.md`. Model policy for these waves:
effort ≤ medium, no flagship models.

| Wave | Task | Branch | Owned paths (summary) | Depends on | Agent / model |
| --- | --- | --- | --- | --- | --- |
| 6 | T12 CI green + coverage gate | `fix/ci` | `.github/`, `Makefile`, `go.mod`, widiba, connector perf test | — | Copilot · claude-sonnet-5 · medium |
| 6 | T13 Module registry, `modules.enabled`, enable/disable use cases | `feat/module-core` | `internal/module/`, `internal/config/`, `internal/app/module.go` | — | Copilot · gpt-5.3-codex · medium |
| 6 | T15 Domain `TxFilter` + `Match`, `textnorm`, domain coverage ≥ 90% | `feat/txfilter-domain` | `internal/domain/txfilter.go`, `internal/textnorm/` | — | Antigravity · gemini-3.8-flash-medium (fallback Copilot) |
| 6 | T18 `fui reset` (backup reason, store rebuild, use case, CLI) | `feat/reset` | `internal/backup/`, `store/sqlite/reset.go`, `app/reset.go`, `cli/reset.go` | — | Claude Code · sonnet · medium |
| 6 | T19 Catppuccin themes, semantic tokens, background detection | `feat/tui-themes` | `internal/tui/theme/`, `internal/config/` (theme keys) | — | Copilot · claude-sonnet-5 · medium |
| 7 | T14 CLI `fui module`, `init --preset/--modules`, command annotations, `module_disabled` | `feat/module-cli` | `internal/cli/` (module, init, root hook, help --json) | T13 | Copilot · gpt-5.3-codex · medium |
| 7 | T16 Store search: migration 0003 (`search_text`, FTS5 trigram), port on `domain.TxFilter`, memstore, SQL-vs-`Match` property, 100k benchmark | `feat/search-store` | `internal/store/`, `internal/app/ports.go`, `internal/app/transaction.go`, `internal/testutil/memstore/` | T15 | Claude Code · sonnet · medium |
| 8 | T17 CLI filter flags (`filterflags.go`), `tx search`, export filters, summary row | `feat/search-cli` | `internal/cli/` (tx, export, filterflags) | T16 | Copilot · claude-sonnet-5 · medium |
| 8 | T20 TUI live search `/` (150 ms debounce + cancel) and filter panel `f` | `feat/search-tui` | `internal/tui/screens/transactions/`, `internal/tui/components/` | T16, T19 | Copilot · claude-sonnet-5 · medium |
| 9 | T21 Integration: e2e testscripts (search, reset, module matrix + `help --json` per preset), skill reads `module list`, ADR 0009–0012, coverage top-up | `feat/mvp-integration` | `cmd/fui/`, `internal/skill/`, `docs/adr/`, tests anywhere | all | Claude Code · sonnet · medium |
| 10 | Coordinator review of each branch, fixes sent back, merge, CI green on GitHub | — | — | — | — |

Task files for waves 7–9 are written when the wave starts. The v0.2 plan is in `docs/PLAN-v0.2.md`.
