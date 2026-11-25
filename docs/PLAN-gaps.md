# FUI v0.5 — closing the v0.2/v0.3 gaps

Baseline: `develop` after v0.4.0 (migrations 0001–0015). Source: a spec-vs-code audit (2026-11-25) of every v0.2 and
v0.3 requirement in `docs/specs/funzionali.md` (`F:`) and `docs/specs/tecniche.md` (`T:`), plus the "Known gaps" of
`PLAN-v0.2.md` and `PLAN-v0.3.md`. 51 gaps, grouped into 9 epics (G01–G09) and a final integration step (G10).

Out of scope: `ai` module, Monte Carlo, FIFO cost basis, cross-currency cash legs for security operations,
Bubble Tea v2, anything v0.6+.

## 1. Team and flow

| Role | Who | Does |
| --- | --- | --- |
| Architect | Claude Code (this session, herdr tab 1, agent name `architect`) | Plan, epic briefs, product answers, PR review, merge, final tests, fix tasks |
| Team Lead (TL) | Claude Code `--model sonnet --effort medium --permission-mode auto`, one herdr **tab** per epic | Owns one epic branch: splits it into mini tasks (MT), briefs devs, reviews, integrates, opens the PR |
| Developer (dev) | Copilot CLI `--allow-all-tools`, `claude-sonnet-5.5` effort medium (low for trivial MTs), one herdr **pane** in the TL tab | Executes one MT in its own worktree, reports with one line |

Limits: **≤ 2 TLs at the same time, ≤ 2 devs per TL at the same time.** Epics start from a rolling queue (§4).

1. Architect writes this plan and the epic briefs (`docs/tasks/G<nn>-*.md`), merges them into `develop`.
2. Architect opens a TL tab per epic (worktree `../fui-wt/g<nn>` on `feat/g<nn>-<slug>` from `develop`).
3. TL reads its brief, splits it into MTs (cards, `_PROTOCOL.md` §TL/dev), runs ≤ 2 devs in panes, each on a
   sub-branch `feat/g<nn>-<slug>-mt<k>` in its own worktree `../fui-wt/g<nn>-mt<k>`, branched from the epic branch.
4. Dev delivers → TL reviews (`F<n>` findings until clean) → TL merges the MT branch into the epic branch.
5. Epic done → TL runs the full verification, rebases on `origin/develop`, pushes, opens the PR to `develop`,
   notifies the Architect.
6. Architect reviews the PR (comments with `F<n>` findings or approves), merges approved PRs with squash in queue
   order (dependent epics need their base merged), and after the last merge runs G10 (full tests, e2e, manual
   sandboxed run). Failures become fix tasks for the owning TL.

Decision rights: TLs decide technical/implementation matters only (package layout, names of unexported symbols,
test strategy, MT split). Anything about product behaviour, CLI command/flag names not fixed in the brief, TUI keys,
output formats or spec interpretation goes to the Architect (`EVT=question`). The Architect asks the user only when a
decision changes existing behaviour.

## 2. Product decisions (user-confirmed 2026-11-25 where marked ★)

| # | Decision |
| --- | --- |
| P1 ★ | Recurrences and loan installments are registered automatically as `pending` at startup (CLI and TUI), idempotent, only for active modules; `recurring run --until` / `loan run` stay to anticipate. Skipped for `help`, `completion`, `__complete`, `init`, `reset`, `restore`, `backup`, `version`, and when the archive is missing/locked. |
| P2 ★ | Cap carry-over defaults to on for new budgets, `--no-carry` (and a TUI toggle) turns it off; existing budgets keep their stored value. |
| P3 ★ | Undo covers every transaction write: add, edit, rm, transfer, split/unsplit, categorize, tag/untag, rules apply, recurring confirm, loan confirm. The stale check compares only the fields the entry wrote; restoring a row whose payee/batch/recurrence is gone clears that reference and warns. TUI gets undo. |
| P4 ★ | Widiba "Rimborso Titoli" stays a transaction of kind `investment`; the spec is updated (G10). |
| P5 | Forecast: default horizon 6 months (`forecast.horizon = "6m"`), `--horizon` accepts `Nd`, `Nw`, `Nm` (bare number = days, backward compatible), `--granularity day\|week`, config key `forecast.min_balance`. |
| P6 | With the investments module off, `investment` accounts (and with crypto off, `wallet` accounts) are excluded from net worth, runway, reports and dashboard, as F "Effetti di un modulo disattivato" requires. Account lists still show them. |
| P7 | New account kind `wallet` (crypto module). Security prices keep at least 8 decimal digits of precision (integers only, no floats). |
| P8 | Budget period `month` or `year`; an annual envelope shows the suggested monthly quota (remaining / remaining months). |
| P9 | Budget thresholds are per budget (`--warn`, `--over`, default 80/100), shown and editable in the TUI. |
| P10 | Reconciliation keeps today's bulk mode and adds per-transaction ticking (`--tick`/`--untick`, TUI `space`); closing requires a zero difference over the ticked set when ticks are used. |
| P11 | Reports share one filter set (the existing `domain.TxFilter`: accounts, categories, tags, payees, amount range, status, text) plus `--group-by category\|tag\|payee\|account\|week\|month\|year` and `--granularity` where a series exists; saved reports store filter, grouping and relative period in `params` JSON (no migration). |
| P12 | `reset --keep-settings` also keeps connector associations: they are re-linked by account name at the next import. |
| P13 | `export --all` writes every entity plus config; `fui restore FILE.json` rebuilds the archive from such a dump (backup first, atomic like reset). |
| P14 | Import preview (CLI and TUI) shows warnings, new categories/tags/payees, transfer proposals (acceptable: CLI `--accept-transfers` or an interactive prompt, TUI toggle), balance mismatches, and records of disabled modules with "enable module or skip" (CLI prompt; non-interactive: `--enable-modules` or skipped with a warning). |
| P15 | CLI accepts the spec examples as written: `loan event NAME --rate X --from DATE`, `rule test "TEXT"` (account/amount optional), `budget set NAME AMOUNT [--month]` creating the budget when missing, `forecast --horizon 12m`. |
| P16 | Period comparison accepts `YYYY-MM`, `YYYY-Qn`, `YYYY` and the relative periods, the P11 filters, can be saved (`report save NAME compare`). All net-worth reports use `NetWorthReport`. |

## 3. Epics

Sizes: L ≈ 1500–3000 changed lines, split by the TL into MTs of ≤ ~300 lines and one layer each.

| Epic | Title | Gaps closed (audit ids: `a2-`=v0.2, `a3-`=v0.3) | Migration | Deps |
| --- | --- | --- | --- | --- |
| G01 | Budget complete | a2-2,3,11,12,13,14,15; P2, P8, P9, P15 (budget set) | yes | — |
| G02 | Import pipeline and preview | a2-1,4,7,25,28,30,33; a3-13,18; P14 | yes | — |
| G03 | Loans through import/export | a2-8,9,10 | maybe | G02 |
| G04 | Transactions, recurrences, reconciliation, CLI/TUI parity | a2-5,24,26,27,29,31,32; P1, P10, P15 (rest) | yes | — |
| G05 | Undo for every transaction write | a3-5; P3 | maybe | G04 |
| G06 | Reports and forecast | a2-16..22; a3-1,15,16; P5, P6, P11, P16 | no | — |
| G07 | Full dump, restore, reset | a2-6,23; a3-2; P12, P13 | maybe | G01–G05, G08 |
| G08 | Investments, crypto, assets | a3-6,7,10,11,12; P7 | yes | — |
| G09 | Insights, simulations, connectors | a3-3,4,8,9,17 | no | — |
| G10 | Integration, specs, release v0.5.0 (Architect) | spec/ADR updates for P1–P16, e2e, CHANGELOG | — | all |

## 4. Schedule (rolling queue, 2 TL slots)

Queue order: **G01, G02, G04, G03, G06, G05, G08, G09, G07**. When a slot frees, the first epic in the queue whose
deps are merged starts. Expected pairs: (G01 ‖ G02) → (G04 ‖ G03) → (G06 ‖ G05) → (G08 ‖ G09) → G07 → G10.
Pairs were chosen to keep owned paths disjoint (§5).

## 5. Conflict rules

- **Migrations**: number = next free number on `develop` at rebase time. The Architect merges one PR at a time; the
  second TL renumbers on rebase and regenerates `schema.sql` / sqlc with `go generate ./internal/store/...`
  (never hand-merge generated files).
- **Shared files** (`internal/cli/root.go`, `cmd/fui/*`, `internal/tui/root.go`, `help --json` goldens, TUI
  goldens, `internal/config/defaults.go`): any epic may edit them; before the PR the TL rebases on `origin/develop`
  and regenerates goldens (`go test ./... -update` in the affected packages). Before merging, the Architect runs
  `gh pr update-branch` and waits for CI.
- Specs and ADRs are updated only in G10 (TLs list deviations in the PR body).
- `internal/app/importing` belongs to G02 then G03 (sequential); G07 owns `importing/dump*.go` and `restore`.

## 6. Definition of Done (every epic)

1. Every gap listed in the brief is closed, or explicitly moved to "Known gaps" with the Architect's OK.
2. Each new behaviour has tests: table-driven, `rapid` properties where the brief says so, testscript e2e in
   `cmd/fui/testdata` for every new/changed command, teatest/golden for every TUI change (80×24, all themes +
   `NO_COLOR`); bugs have a regression test.
3. Green on the whole repo: `gofumpt -l .` (empty), `go build ./...`, `go vet ./...`,
   `go test -race -count=1 ./...` with `TZ=UTC` and `TZ=Pacific/Kiritimati`, `golangci-lint run ./...`; coverage
   total ≥ 80%, `internal/domain` ≥ 90%, every touched package ≥ 80%.
4. CLI: `--json`/`--csv` where applicable, `--dry-run` on writes, module annotation, examples, `help --json` golden.
5. No commit trailer `Co-authored-by` and no AI mention in any commit on the pushed branch.
6. PR to `develop`, Conventional Commits title, body: gaps closed, verification, deviations, known gaps; CI green.

## 7. Status

| Epic | TL | Branch | PR | Status |
| --- | --- | --- | --- | --- |
| G01 | — | `feat/g01-budget` | — | queued |
| G02 | — | `feat/g02-import` | — | queued |
| G03 | — | `feat/g03-loan-import` | — | queued |
| G04 | — | `feat/g04-tx-recur` | — | queued |
| G05 | — | `feat/g05-undo` | — | queued |
| G06 | — | `feat/g06-reports` | — | queued |
| G07 | — | `feat/g07-dump` | — | queued |
| G08 | — | `feat/g08-invest` | — | queued |
| G09 | — | `feat/g09-insight-sim` | — | queued |
