# U01 — Theme: tier, info and soft-flow tokens
Branch `feat/u01-theme-tokens`. Owned: `internal/tui/theme/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `internal/tui/theme/{theme,palette,contrast}.go` and their tests.

Add tokens to `Tokens` and fill them in **every** theme (4 Catppuccin flavors, terminal, high-contrast, no-color path):
- `Caution` (Peach) — budget tier 90–99. `Warning` (Yellow) already exists for 70–89, `Over` (Red) for ≥ 100.
- `Info` (Blue) — info alerts.
- `IncomeSoft`, `ExpenseSoft` — lighter tone of Green/Red for pending rows (blend toward `Base` ~40%, or the flavor's lighter neighbor; terminal theme: the non-bright ANSI color vs bright; no-color: same as base).
- `Chip` — chip background (Surface1).

Add styles on `Theme`: `Caution`, `Info`, `Chip` (Chip = bg Chip, fg Text, Padding(0,1)), and a helper:
```go
// Flow is the direction of a money movement, for coloring.
type Flow int // FlowIncome, FlowExpense, FlowTransfer
func (t Theme) FlowStyle(f Flow, pending bool) lipgloss.Style // transfer = Base (plain text) in both states
```
Tests: every theme has non-empty new tokens; `FlowStyle` table test (6 cases); extend the contrast test so `IncomeSoft`/`ExpenseSoft`/`Caution`/`Info` keep the same minimum contrast rule used for the other text tokens on `Base`.
Out of scope: using the tokens in screens.
