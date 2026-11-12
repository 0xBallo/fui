# FUI v0.2 — development plan

Same method as `docs/PLAN.md`: waves follow the hexagonal order (domain → store → app → cli → tui → integration); tasks in a
wave run in parallel in their own worktree/branch and are rebased into `main` by the coordinator at the end of the wave.
Task files `docs/tasks/V<nn>.md` are written when the wave starts. Specs (Italian) stay the source of truth; sections
below are cited as `F:` (funzionali.md) and `T:` (tecniche.md).

Baseline assumed from the MVP: module registry `internal/module` (+ `module.Set`, `Require`), `domain.TxFilter` + FTS
search, `fui reset`, Catppuccin themes, `fui kpi networth` (exists: `internal/cli/kpi_networth.go`), static shell
completion (exists: `internal/cli/completion.go`, bash/zsh/fish), migrations 0001-0003.

## 1. Scope

| Item | Module | Spec |
| --- | --- | --- |
| Tags (CRUD, on transactions, `-t` filter, tag budgets) and payees/aliases | core | F: Modello di dominio, CLI `fui tag`; T: Modello dati |
| Splits (rows sum to the amount, count per row in reports/budgets) | core | F: Modello di dominio, Report; T: Invarianti |
| Categorization rules (`rule add/list/test/apply`, hook in import step 4) | core | F: CLI, Flusso di import; T: Modello dati (`rules`) |
| Reconciliation (`fui reconcile`, status `reconciled`, edit confirmation) | core | F: Import, export e riconciliazione |
| Budget: caps and envelopes (`budget.mode`), carry-over, moves, thresholds | budget | F: Budget; T: Invarianti (envelopes) |
| Recurrences (`recurring add/list/run/skip/confirm`, pending tx, import match) | recurring | F: UC-08, Modello di dominio |
| Loans/mortgages: versioned amortization, events, installment split | loans | F: Mutui e prestiti; T: Invarianti (amortization) |
| Cash forecast (`fui forecast`, `forecast.horizon`) | analytics | F: Previsione di cassa |
| Net worth: report over time + loan debt as negative. `fui kpi networth` already exists; v0.2 only adds the report and loans | analytics | F: Report |
| Reports: budget vs actual, balance over time, remaining debt, interest paid, net worth, top payees; saved reports (`report save/list/run`) | analytics | F: Report e analytics |
| TUI screens: Budget, Report (incl. forecast, saved), Import, Settings (modules, theme, categories, tags, rules, recurrences, import profiles) | per module | F: TUI |
| Shell completion: keep static scripts, add dynamic completion of account/category/tag/payee/loan/report names | core | F: Requisiti trasversali |

Out of scope (v0.3): OFX/CAMT.053 connectors, investments/crypto/assets, automatic insights, simulations, period
comparison report, generic undo, AI features, sync/web/mobile.

## 2. Schema plan (forward-only migrations, one per task)

MVP ends at 0003 (assumed: FTS/`search_text`, modules). If `payees`/`tags` already exist there, V07 drops those tables from 0004.

| Migration | Owner | Tables / changes |
| --- | --- | --- |
| `0004_tags_splits.sql` | V07 | `tags` (name UNIQUE NOCASE), `transaction_tags`, `transaction_splits` (tx, category, amount, note), `payees`, `payee_aliases` + FK from `transactions.payee_id` |
| `0005_rules.sql` | V08 | `rules` (priority, conditions/actions JSON validated, enabled) |
| `0006_budget.sql` | V09 | `budgets` (mode, period, name), `budget_targets` (budget → category or tag), `budget_amounts` (month override), `budget_moves` (envelope assign/move); one budget per category per period |
| `0007_recurring.sql` | V10 | `recurrences` (explicit rule columns), `transactions.recurrence_id` + `due_date` (nullable) |
| `0008_loans.sql` | V11 | `loans`, `loan_schedules` (`UNIQUE(loan_id, valid_from)`), `loan_installments` (links `transactions`), `loan_events` |
| `0009_reports_reconcile.sql` | V12 | `saved_reports`, `reconciliations` (account, date, statement balance, closed_at) |

Generated files (`schema.sql`, `sqlc/`) are regenerated after each rebase with `go generate`, never hand-merged.

## 3. Waves

Rules: one layer per task; tasks in the same wave own disjoint files; ≤ ~800 changed lines; no model above effort medium;
no opus/flagship. CLI Cobra registration lines and `internal/tui` screen registration are done only by V32, so wave 4/5
tasks never touch `root.go`/`register.go` (each exposes a `NewXCmd(deps)` / `screens.X` constructor).

| Wave | Task | Title | Branch | Owned paths | Depends on | Agent / model | Size |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | V01 | Domain: tag, payee, split | `feat/v-domain-split` | `internal/domain/{tag,payee,split}*.go` | — | Copilot · claude-sonnet-5 · medium | S |
| 1 | V02 | Domain: budget engine (caps, envelopes, carry-over) | `feat/v-domain-budget` | `internal/domain/budget*.go` | — | Claude Code · sonnet · medium | L |
| 1 | V03 | Domain: recurrence expansion, pending matching | `feat/v-domain-recur` | `internal/domain/recurrence*.go` | — | Copilot · gpt-5.3-codex · medium | M |
| 1 | V04 | Domain: loan amortization, events, installment split | `feat/v-domain-loan` | `internal/domain/loan*.go` | — | Claude Code · sonnet · medium | L |
| 1 | V05 | Domain: rule conditions/actions engine | `feat/v-domain-rule` | `internal/domain/rule*.go` | — | Copilot · gpt-5.3-codex · medium | M |
| 1 | V06 | Domain: reconciliation + forecast projection | `feat/v-domain-recon` | `internal/domain/{reconcile,forecast}*.go` | — | Copilot · gpt-5.3-codex · medium | M |
| 2 | V07 | Store: tags, splits, payees (0004) | `feat/v-store-split` | `internal/store/sqlite/{migrations/0004*,queries/split*,repo_tags*,repo_splits*,repo_payees*}` | V01 | Copilot · gpt-5.3-codex · medium | M |
| 2 | V08 | Store: rules (0005) | `feat/v-store-rule` | `…/migrations/0005*`, `queries/rule*`, `repo_rules*` | V05 | Antigravity · gemini-3.8-flash-medium | S |
| 2 | V09 | Store: budgets (0006) | `feat/v-store-budget` | `…/migrations/0006*`, `queries/budget*`, `repo_budgets*` | V02 | Copilot · claude-sonnet-5 · medium | M |
| 2 | V10 | Store: recurrences (0007) | `feat/v-store-recur` | `…/migrations/0007*`, `queries/recur*`, `repo_recurrences*` | V03 | Copilot · gpt-5.3-codex · medium | M |
| 2 | V11 | Store: loans (0008) | `feat/v-store-loan` | `…/migrations/0008*`, `queries/loan*`, `repo_loans*` | V04 | Copilot · gpt-5.3-codex · medium | M |
| 2 | V12 | Store: saved reports, reconciliations (0009) | `feat/v-store-report` | `…/migrations/0009*`, `queries/report*`, `repo_reports*`, `repo_reconcile*` | V06 | Antigravity · gemini-3.8-flash-medium | S |
| 3 | V13 | App: tags, payees, splits; split-aware filters | `feat/v-app-split` | `internal/app/tagging/`, `internal/testutil/memstore/split*` | V01, V07 | Copilot · claude-sonnet-5 · medium | M |
| 3 | V14 | App: rules use cases + import step 4 hook | `feat/v-app-rule` | `internal/app/rules/`, `internal/app/importing/rules*.go`, `memstore/rule*` | V05, V08 | Claude Code · sonnet · medium | M |
| 3 | V15 | App: budget use cases and status | `feat/v-app-budget` | `internal/app/budgeting/`, `memstore/budget*` | V02, V09, V13 | Claude Code · sonnet · medium | L |
| 3 | V16 | App: recurrences (run/skip/confirm, import match) | `feat/v-app-recur` | `internal/app/recurring/`, `internal/app/importing/recur*.go`, `memstore/recur*` | V03, V10 | Copilot · claude-sonnet-5 · medium | M |
| 3 | V17 | App: loans (schedule, events, confirm → transfer+interest) | `feat/v-app-loan` | `internal/app/lending/`, `memstore/loan*` | V04, V11, V16 | Claude Code · sonnet · medium | L |
| 3 | V18 | App: reconcile | `feat/v-app-recon` | `internal/app/reconciling/`, `memstore/reconcile*` | V06, V12 | Copilot · gpt-5.3-codex · medium | S |
| 4 | V19 | App: forecast + new reports + saved reports | `feat/v-app-analytics` | `internal/app/reporting/` (new files), `memstore/report*` | V06, V12, V15, V16, V17 | Claude Code · sonnet · medium | L |
| 4 | V20 | CLI: `tag`, `tx --split/-t`, `payee` | `feat/v-cli-split` | `internal/cli/{tag,payee}*.go`, `tx_split*.go` | V13 | Antigravity · gemini-3.8-flash-medium | S |
| 4 | V21 | CLI: `rule`, `reconcile` | `feat/v-cli-rule` | `internal/cli/{rule,reconcile}*.go`, `render/{rule,reconcile}.go` | V14, V18 | Copilot · gpt-5.3-codex · medium | M |
| 4 | V22 | CLI: `budget` (module budget) | `feat/v-cli-budget` | `internal/cli/budget*.go`, `render/budget.go` | V15 | Copilot · claude-sonnet-5 · medium | M |
| 4 | V23 | CLI: `recurring`, `loan` (modules recurring, loans) | `feat/v-cli-recur` | `internal/cli/{recurring,loan}*.go`, `render/{recurring,loan}.go` | V16, V17 | Copilot · claude-sonnet-5 · medium | M |
| 4 | V24 | CLI: `forecast`, `report save/list/run` extension, `kpi networth` with loans, `--chart` | `feat/v-cli-report` | `internal/cli/{forecast,report*,kpi_networth*}.go`, `render/{report,forecast,networth}.go` | V19 | Copilot · gpt-5.3-codex · medium | M |
| 4 | V25 | CLI: dynamic shell completion | `feat/v-cli-complete` | `internal/cli/completion*.go` | V13, V15, V16, V17 | Antigravity · gemini-3.8-flash-medium | S |
| 5 | V26 | TUI: Budget screen | `feat/v-tui-budget` | `internal/tui/screens/budget/` | V15 | Copilot · claude-sonnet-5 · medium | M |
| 5 | V27 | TUI: Report screen (reports, forecast, saved) | `feat/v-tui-report` | `internal/tui/screens/report/` | V19 | Copilot · claude-sonnet-5 · medium | M |
| 5 | V28 | TUI: Import screen (preview, mapping, duplicates, include/exclude) | `feat/v-tui-import` | `internal/tui/screens/importer/` | — (MVP import app) | Claude Code · sonnet · medium | L |
| 5 | V29 | TUI: Settings shell, modules, theme picker | `feat/v-tui-settings` | `internal/tui/screens/settings/{shell,modules,theme}*` | — | Copilot · claude-sonnet-5 · medium | M |
| 5 | V30 | TUI: Settings lists (categories, tags, rules, recurrences, import profiles) | `feat/v-tui-settings-crud` | `internal/tui/screens/settings/lists/` | V13, V14, V16, V29 | Copilot · gpt-5.3-codex · medium | M |
| 5 | V31 | TUI: split editor + reconcile action in Transactions/Accounts | `feat/v-tui-split` | `internal/tui/screens/{transactions,accounts}/` | V13, V18 | Copilot · claude-sonnet-5 · medium | M |
| 6 | V32 | Integration: wiring, registration per module, e2e, golden, skill, CI matrix | `feat/v-integration` | `cmd/fui/`, `internal/cli/root.go`, `internal/tui/register.go`, `internal/skill/`, `testdata/`, `.github/` | all | Claude Code · sonnet · medium | L |
| 7 | Review by coordinator; fixes go back to the owning agent | — | — | — | V32 | — | — |

## 4. Task briefs

All tasks: AGENTS.md workflow, domain purity, table-driven tests, `TZ=UTC` and `Pacific/Kiritimati`.

**V01** Entities `Tag`, `Payee` (alias normalization), `Split` with `ValidateSplits(total, rows)`. Accept: rapid —
split sum equals amount; round-trip of normalization. Read F: Modello di dominio; T: Invarianti.

**V02** Pure functions: cap status (spent, planned, remaining, %, thresholds 80/100), envelope ledger (assigned, moves,
"Da assegnare"), carry-over/overspill, `month_start_day` periods, one-budget-per-category check, pending = "planned" only,
transfers/loan principal excluded, splits counted per row. Accept: rapid — envelopes + Da assegnare = sum of included
liquidity balances; carry-over conservation. Read F: Budget.

**V03** `Recurrence` rule (N days/weeks/months, day-of-month clamp to month end), `Expand(from, until)`, matching a
booked tx to a pending one (amount tolerance, date window). Accept: table tests for month-end/leap years; rapid —
expansion is monotonic, idempotent for the same window. Read F: Modello di dominio (Ricorrenza), UC-08.

**V04** French amortization, fixed/variable, new schedule version from a date, partial prepayment (reduce installment or
term), bank-stated installment overrides computed one, installment → principal/interest/fees split. Accept: rapid — principal
sum = capital and final residual = 0 (also after events); installments before an event unchanged. Read F: Mutui e prestiti.

**V05** Conditions (description contains/regex, amount range, account) and actions (category, payee, tags); first-match by
priority; JSON (de)serialization with validation; `category_hint` never applied directly. Accept: table tests, fuzz on
JSON decoding. Read F: Modello di dominio (Regola), Flusso di import step 4.

**V06** Reconciliation: difference at date, unreconciled list, close when zero; forecast projection from opening balances
+ dated flows, first day below zero/threshold. Accept: rapid — balance at date independent of insertion order, forecast
at horizon 0 equals current balance. Read F: Riconciliazione, Previsione di cassa.

**V07** Migration 0004 + sqlc queries + repos; split sum verified by an integrity test; `ON DELETE RESTRICT` on used
categories; TxFilter tag/payee/split-aware (extend SQL-vs-`Match` oracle test). Accept: contract tests shared with memstore.
Read T: Modello dati, Persistenza e migrazioni.

**V08/V09/V10/V11/V12** Forward migration + queries + repo for the listed tables; constraints as in T (explicit rule
columns for recurrences, JSON validated for rules, `UNIQUE(loan_id, valid_from)`, one budget per category and period);
migration test (up on populated 0003 archive), `reset --keep-settings` updated: rules and tags kept, everything else wiped
(V08/V10/V11 extend the reset test). Read T: Modello dati.

**V13** Use cases Plan/Apply for tags, payees, split/unsplit; module `core`. Accept: dry-run, `module_disabled` not
applicable; rapid — split edit keeps sum; export/import FUI round-trip includes tags. Read T: Architettura.

**V14** `rules` use cases (add/list/test/apply `--dry-run`) and the import step 4 hook (suggestions flagged in plan).
Accept: idempotent re-apply; import preview shows rule effects. Read F: Flusso di import.

**V15** Budget use cases (`set`, `status`, `assign`, `move`, copy from previous month); `Require(budget)`; mode switch via
`budget.mode`. Accept: e2e-ready services against memstore; rapid invariants of V02 through the store. Read F: Budget.

**V16** `recurring` use cases: auto-create pending tx (`run --until`), skip, confirm, import match turns pending → booked
(module-gated; works when `recurring` is on, ignored otherwise). Accept: running twice creates no duplicates. Read F: UC-08.

**V17** `loan` use cases: add (computed or imported), schedule, event (new version), installments generate pending tx on
the debit account; confirm splits principal (transfer to loan account) and interest (dedicated category); mismatch warns.
Accept: confirm preserves net worth change = −interest; schedules versioning. Read F: Mutui e prestiti.

**V18** `reconcile` use case with Plan/Apply; editing a reconciled tx requires explicit confirmation (cross-check with
`tx edit`). Accept: closes only at zero difference; `import undo` refuses reconciled tx (existing rule). Read F: Riconciliazione.

**V19** Forecast (balances + recurrences + loan installments + budget/3-month average variable spend), reports (budget vs
actual, balance over time, remaining debt, interest paid, net worth over time, top payees), saved reports with relative
periods; all take `module.Set` and filter loans/budgets out when off. Accept: golden JSON per report; rapid — net worth
unchanged by a fee-less transfer. Read F: Report e analytics.

**V20-V24** Cobra commands with `--json/--csv`, `--dry-run`, interactive prompts, `module` annotation (budget, recurring,
loans, analytics), examples on every command, `help --json` golden updated. Accept: command tests + golden outputs; e2e
scripts are in V32. V24 updates `kpi networth` to subtract loan debt and adds a `--series` chart output only if V19 exposes it.
Read F: CLI, Requisiti trasversali.

**V25** Cobra `ValidArgsFunction` for accounts, categories, tags, payees, loans, reports, `--connector`; disabled modules
yield no suggestions. Accept: tests run `__complete` against a temp archive. Read F: Aiuto (completamento).

**V26-V28** teatest screens using `screentest`, key map from F: Requisiti TUI (keys 4/5/6 fixed), 80x24 snapshots, all
themes + `NO_COLOR`, deletion with confirmation, over-budget shown with a label (not colour only). V28 includes the file
picker, connector/profile choice, row include/exclude and duplicate highlighting. Read F: TUI, Import.

**V29-V31** Settings: module list with dependency-closure warning and disable confirmation (calls `module.Enable/Disable`
use cases, writes `modules.enabled` only), theme picker writing the theme config key with live preview; V30 CRUD lists;
V31 split editor (sum guard) and `r` reconcile. Accept: teatest flows; contrast tests unchanged.

**V32** Compose services in `cmd/fui`, register commands and screens only for active modules, testscript e2e UC-08 (recurring),
loan lifecycle, budget (caps and envelopes), rules in import, reconcile, forecast; module matrix (`expense`, `full`,
each module off) with per-preset `help --json` golden; skill updated (`fui module list --json`, new flows); CI matrix. Read T: Test.

## 5. Risks and open questions

Risks: (1) V15/V17/V19 are the longest critical path (budget and loans are the largest domains); (2) store tasks in wave 2
all touch generated files, so rebase order is V07 → V12 with `go generate` each time; (3) `internal/app/importing` is edited by
V14 and V16 in the same wave (separate files, but a shared plan struct, so coordinate the hook interfaces first); (4) transaction
table changes (`recurrence_id`, `due_date`) alter an MVP-critical table: the migration test must run on a populated archive.

Open questions (product decision):
1. Do `payees`/`tags` tables already exist in migration 0003? Plan assumes tags/payees arrive in 0004.
2. Envelope mode: are credit cards/loans excluded from "liquidity accounts included in the budget", and is inclusion a per-account flag?
3. Tag-based budgets overlapping category budgets: only reported separately (as in the spec) or also deducted from "Da assegnare"?
4. Pending recurrences: should `recurring run` run automatically at every launch (needs a clock-driven hook) or only on demand?
5. Variable-rate loan: is the new rate always entered by the user, or inferred from a mismatched debit (spec: "proposes")? Default plan: proposes only.
6. Reconciled-edit confirmation: also for bulk edits and rules apply (`--dry-run` shows them), or hard block?
7. Rule conditions scope: regex allowed, or only contains/equals (safer for performance and testing)?
8. Net worth report granularity and whether archived accounts count in historical series.
9. Settings theme choice: stored in config (`ui.theme`) or in the archive? Plan assumes config, hot reload.
10. Dynamic completion reading the archive: acceptable when the archive is locked or missing (fall back to no suggestions)?
