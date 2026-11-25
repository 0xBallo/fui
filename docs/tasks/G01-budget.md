# G01 — Budget complete

Branch `feat/g01-budget` · worktree `../fui-wt/g01` · TL: Claude Sonnet medium · deps: none.
Read first: `docs/PLAN-gaps.md` (§2 P2, P8, P9, P15; §5; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum),
F: "Budget", "Requisiti funzionali — TUI" (Budget), UC-07; T: "Modello dati" (budgets), "Invarianti" (envelopes).

## Owned paths

`internal/domain/budget*.go`, `internal/store/sqlite/{migrations/<next>_budget_*.sql,queries/budget*.sql,repo_budgets*}`
(+ regenerated `schema.sql`, `sqlc/`), `internal/app/budgeting/`, `internal/testutil/memstore/budget*`,
`internal/cli/budget*.go`, `internal/cli/render/budget*.go`, `internal/tui/screens/budget/`, shared files per
PLAN §5 (help goldens, completion of budget names may be left to G04).

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a2-11 | No annual period | `internal/domain/budget.go:54-63,115` | P8: `period month\|year`; annual cap compares spending over the calendar year (or `month_start_day`-aligned year); annual envelope shows `suggested_monthly = remaining / remaining months` (integer rounding per ADR 0002) |
| a2-12 | A budget covers one target from CLI/TUI | `internal/cli/budget.go:22-27` | `budget set NAME AMOUNT --category A --category B --tag T` (repeatable); domain/app already take slices |
| a2-13 | Thresholds fixed 80/100 | `internal/domain/budget.go:40-41,92-94` | P9: per-budget `--warn N --over N` (percent, `0 < warn < over`), stored, shown in status and TUI |
| a2-14 | Carry opt-in | `budget.go:61`, `internal/cli/budget_set.go:47` | P2: default on for new budgets, `--no-carry`; existing rows unchanged |
| a2-15 | TUI assign/move books on today | `internal/tui/screens/budget/form.go:117` | Move dated in the month on screen (first day of the shown period, or today if inside it) |
| a2-3 | No TUI create form; `s` writes only a month override | `screens/budget/form.go:15-25,101` | `n` new budget form (name, mode-independent targets with category/tag pickers, period, default amount, carry, thresholds); `e` edits the budget (default amount etc.); `s` keeps "set amount for this month"; empty state hints `n` |
| a2-2 | `budget copy --from` ignored | `internal/cli/budget_copy.go:12-14,58-87`; `app/budgeting/budgeting.go:371-383` | Honour `--from YYYY-MM` (default previous month) in app, CLI and TUI copy |
| P15 | `budget set Spesa 400 --month 2026-11` fails on a new budget | `app/budgeting/budgeting.go:136-138` | Creates the budget when missing (target = category with that name, else error `not_found` listing how to pass `--category`) |

## Suggested MTs (TL may re-split)

1. domain: period + thresholds + carry default + annual envelope suggestion (rapid: annual cap = sum of the 12
   monthly spends; carry conservation still holds; envelopes + "Da assegnare" invariant unchanged).
2. store: migration (budget period, warn/over columns; carry default stays per row), queries, repo, migration test on
   a populated archive.
3. app: budgeting use cases for period/targets/thresholds/copy-from/create-on-set; memstore; contract tests.
4. cli: flags and render (status shows period, thresholds, suggested monthly), examples, `help --json` golden,
   testscript `g01_budget.txtar` (create annual envelope, multi-target cap, thresholds, copy --from, spec example).
5. tui: create/edit form, month-dated moves, thresholds/period in rows, `i` help markdown updated, goldens.

## DoD

PLAN §6, plus: `rapid` properties above in `internal/domain`; TUI flow test create → set → assign → move with the
month on screen; F requirement "no information by color alone" kept (OVER label).
