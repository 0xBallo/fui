# U20 — Transactions: period totals
Branch `feat/u20-tx-totals`. Owned: `internal/tui/screens/transactions/{totals.go,totals_test.go,ports.go}`, `model.go` (one call in the list view + one in load).

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Wiring note: `cmd/fui` passes `tuiUseCases{*app.Service, ...}`, so a method of `*app.Service` added to a screen `UseCases` interface needs no wiring; anything else is an **optional port** with a setter, wired by U27.

Read only: `transactions/model.go` (`viewList`, load), `app.Service.TxTotals` (U07), `components/bigtext.go` (U03).

Do: add `TxTotals` to `UseCases`; after every load with the current `ListTxRequest`, fetch totals in the same `tea.Cmd` (errors like a missing rate → muted `totals n/a`). Above the table render `IN` and `OUT` side by side: amounts in `BigText` with `th.Positive`/`th.Negative` when height ≥ 20 and width fits, otherwise one bold line `IN +1.234,00 · OUT −987,00 · 42 tx`. The table height shrinks by the header height (`SetSize`).
Tests: totals follow the filter, big vs compact by size, error state, goldens for this package.
