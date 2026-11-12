# V05 — Domain: categorization rules
Branch `feat/v-domain-rule`. Owned: `internal/domain/rule*.go` (new).

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Domain only: stdlib, pure, no I/O/clock/globals, money as `Money`/int64 minor units, `Date` without time. Reuse existing domain types/errors (`invalidValue`, `missingField`, rounding in `rounding.go`). New error codes: prefer existing codes; if a new one is unavoidable add it to `errors.go` `Codes()` AND map it in `internal/cli/exitcode.go` (user error → 1). Coverage of new files ≥ 90%. Plan: `docs/PLAN-v0.2.md` §4.

Spec: funzionali.md "Modello di dominio" (Regola), "Flusso di import" step 4, tecniche.md "Motore delle regole".
1. `Rule` (name, priority, enabled, conditions: description contains (normalized, case/accent-insensitive via caller-provided normalized text) | equals, amount range (abs, sign), account ids; actions: set category, payee, tags).
2. Regex conditions: NOT in this version (default for the open question; record in DEV).
3. `Apply(rules, tx, normText) Result`: rules by priority, first rule that sets a field wins for that field; result lists which rule set each field. `category_hint` from connectors is never applied directly (only used as a condition input? no — leave it out, note it).
4. JSON (de)serialization of conditions/actions with strict validation (unknown fields rejected) — fuzz test `FuzzDecodeRule`. Table tests for priority/field precedence.
