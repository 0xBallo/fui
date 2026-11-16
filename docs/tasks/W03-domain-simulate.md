# W03 — Domain: simulate engine

Branch `feat/w-domain-simulate`. Owned: `internal/domain/simulate/` (only package allowed `float64`; add the forbidigo exception in `.golangci.yml` for this path only). Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Spec: F "Simulazioni" (all three tables + bullets). Pure, deterministic, no I/O, stdlib only, no Money floats leaking: public API takes/returns minor-unit int64 and rates as basis points or float64 inside the package.
1. `Investment(params)` and `PAC(params)` share ONE engine (PAC = periodic contributions with annual growth, frequency monthly/quarterly/yearly). Params per F table: initial capital, expected annual return, horizon, compounding monthly|annual, annual cost %, capital-gains tax rate, inflation. Output: year-by-year nominal and real value, contributions total, gross and net gain. Three rates (pessimistic/base/optimistic) → three results.
2. `CashFlow(base []DatedBalance/Flow, events []Event)` : events = one-off expense/income, recurring change from a date, income stop, new loan (capital, rate, years → annuity); output monthly balance vs base, first month below zero, end-horizon delta, resulting runway months. Reuse `domain.Forecast` input types where they exist (read `internal/domain/forecast.go`).
Accept: table tests with hand-computed numbers, rapid — zero return ⇒ value = contributions; monotonic in rate; PAC with zero contributions = Investment; coverage ≥ 90%.
