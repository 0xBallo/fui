# U04 — Components: chips, row style, bar style
Branch `feat/u04-chips-styles`. Owned: `internal/tui/components/{chips.go,chips_test.go,virtualtable.go,barchart.go}` and their tests/goldens.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: the three component files above, `internal/tui/theme/theme.go` (styles API).

Build:
1. `func Chips(th theme.Theme, labels []string, maxWidth int) string` — each label rendered with `th.Chip`, separated by one space, truncated as a whole to `maxWidth` with `…` (`github.com/charmbracelet/x/ansi` `Truncate`, already a dependency).
2. `Row` gains `Style lipgloss.Style` (zero value = no style). In `Table.View` the row style applies to every cell; the selected row still wins (inherit order: selected, row style, cell).
3. `BarItem` gains `Style lipgloss.Style` used for the filled bar when set (fallback `th.BarFill`). Negative values: `renderRow` uses the absolute value for the bar length; the caller puts the sign in `DisplayValue`.
Tests: chips truncation table test (fits, cuts mid-chip, maxWidth ≤ 1); table golden with a styled row and the cursor on it; barchart with a negative value and a custom style. Existing tests stay green.
