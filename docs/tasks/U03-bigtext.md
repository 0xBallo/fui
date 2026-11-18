# U03 — Component: big digits
Branch `feat/u03-bigtext`. Owned: `internal/tui/components/bigtext.go`, `bigtext_test.go`, its goldens.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Components never import `internal/domain`.

Read only: `internal/tui/components/barchart.go` (style of a component), `internal/tui/theme/theme.go` (API).

Build a 3-row block font for amounts, used by the dashboard net worth and the transaction totals:
```go
// BigText renders s with 3-row block glyphs; runes without a glyph render as normal text, vertically centered.
func BigText(s string, style lipgloss.Style) string
// BigTextWidth is the rendered width of s, so callers can fall back to normal text when it does not fit.
func BigTextWidth(s string) int
```
Glyphs: `0-9 . , ' - + − ` and space, built with `█ ▀ ▄` (each glyph 3 cols + 1 col gap; `.`/`,`/`'` 1–2 cols). Keep the glyph table a `[...]string` literal, no file I/O.
Tests: golden for `"-1.234.567,89"` and `"+0"`, width test equals `lipgloss.Width` of output, unknown rune passthrough (`"€"`).
