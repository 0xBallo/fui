# T20 — TUI live search and filter panel

Branch `feat/search-tui` (based on `main` after T16 and T19). Owned paths: `internal/tui/screens/transactions/`,
`internal/tui/components/` (new component files only if needed), `internal/tui/testdata/` goldens it changes.
Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md TUI "Ricerca e filtri", "Liste grandi", "Tastiera e aiuto"; funzionali.md "Ricerca e filtri
delle transazioni" (TUI bullet). Read the merged `domain.TxFilter`, `app.ListTxRequest`, theme tokens (no literal
colors: lint forbids them outside `internal/tui/theme`), and the existing `filter.go` of the screen.

1. `/` opens a search input; every keystroke restarts a 150 ms debounce timer (tea.Tick with a sequence id) and
   cancels the in-flight query via `context.CancelFunc`; stale results (older sequence id) are dropped, so only
   the latest result reaches the screen. `esc` clears the search.
2. `f` opens a filter panel building the same request as the CLI (period, accounts, categories, uncategorized,
   min/max, income/expense, status, kind); active filters are shown as removable chips in a header above the
   list (`backspace`/`x` on a focused chip removes it). Bindings declared with `bubbles/key` so `?` help lists
   them.
3. Footer: row count and sum (per currency) like the CLI summary.
4. Tests: teatest for typing a query (assert only the final query is executed: fake port counting calls with a
   fake clock/ticks), chip removal, filter panel apply; unit tests for debounce/cancel logic. Coverage of
   `internal/tui/screens/transactions` ≥ 80% (currently 78%).
