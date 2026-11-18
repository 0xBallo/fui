# U22 — Budget: tiered progress + icons
Branch `feat/u22-budget-tiers`. Owned: `internal/tui/screens/budget/{progress.go,view.go}` and their tests/goldens.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `budget/progress.go`, `budget/view.go`, theme styles `Positive`, `Warning`, `Caution` (U01), `Error`.

Do: `tier(percent int)` → green < 70 (`Positive`), yellow 70–89 (`Warning`), orange 90–99 (`Caution`), red ≥ 100 (`Error`). The bar fill uses the tier style (both caps and envelopes, envelopes via `envelopePercent`). Replace `stateLabel` text with the tier icon: `✓`, `!`, `‼`, `✗ OVER` (OVER stays as text, F requirement). The domain `BudgetState` no longer drives color here.
Tests: tier boundaries 0/69/70/89/90/99/100/150; icon per tier; goldens.
