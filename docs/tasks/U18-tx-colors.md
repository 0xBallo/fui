# U18 — Transactions: flow colors + tag chips
Branch `feat/u18-tx-colors`. Owned: `internal/tui/screens/transactions/model.go` (row building and columns only) and its tests/goldens.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `transactions/model.go` (`NewTable` columns ~line 117, row builder ~line 460), `theme.FlowStyle` (U01), `components.Chips` and `Row.Style` (U04), `app.TxView.Tags` (U06).

Do: style every row with `th.FlowStyle`: expense = negative amount, income = positive, transfer = `Transfer != nil` (plain); pending status → the soft variant. Keep the minus sign and the status text (no color-only info). Add a `Tags` column rendered with `components.Chips(th, v.Tags, colWidth)`; it is the first column dropped when width < 100.
Tests: style per flow/pending (assert via the row's Style, not ANSI strings), tags column present/absent by width, goldens regenerated for this package only.
