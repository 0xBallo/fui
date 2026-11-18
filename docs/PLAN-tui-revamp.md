# FUI — TUI revamp plan

Same method as `PLAN-v0.3.md`: layers in order (domain → app → components → screens → integration), disjoint owned
paths, git flow of ADR 0013 (feature branch from `develop`, squash PR). Workers follow `docs/tasks/_PROTOCOL.md`
(v0.3 addendum: terse report + `RESULT` line). Tasks are sized for **low-effort agents**: one package, ≤ ~300 changed
lines, every brief lists the only files worth reading. Specs (Italian) stay the source of truth: `F:` = funzionali.md,
`T:` = tecniche.md; U00 updates them first.

Baseline: `develop` with `fix/tui-key-hints` merged (U11 replaces its top hint line with a bottom help bar).

## 1. Scope (from the product request, 2026-11-18)

| Area | Requested |
| --- | --- |
| General | One-line help bar at the bottom, truncated with `…`; `?` expands it in place with commands split into Global and current screen. ASCII-art title "FinanceUI" above the tabs, subtitle `vX.Y by 0xBallo` with `0xBallo` an OSC 8 link to `https://github.com/0xBallo` |
| Dashboard | Sections in order: Net worth (big gross liquidity, small net of loans), Runway (countdown `Xy Xm Xd` + zero date), Flow chart (horizontal bars: income green, expenses red, savings white, values shown), Last 10 transactions + "view all" → Transactions, Alerts (budget near threshold = warning, anomalous tx = warning, budget over = danger, upcoming recurrences in the billing period = info). Every section follows the selected cycle (month, quarter, year, lifetime), switchable with a key and navigable back/forward |
| Transactions | Expenses red, income green, transfers plain; unconfirmed (pending) rows in a lighter tone of the same color; tags as chips; date picker for the date filters; period income/expense totals in big digits |
| Accounts | Currency column; loans/mortgages separated from the accounts that make up gross wealth; per-section totals |
| Budget | Progress bar colored by tier: green < 70, yellow 70–89, orange 90–99, red ≥ 100; icon instead of the `OK` label; `i` opens a markdown modal (glamour) explaining budget types and how to set them |
| Reports | `i` explanation modal as for Budget; charts for the report results |

Out of scope: Bubble Tea v2 migration, new report kinds, new alert types beyond the four above, mouse interactions
beyond what exists.

## 2. Decisions taken (change them in U00 if the product decision differs)

1. **Stay on Bubble Tea v1** (`bubbletea v1.3`, `bubbles v1`, `lipgloss v1`). v2 (`charm.land/*/v2`) is released, but
   migrating ~15k lines of TUI is its own project. Every new library is picked at its v1-compatible version.
2. **New dependencies** (one ADR, `docs/adr/0019-tui-libraries.md`): `github.com/charmbracelet/glamour` (v0.x line,
   lipgloss v1) for markdown modals; `github.com/NimbleMarkets/ntcharts` (`main` branch = Bubble Tea v1) for line/time
   series charts in Reports; `github.com/ethanefung/bubble-datepicker` (Bubble Tea v1) for the date filters. Each is
   added by the one task that uses it; the existing `go list -deps ./cmd/fui` test must stay free of
   `net/http`/`net/rpc` (if a library fails it, the task stops and reports `blocked`).
3. **No new dependency** where the stack already covers it: `bubbles/help` (truncated short help with `…`),
   `bubbles/viewport` (scrolling modal and dashboard), `bubbles/progress` (budget bars), `termenv.Hyperlink` (OSC 8,
   `termenv` is already a direct dependency), the house `components.BarChart` (horizontal bars) and `components.Modal`
   (no `bubbletea-overlay`). Big digits and the banner are small hand-drawn block fonts (no figlet library).
4. **Layout budget at 80×24**: banner (3 rows) only when height ≥ 30, otherwise a one-line title right-aligned in the
   tab bar gap; then tabs (3), body, status line (1), help line (1). The dashboard body scrolls in a viewport and uses
   two columns when width ≥ 120.
5. **Cycles**: month = accounting month (`domain.MonthPeriod`, `month_start_day`); quarter and year = calendar;
   lifetime = from the oldest transaction to today. "As of" values (net worth, runway) use `min(period end, today)`.
   Dashboard keys: `p` next cycle kind, `h/←` `l/→` previous/next period (next stops at the current one).
6. **Dashboard content**: the accounts table and the spending-by-category chart leave the dashboard (accounts are on
   tab 3, spending on Reports). The v0.3 insights panel is absorbed by Alerts: anomalies become warnings, subscription
   and trend insights become info entries, `x` dismiss and `a` accept keep working on the focused alert.
7. **Net worth figures** (core only, no module needed): gross = sum of `app.NetWorth.Lines[].Converted` for non-loan
   accounts; net = gross + loan lines (negative). Positions/assets lines (v0.3) stay as muted lines when present.
8. **Budget tiers** are visual only (fixed 70/90/100); the Alerts warning/over still use the budget's own thresholds
   (`domain.BudgetState`). Icons: `✓` < 70, `!` 70–89, `‼` 90–99, `✗ OVER` ≥ 100 (the `OVER` text stays: F requires
   that no information is conveyed by color alone). Pending transactions keep their status symbol for the same reason.
9. **UI text stays English** (AGENTS.md); help markdown files are English and embedded with `go:embed`.
10. **Explanation key** is `i` on Budget and Reports (free there); on the Dashboard `i` keeps focusing Alerts.

## 3. Tasks (wave = earliest start; max 3 agents in parallel, rolling pool)

Agent policy: effort **low**, no flagship; escalate a task to medium only after a `blocked` result.

| Wave | Task | Title | Branch | Owned paths | Deps |
| --- | --- | --- | --- | --- | --- |
| 0 | U00 | Docs: spec update + ADR 0019 | `docs/u00-tui-revamp` | `docs/specs/`, `docs/adr/0019*` | — |
| 1 | U01 | Theme: tier, info and soft-flow tokens | `feat/u01-theme-tokens` | `internal/tui/theme/` | U00 |
| 1 | U02 | Domain: calculation cycles | `feat/u02-domain-cycle` | `internal/domain/cycle*.go` | U00 |
| 1 | U03 | Component: big digits | `feat/u03-bigtext` | `internal/tui/components/bigtext*` | U00 |
| 1 | U05 | Component: help bar | `feat/u05-helpbar` | `internal/tui/components/helpbar*` | U00 |
| 1 | U06 | App+store: tags in `TxView` | `feat/u06-tx-tags` | `internal/app/transaction.go`, store tag queries/repo, `memstore/` | U00 |
| 1 | U07 | App: transaction totals | `feat/u07-tx-totals` | `internal/app/txtotals*.go` | U00 |
| 2 | U04 | Components: chips, row style, bar style | `feat/u04-chips-styles` | `components/{chips*,virtualtable*,barchart*}` | U01 |
| 2 | U08 | Component: markdown modal (glamour) | `feat/u08-mdmodal` | `components/mdmodal*`, `go.mod/sum` | U00 |
| 2 | U09 | Component: line chart (ntcharts) | `feat/u09-linechart` | `components/linechart*`, `go.mod/sum` | U01 |
| 2 | U10 | Component: date picker | `feat/u10-datepicker` | `components/datepicker*`, `go.mod/sum` | U01 |
| 2 | U11 | Root: bottom help bar | `feat/u11-root-help` | `internal/tui/{root,keymap}*.go`, `internal/tui/testdata/` | U05 |
| 3 | U12 | Root: header banner + version link | `feat/u12-root-banner` | `internal/tui/{root,banner}*.go`, `internal/tui/testdata/` | U11 |
| 3 | U13 | Dashboard: frame, cycles, net worth | `feat/u13-dash-frame` | `screens/dashboard/` except insight files | U01–U03 |
| 3 | U18 | Transactions: flow colors + tag chips | `feat/u18-tx-colors` | `screens/transactions/model.go` (+tests) | U01,U04,U06 |
| 3 | U21 | Accounts: currency, sections, totals | `feat/u21-accounts` | `screens/accounts/` except `pos*` | U01 |
| 3 | U22 | Budget: tiered progress + icons | `feat/u22-budget-tiers` | `screens/budget/{progress,view}*` | U01 |
| 4 | U14 | Dashboard: runway countdown | `feat/u14-dash-runway` | `dashboard/runway*` | U13 |
| 4 | U15 | Dashboard: flow chart | `feat/u15-dash-flow` | `dashboard/flow*` | U04,U13 |
| 4 | U16 | Dashboard: last 10 transactions | `feat/u16-dash-recent` | `dashboard/recent*` | U13 |
| 4 | U17 | Dashboard: alerts | `feat/u17-dash-alerts` | `dashboard/{alerts,insight}*` | U13 |
| 4 | U19 | Transactions: date picker in filters | `feat/u19-tx-datepicker` | `transactions/{filter*,keymap.go}` | U10 |
| 4 | U20 | Transactions: period totals | `feat/u20-tx-totals` | `transactions/{totals*,ports.go}`, `model.go` view only | U03,U07,U18 |
| 4 | U23 | Budget: explanation modal | `feat/u23-budget-help` | `screens/budget/{help*,keymap*,model.go}` | U08 |
| 4 | U24 | Reports: explanation modal | `feat/u24-report-help` | `screens/report/{help*,keymap*,model.go}` | U08 |
| 5 | U25 | Reports: series charts | `feat/u25-report-series` | `screens/report/{charts*,view.go}` | U09,U24 |
| 6 | U26 | Reports: bar charts | `feat/u26-report-bars` | `screens/report/charts_bar*` | U25 |
| 7 | U27 | Integration: wiring, goldens, e2e | `feat/u27-integration` | `cmd/fui/`, `internal/tui/root.go` Config, `internal/tui/*_test.go`, goldens, `CHANGELOG.md` | all |
| 8 | — | Release PR (CHANGELOG), CI green | — | — | U27 |

Rules that keep tasks small and parallel:

- Screen tasks never touch `cmd/fui` or `Root.Config`: a new data need is an **optional port** declared in the
  section's own file with a `SetXUseCases` setter (pattern: `dashboard.SetInsights`); a nil port hides the section.
  U27 wires every port.
- U13 creates the dashboard section files (`runway.go`, `flow.go`, `recent.go`, `alerts.go`) as stubs with the
  agreed signatures (see U13); U14–U17 then own one file each and never edit `model.go`.
- Three tasks touch `go.mod`/`go.sum` (U08–U10): after merge of the first, the others rebase and run `go mod tidy`.
- Goldens: a task regenerates only the goldens of its package (`go test ./<pkg> -update`); root goldens are owned by
  U11, U12 and U27.

## 4. Task briefs

Each `docs/tasks/U<nn>-*.md` holds the brief. Common acceptance: table-driven tests, `teatest`/golden snapshots for
visible changes, `TZ=UTC` and `Pacific/Kiritimati`, coverage per package ≥ 80% (`internal/domain` ≥ 90%), theme
tokens only (no literal colors), keymap collision test green.

## 5. Status

| Task | Status | PR |
| --- | --- | --- |
| U00–U27 | merged | #36–#63 |
