# V06 — Domain: reconciliation and cash forecast
Branch `feat/v-domain-recon`. Owned: `internal/domain/{reconcile,forecast}*.go` (new).

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Domain only: stdlib, pure, no I/O/clock/globals, money as `Money`/int64 minor units, `Date` without time. Reuse existing domain types/errors (`invalidValue`, `missingField`, rounding in `rounding.go`). New error codes: prefer existing codes; if a new one is unavoidable add it to `errors.go` `Codes()` AND map it in `internal/cli/exitcode.go` (user error → 1). Coverage of new files ≥ 90%. Plan: `docs/PLAN-v0.2.md` §4.

Spec: funzionali.md "Import, export e riconciliazione" (riconciliazione), "Previsione di cassa".
1. Reconciliation: `Reconcile(statementBalance, opening, txs, date) (difference, unreconciled []TransactionID)`; can close only when difference == 0; closing marks booked tx up to date as reconciled (returns the ids).
2. Forecast: inputs opening balances per account (same currency per account), dated flows (recurrences, installments, estimated variable spend per day); output daily balance series to horizon, first day below zero and below a threshold. Horizon 0 == current balance.
3. rapid: balance at date independent of insertion order; forecast(0) == opening; adding a zero flow changes nothing.
