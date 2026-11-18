# U23 — Budget: explanation modal
Branch `feat/u23-budget-help`. Owned: `internal/tui/screens/budget/{help.md,help.go,help_test.go,keymap.go,model.go}`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: F section "Budget" (`docs/specs/funzionali.md`), `budget/keymap.go`, `components/mdmodal.go` (U08), CLI `fui budget --help` text in `internal/cli/budget*.go` (for exact commands).

Do: `help.md` (English, ≤ 80 lines, embedded with `go:embed`): what caps (ceilings) and envelopes are, when to use each, carry-over, warning/over thresholds, the progress colors and icons of U22, how to set amounts here (`s`, `a`, `m`, `c`, `h/l`) and with the CLI. Key `i` (help `i explain budgets`) opens a `MarkdownModal`; while open it gets all keys, `InputActive()` is true, View renders it.
Tests: `i` opens, esc closes, content contains both mode names, keymap collision test green.
