# U24 — Reports: explanation modal
Branch `feat/u24-report-help`. Owned: `internal/tui/screens/report/{help.md,help.go,help_test.go,keymap.go,model.go}`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: F section "Report e analytics", `report/{keymap,kinds}.go`, `components/mdmodal.go` (U08).

Do: `help.md` (English, ≤ 100 lines, `go:embed`): one short paragraph per predefined report in `kinds.go` (what it answers, its parameters), saved reports, compare and simulations when their module is on, keys of this screen. Key `i` (help `i explain reports`) opens a `MarkdownModal` as in U23.
Tests: open/close, every `predefinedKinds()` label appears in the markdown (guards drift), keymap collision test green.
