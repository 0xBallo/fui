# V02 — Domain: budget engine (caps, envelopes, carry-over)
Branch `feat/v-domain-budget`. Owned: `internal/domain/budget*.go` (new).

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Domain only: stdlib, pure, no I/O/clock/globals, money as `Money`/int64 minor units, `Date` without time. Reuse existing domain types/errors (`invalidValue`, `missingField`, rounding in `rounding.go`). New error codes: prefer existing codes; if a new one is unavoidable add it to `errors.go` `Codes()` AND map it in `internal/cli/exitcode.go` (user error → 1). Coverage of new files ≥ 90%. Plan: `docs/PLAN-v0.2.md` §4.

Spec: funzionali.md "Budget" (read fully), tecniche.md "Servizi di dominio" (Budget) + invariants.
1. Types: `BudgetMode` (cap|envelope), `BudgetTarget` (category or tag), monthly amount with per-month override, carry-over flag, thresholds 80/100.
2. Pure functions over inputs (tx rows already filtered by caller, split rows counted per row, pending = planned only, transfers/investment/loan principal excluded by `Kind`): cap status (spent, planned, remaining, %, threshold state); envelope ledger (assigned, moves, balance, overspend, "Da assegnare" = included liquidity balances − envelope balances); carry-over of remaining/overspend to next month; periods via existing `MonthPeriod`/`month_start_day`; one-budget-per-category-per-period validation.
3. Defaults for open questions (record in DEV): liquidity accounts = checking/savings/cash kinds only (cards and loans excluded); tag budgets reported separately, not deducted from "Da assegnare".
4. rapid: envelopes total + Da assegnare == sum of included balances; carry-over conserves money across months.
