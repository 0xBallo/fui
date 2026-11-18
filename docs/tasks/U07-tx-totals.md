# U07 — App: transaction totals
Branch `feat/u07-tx-totals`. Owned: `internal/app/txtotals.go`, `internal/app/txtotals_test.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `ListTxRequest`/`ListTx` in `internal/app/transaction.go`, the conversion used by `NetWorth` in `internal/app/fx.go` (reuse it, do not duplicate rate lookup).

Build:
```go
// TxTotals are the income and expense sums of every transaction matching a filter, in the base currency.
type TxTotals struct{ Base domain.Currency; Income, Expense domain.Money; Count int }
// TxTotals walks every page of ListTx(req) (After/Limit ignored), skips transfers, converts each
// amount at its date; a missing rate fails with ErrRateMissing like NetWorth. Expense is negative.
func (s *Service) TxTotals(ctx context.Context, req ListTxRequest) (TxTotals, error)
```
`// ponytail: pages through ListTx; add a SQL SUM query if 100k-row filters get slow.`
Tests: mixed signs, transfer excluded, foreign currency converted, missing rate error, no base currency error, empty result = zero amounts.
