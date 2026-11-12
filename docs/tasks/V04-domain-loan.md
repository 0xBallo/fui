# V04 — Domain: loans and amortization
Branch `feat/v-domain-loan`. Owned: `internal/domain/loan*.go` (new).

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Domain only: stdlib, pure, no I/O/clock/globals, money as `Money`/int64 minor units, `Date` without time. Reuse existing domain types/errors (`invalidValue`, `missingField`, rounding in `rounding.go`). New error codes: prefer existing codes; if a new one is unavoidable add it to `errors.go` `Codes()` AND map it in `internal/cli/exitcode.go` (user error → 1). Coverage of new files ≥ 90%. Plan: `docs/PLAN-v0.2.md` §4.

Spec: funzionali.md "Mutui e prestiti" (read fully), tecniche.md "Servizi di dominio" (Ammortamento formula).
1. `Loan` (kind mortgage|loan, principal, disbursed on, term in months, rate type fixed|variable|mixed, debit account), `Schedule` versions valid from a date (origin imported|computed, annual rate as `Rate`), `Installment` (due, principal, interest, fees, residual).
2. French amortization with `math/big.Rat`, round to cents only per installment, last installment absorbs residual. Events: rate change from date, renegotiation, partial prepayment (reduce installment | reduce term) → new schedule version from the event date, installments before it unchanged. Bank-stated installment overrides computed one (imported schedule wins).
3. `SplitInstallment(inst) (principal, interest, fees)` for the app layer.
4. rapid: principal sum == capital and final residual == 0, also after random events; installments before an event unchanged. Table tests with a known schedule (document the source of expected numbers).
