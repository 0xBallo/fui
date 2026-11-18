# U12 — Root: header banner + version link
Branch `feat/u12-root-banner`. Owned: `internal/tui/banner.go`, `internal/tui/banner_test.go`, `internal/tui/root.go` (View/layout only), `internal/tui/testdata/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `internal/tui/root.go`, `go doc github.com/muesli/termenv Hyperlink`.

Do:
1. `banner.go`: a hand-drawn 3-row ASCII-art logo "FinanceUI" (half-block `▀▄█` style, ≤ 40 cols) as a string constant, styled with `th.Title`/`Tokens.Accent`; subtitle at its right on the bottom row: `vX.Y by 0xBallo`, where `0xBallo` is `termenv.Hyperlink("https://github.com/0xBallo", "0xBallo")` (plain text under the no-color theme and in tests' golden profile if hyperlinks break width).
2. Version: add `Version string` to `Config`; display `vMAJOR.MINOR` parsed from `v0.3.0` style values, the raw value otherwise (`dev`). Wiring from `cmd/fui` is U27.
3. Layout: banner rows above the tabs only when height ≥ 30; otherwise a one-line `FinanceUI vX.Y by 0xBallo` right-aligned in the tab bar gap (omitted if it does not fit). Keep `reservedRows`/mouse `tabBarRows` offsets correct: tab clicks must still hit the right tab with the banner shown.
Tests: version formatting table test; 80×24 (compact) and 120×40 (banner) goldens; tab click with banner; `lipgloss.Width` of the hyperlink equals 7.
