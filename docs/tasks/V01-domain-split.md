# V01 — Domain: tag, payee, split
Branch `feat/v-domain-split`. Owned: `internal/domain/{tag,payee,split}*.go` (new).

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Domain only: stdlib, pure, no I/O/clock/globals, money as `Money`/int64 minor units, `Date` without time. Reuse existing domain types/errors (`invalidValue`, `missingField`, rounding in `rounding.go`). New error codes: prefer existing codes; if a new one is unavoidable add it to `errors.go` `Codes()` AND map it in `internal/cli/exitcode.go` (user error → 1). Coverage of new files ≥ 90%. Plan: `docs/PLAN-v0.2.md` §4.

1. `Tag` (name trimmed, case-insensitive identity, no spaces/commas), `Payee` with aliases: `NormalizePayee(s)` (reuse `NormalizeDescription` style), alias match.
2. `Split` rows (category, amount, note) + `ValidateSplits(total Money, rows []Split) error`: ≥2 rows, same currency, non-zero rows, sum == total; helper to rebalance the last row.
3. Tests: table + rapid (split sum == amount; normalize idempotent). Spec: funzionali.md "Modello di dominio" (Tag, Split), tecniche.md invariants.
