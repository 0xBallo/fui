# V03 — Domain: recurrences
Branch `feat/v-domain-recur`. Owned: `internal/domain/recurrence*.go` (new).

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Domain only: stdlib, pure, no I/O/clock/globals, money as `Money`/int64 minor units, `Date` without time. Reuse existing domain types/errors (`invalidValue`, `missingField`, rounding in `rounding.go`). New error codes: prefer existing codes; if a new one is unavoidable add it to `errors.go` `Codes()` AND map it in `internal/cli/exitcode.go` (user error → 1). Coverage of new files ≥ 90%. Plan: `docs/PLAN-v0.2.md` §4.

Spec: funzionali.md "Modello di dominio" (Ricorrenza), UC-08.
1. `Recurrence` (template amount/account/category/description, rule every N days|weeks|months, day-of-month with clamp to month end, start, optional end, next date).
2. `Expand(r, from, until) []Date` (occurrences in window, inclusive), `Advance`.
3. `MatchPending(pending []Transaction, booked Transaction, tolerance) (match, ok)`: same account, amount within tolerance (default exact; option ± minor units), date window ±3 days; deterministic tie-break (closest date, then id).
4. Tests: month-end/leap-year tables (31st → Feb 28/29), rapid: expansion monotonic, same window idempotent, `Expand(a,c)` = `Expand(a,b)`∪`Expand(b+1,c)`.
