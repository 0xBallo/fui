# U16 — Dashboard: last 10 transactions
Branch `feat/u16-dash-recent`. Owned: `internal/tui/screens/dashboard/recent.go`, `recent_test.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Do not edit `model.go`; `ListTx` is already in `UseCases` (U13).

Read only: `dashboard/section.go`, `dashboard/recent.go` (stub), `app.ListTxRequest/TxView`, `components/virtualtable.go` (`Row.Style`), `theme.FlowStyle` (U01), `screens.SwitchScreenMsg`.

Do: load `ListTx{From, To: period dates, Limit: 10}` (newest first); render a non-interactive `components.Table` (Date, Description, Category, Amount right-aligned) with each row styled by `th.FlowStyle` (transfer = `Transfer != nil`, pending = status pending). Under it the CTA `→ View all transactions (t)`; binding `t` returns `screens.SwitchScreenMsg{Target: screens.TabTransactions}`. Empty period → muted "no transactions in this period".
Tests: 10-row limit, styles per flow, `t` emits the switch msg, empty state.
