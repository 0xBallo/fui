# T10 — TUI (Dashboard, Transactions, Accounts)

Branch `feat/tui`. Owned path: `internal/tui/`. Read `AGENTS.md` first, then the merged `internal/app` (Service),
`internal/app/reporting` (Spending, CashFlow, Runway), `internal/domain`, `internal/testutil/memstore`.
`internal/tui` imports only `app`, `app/reporting`, `domain` — never `store` or `connector`.
Spec refs: technical spec "TUI"; functional spec "Requisiti funzionali — TUI" (MVP screens only: Dashboard,
Transactions, Accounts; the other four screens are v0.2 — show them as disabled tabs or omit them).

## Deliverables

1. `tui.Root` (Bubble Tea, `charmbracelet/bubbletea` + `bubbles` + `lipgloss`): screen router (keys 1/2/3),
   status bar (errors shown here), help `?` generated from keymaps, global keymap (`q` quit, `esc` back).
   Exported constructor taking the use cases as interfaces + config (locale, theme) and a `Run(ctx) error`.
2. `screens/dashboard`: account balances, net worth (base currency), runway (months/days/zero date or
   "not enough data"), current-month spending by category, quick add (`a`).
3. `screens/transactions`: virtualized table, pages of 200 rows loaded via the app's keyset pagination `(date,id)`,
   max 3 pages in memory around the selection; filters (`f`), search (`/`), add (`a`), edit (`e`/enter), delete
   (`d` with confirm), multi-select (space) + bulk categorize. Form with autocompletion for account, category,
   payee.
4. `screens/accounts`: list with balances, detail (movements of the account), add, archive.
5. Components in `internal/tui/components` (no domain knowledge): virtual table, autocomplete form field, confirm
   dialog, text bar chart.
6. Rules: `Update`/`View` never do I/O — all reads/writes are `tea.Cmd` returning messages; every change calls the
   use case immediately and reloads from the store (no optimistic state). Below 80×24 show only a minimum-size
   warning. Colors via `lipgloss.AdaptiveColor`, `high-contrast` theme; no color-only information (minus sign on
   outflows, status symbols). Keymaps with `bubbles/key` + descriptions; a test asserts no screen binds the same key
   twice.

## Acceptance

- `charmbracelet/x/exp/teatest` snapshots at 80×24 for each screen on a memstore fixture, plus `Update` tests for
  key sequences (navigate, add transaction, delete with confirm, page loading).
- `golangci-lint run ./internal/tui/...` clean; coverage ≥ 70 %.
