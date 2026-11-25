# G06 — Reports and forecast

Branch `feat/g06-reports` · worktree `../fui-wt/g06` · TL: Claude Sonnet medium · deps: none.
Read first: `docs/PLAN-gaps.md` (§2 P5, P6, P11, P16; §5; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum),
F: "Report e analytics", "Previsione di cassa", "Moduli" (Effetti di un modulo disattivato),
"Requisiti funzionali — TUI" (Report); T: "Patrimonio netto", "Moduli".

## Owned paths

`internal/app/reporting/` (except `insight*.go`, G09), `internal/domain/reporting_service*.go`,
`internal/domain/forecast*.go`, store report/transaction aggregate queries, `internal/config/` (forecast keys),
`internal/cli/{report*,forecast*,kpi*}.go`, `internal/cli/render/{report*,forecast*,networth*}.go`,
`internal/tui/screens/report/` (except `sim*`, G09), `internal/tui/screens/screen.go` (`SwitchScreenMsg` filter),
`internal/tui/screens/transactions/` (only to accept an incoming filter), `internal/tui/screens/dashboard/` (P6 only).

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a2-16 | No common filters / group-by; saved reports store no filters | `reporting/reports.go:17-22`, `cli/report.go:39-43`, `report_saved.go:23-27` | P11: every report takes the `domain.TxFilter` flags (reuse `cli/filterflags.go`) and `--group-by`; saved reports persist filter + grouping + relative period |
| a2-17 | Series are monthly only | `reporting/reports.go:71-75` | `--granularity day\|week\|month\|year` for balance over time and net worth |
| a2-18 | TUI rows do not open their transactions | `screens/report/keymap.go`, `screens/screen.go:54` | `enter` on a row switches to Transactions with the row's filter (period + category/tag/payee/account) |
| a2-19 | TUI export writes transactions, not the result | `screens/report/form.go:72-75,132-166` | Export the report result as CSV/JSON (same encoder as CLI `--csv/--json`) |
| a2-20 | Spending/cash flow not runnable ad hoc in TUI | `screens/report/kinds.go:17-21` | Both listed as predefined reports |
| a2-21 | Spending by category ignores splits (bug) | `queries/transactions.sql:136-143`, `domain/reporting_service.go:54` | Split rows counted in each row's category; regression test |
| a2-22 | Forecast deviations | `config/defaults.go:20`, `cli/forecast.go:20-21` | P5 |
| a3-1 | Compare not saveable; old `Analytics.NetWorth` used | `reporting/saved.go:30,199-200`, `cli/report_run.go:76-77`, `cmd/fui/tuiports.go:60-61`, `screens/report/report.go:78-79` | P16: `report save NAME compare …`; every net-worth path uses `NetWorthReport`; remove `Analytics.NetWorth` if unused (update goldens) |
| a3-15 | Investment accounts counted when the module is off | `reporting/networth.go:136-150` | P6 (also `wallet` with crypto off once G08 adds the kind: filter by a module→account-kind map so G08 only adds an entry) |
| a3-16 | Compare: relative periods only, no filters, no drill-down | `reporting/period.go:31-67`, `compare.go:13-16`, `screens/report/compare.go:30-34` | P16 periods and filters; `--chart`; TUI compare rows drill down like a2-18 |

## Suggested MTs

1. domain/store: split-aware spending (bug first, with test); granularity helpers (week/day/year period ends).
2. app: common filter + group-by in report requests; saved params v2 (old params still load); compare periods and
   filters; saved compare; NetWorthReport everywhere; P6 module→kind exclusion.
3. app + config: forecast P5 (horizon parsing, weekly buckets, min balance key).
4. cli: filter/group/granularity flags on report commands, `forecast` flags, `report save … compare`, goldens,
   testscripts `g06_reports.txtar`.
5. tui: drill-down to Transactions (`SwitchScreenMsg` gains an optional filter), export of results, ad-hoc
   spending/cash flow, compare filters, goldens.

## DoD

PLAN §6, plus: golden JSON per report kind with filters and grouping; rapid — sum over `--group-by category` equals
the ungrouped total for the same filter; net worth unchanged by a fee-less transfer (existing property still green).
